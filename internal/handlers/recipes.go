package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/szehnder/recipeme/internal/spoonacular"
)

// LLMProvider abstracts the AI backend used for prompt interpretation.
type LLMProvider interface {
	ProcessPrompt(ctx context.Context, prompt string) ([]string, error)
}

// SpoonacularClient abstracts the recipe search client.
type SpoonacularClient interface {
	FanOut(ctx context.Context, terms []string, page, target int, seen map[int]bool) ([]spoonacular.Recipe, error)
}

// recipesResponse is the JSON shape returned by GET /api/recipes.
type recipesResponse struct {
	Prompt      string               `json:"prompt"`
	SearchTerms []string             `json:"searchTerms"`
	Recipes     []spoonacular.Recipe `json:"recipes"`
	Buffer      []spoonacular.Recipe `json:"buffer"`
}

// moreRequest is the JSON body expected by POST /api/recipes/more.
type moreRequest struct {
	SearchTerms []string `json:"searchTerms"`
	Page        int      `json:"page"`
	Seen        []int    `json:"seen"`
}

// moreResponse is the JSON shape returned by POST /api/recipes/more.
type moreResponse struct {
	Recipes []spoonacular.Recipe `json:"recipes"`
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorJSON writes a JSON error response.
func errorJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// RecipesHandler handles GET /api/recipes?q=<prompt>.
// It calls the LLM to interpret the prompt, then concurrently fetches an
// initial page of recipes and a pre-fetch buffer from Spoonacular.
func RecipesHandler(ai LLMProvider, sp SpoonacularClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			errorJSON(w, http.StatusBadRequest, "q parameter is required")
			return
		}

		// Step 1: interpret the prompt via the LLM (30-second timeout).
		llmCtx, llmCancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer llmCancel()

		searchTerms, err := ai.ProcessPrompt(llmCtx, q)
		if err != nil {
			errorJSON(w, http.StatusInternalServerError, "Failed to interpret your prompt — please try again")
			return
		}

		// Steps 2+3: concurrently fetch initial recipes (page 0) and buffer (page 1).
		type fanOutResult struct {
			recipes []spoonacular.Recipe
			err     error
		}

		var wg sync.WaitGroup
		wg.Add(2)

		initialCh := make(chan fanOutResult, 1)
		bufferCh := make(chan fanOutResult, 1)

		go func() {
			defer wg.Done()
			recipes, err := sp.FanOut(r.Context(), searchTerms, 0, 20, map[int]bool{})
			initialCh <- fanOutResult{recipes, err}
		}()

		go func() {
			defer wg.Done()
			recipes, err := sp.FanOut(r.Context(), searchTerms, 1, 20, map[int]bool{})
			bufferCh <- fanOutResult{recipes, err}
		}()

		wg.Wait()

		initialResult := <-initialCh
		bufferResult := <-bufferCh

		if initialResult.err != nil {
			errorJSON(w, http.StatusInternalServerError, "Failed to fetch recipes")
			return
		}

		buffer := bufferResult.recipes
		if buffer == nil {
			buffer = []spoonacular.Recipe{}
		}

		recipes := initialResult.recipes
		if recipes == nil {
			recipes = []spoonacular.Recipe{}
		}

		writeJSON(w, http.StatusOK, recipesResponse{
			Prompt:      q,
			SearchTerms: searchTerms,
			Recipes:     recipes,
			Buffer:      buffer,
		})
	}
}

// MoreHandler handles POST /api/recipes/more.
// It fetches the next page of recipes based on the provided search terms and seen IDs.
func MoreHandler(sp SpoonacularClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req moreRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request body")
			return
		}

		seen := make(map[int]bool, len(req.Seen))
		for _, id := range req.Seen {
			seen[id] = true
		}

		recipes, err := sp.FanOut(r.Context(), req.SearchTerms, req.Page, 20, seen)
		if err != nil {
			errorJSON(w, http.StatusInternalServerError, "Failed to fetch recipes")
			return
		}

		if recipes == nil {
			recipes = []spoonacular.Recipe{}
		}

		writeJSON(w, http.StatusOK, moreResponse{Recipes: recipes})
	}
}
