package spoonacular

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Recipe represents a recipe returned from the Spoonacular API.
type Recipe struct {
	ID               int
	Title            string
	Image            string
	ReadyInMinutes   int
	Servings         int
	SourceURL        string
	Summary          string
	Cuisines         []string
	Diets            []string
	SpoonacularScore float64
}

// apiRecipe is the raw JSON shape returned by Spoonacular.
type apiRecipe struct {
	ID               int      `json:"id"`
	Title            string   `json:"title"`
	Image            string   `json:"image"`
	ReadyInMinutes   int      `json:"readyInMinutes"`
	Servings         int      `json:"servings"`
	SourceURL        string   `json:"sourceUrl"`
	Summary          string   `json:"summary"`
	Cuisines         []string `json:"cuisines"`
	Diets            []string `json:"diets"`
	SpoonacularScore float64  `json:"spoonacularScore"`
}

// searchResponse is the top-level JSON response from complexSearch.
type searchResponse struct {
	Results      []apiRecipe `json:"results"`
	TotalResults int         `json:"totalResults"`
}

// Client is a Spoonacular API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Client with a 10-second timeout.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Search fetches recipes from Spoonacular complexSearch endpoint.
// offset controls pagination; number is how many to request.
func (c *Client) Search(ctx context.Context, query string, offset, number int) ([]Recipe, error) {
	const baseURL = "https://api.spoonacular.com/recipes/complexSearch"

	params := url.Values{}
	params.Set("apiKey", c.apiKey)
	params.Set("query", query)
	params.Set("number", strconv.Itoa(number))
	params.Set("offset", strconv.Itoa(offset))
	params.Set("addRecipeInformation", "true")
	params.Set("instructionsRequired", "true")
	params.Set("fillIngredients", "false")
	params.Set("addRecipeNutrition", "false")
	params.Set("sort", "popularity")

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spoonacular: unexpected status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("spoonacular: decode response: %w", err)
	}

	recipes := make([]Recipe, 0, len(sr.Results))
	for _, r := range sr.Results {
		recipes = append(recipes, Recipe{
			ID:               r.ID,
			Title:            r.Title,
			Image:            r.Image,
			ReadyInMinutes:   r.ReadyInMinutes,
			Servings:         r.Servings,
			SourceURL:        r.SourceURL,
			Summary:          r.Summary,
			Cuisines:         r.Cuisines,
			Diets:            r.Diets,
			SpoonacularScore: r.SpoonacularScore,
		})
	}

	return recipes, nil
}

// FanOut runs one goroutine per search term, merges and deduplicates results.
// page 0 = offset 0, page 1 = offset perTerm, etc.
// perTerm = ceil(target / len(terms))
// seen contains IDs to exclude (for deduplication across calls).
// Results are merged, deduplicated by ID, trimmed to target length.
// Terms that error are skipped (partial results are fine for a personal tool).
func (c *Client) FanOut(ctx context.Context, terms []string, page, target int, seen map[int]bool) ([]Recipe, error) {
	if len(terms) == 0 {
		return nil, nil
	}

	// Integer ceiling division: ceil(target / len(terms))
	perTerm := (target + len(terms) - 1) / len(terms)

	type result struct {
		recipes []Recipe
		err     error
	}

	results := make([]result, len(terms))
	var wg sync.WaitGroup

	for i, term := range terms {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			recipes, err := c.Search(ctx, t, page*perTerm, perTerm)
			results[idx] = result{recipes: recipes, err: err}
		}(i, term)
	}

	wg.Wait()

	// Merge, deduplicate, and collect errors.
	var merged []Recipe
	var lastErr error
	errCount := 0
	// Track IDs we've already added in this call (plus caller-supplied seen).
	localSeen := make(map[int]bool)

	for _, r := range results {
		if r.err != nil {
			lastErr = r.err
			errCount++
			continue
		}
		for _, recipe := range r.recipes {
			if seen[recipe.ID] || localSeen[recipe.ID] {
				continue
			}
			localSeen[recipe.ID] = true
			merged = append(merged, recipe)
		}
	}

	// If we got some results, partial success is fine.
	if len(merged) > 0 {
		if len(merged) > target {
			merged = merged[:target]
		}
		return merged, nil
	}

	// All terms errored.
	if errCount == len(terms) && lastErr != nil {
		return nil, lastErr
	}

	// No results but not all errored (e.g., empty responses).
	return merged, nil
}
