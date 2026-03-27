package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/szehnder/recipeme/internal/handlers"
	"github.com/szehnder/recipeme/internal/spoonacular"
	"github.com/szehnder/recipeme/internal/vault"
)

// --- Stub implementations ---

type stubLLM struct {
	terms []string
	err   error
}

func (s *stubLLM) ProcessPrompt(_ context.Context, _ string) ([]string, error) {
	return s.terms, s.err
}

type stubSpoonacular struct {
	recipes []spoonacular.Recipe
	err     error
}

func (s *stubSpoonacular) FanOut(_ context.Context, _ []string, _, _ int, _ map[int]bool) ([]spoonacular.Recipe, error) {
	return s.recipes, s.err
}

type stubVault struct {
	path string
	err  error
}

func (s *stubVault) WriteSession(_ vault.Session) (string, error) {
	return s.path, s.err
}

// --- RecipesHandler tests ---

func TestRecipesHandler_MissingQ_Returns400(t *testing.T) {
	h := handlers.RecipesHandler(&stubLLM{}, &stubSpoonacular{})
	req := httptest.NewRequest(http.MethodGet, "/api/recipes", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message in response body")
	}
}

func TestRecipesHandler_LLMError_Returns500(t *testing.T) {
	h := handlers.RecipesHandler(
		&stubLLM{err: context.DeadlineExceeded},
		&stubSpoonacular{},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?q=pasta", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRecipesHandler_SpoonacularError_Returns500(t *testing.T) {
	h := handlers.RecipesHandler(
		&stubLLM{terms: []string{"pasta"}},
		&stubSpoonacular{err: context.DeadlineExceeded},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?q=pasta", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRecipesHandler_Success_ReturnsJSON(t *testing.T) {
	recipes := []spoonacular.Recipe{{ID: 1, Title: "Pasta"}}
	h := handlers.RecipesHandler(
		&stubLLM{terms: []string{"pasta"}},
		&stubSpoonacular{recipes: recipes},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?q=pasta", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["prompt"] != "pasta" {
		t.Errorf("expected prompt=pasta, got %v", body["prompt"])
	}
	if _, ok := body["recipes"]; !ok {
		t.Error("expected 'recipes' key in response")
	}
	if _, ok := body["buffer"]; !ok {
		t.Error("expected 'buffer' key in response")
	}
}

// --- MoreHandler tests ---

func TestMoreHandler_BadJSON_Returns400(t *testing.T) {
	h := handlers.MoreHandler(&stubSpoonacular{})
	req := httptest.NewRequest(http.MethodPost, "/api/recipes/more", strings.NewReader("{bad json}"))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message in response body")
	}
}

func TestMoreHandler_Success_ReturnsJSON(t *testing.T) {
	recipes := []spoonacular.Recipe{{ID: 2, Title: "Pizza"}}
	h := handlers.MoreHandler(&stubSpoonacular{recipes: recipes})

	body, _ := json.Marshal(map[string]any{
		"searchTerms": []string{"pizza"},
		"page":        1,
		"seen":        []int{1},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/recipes/more", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if _, ok := resp["recipes"]; !ok {
		t.Error("expected 'recipes' key in response")
	}
}

// --- SaveHandler tests ---

func TestSaveHandler_TriggersShutdown(t *testing.T) {
	shutdown := make(chan struct{})
	h := handlers.SaveHandler(&stubVault{path: "/tmp/test.md"}, shutdown)

	body, _ := json.Marshal(map[string]any{
		"prompt":      "pasta",
		"searchTerms": []string{"pasta"},
		"saved":       []spoonacular.Recipe{{ID: 1, Title: "Pasta"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/save", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Shutdown should be triggered within 2 seconds.
	select {
	case <-shutdown:
		// Expected: shutdown was closed.
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown channel was not closed within 2s after save")
	}
}

func TestSaveHandler_VaultError_Returns500(t *testing.T) {
	shutdown := make(chan struct{})
	h := handlers.SaveHandler(&stubVault{err: context.DeadlineExceeded}, shutdown)

	body, _ := json.Marshal(map[string]any{
		"prompt":      "pasta",
		"searchTerms": []string{"pasta"},
		"saved":       []spoonacular.Recipe{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/save", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestSaveHandler_BadJSON_Returns400(t *testing.T) {
	shutdown := make(chan struct{})
	h := handlers.SaveHandler(&stubVault{path: "/tmp/test.md"}, shutdown)

	req := httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader("{bad json}"))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSaveHandler_ReturnsFilePathAndCount(t *testing.T) {
	shutdown := make(chan struct{})
	h := handlers.SaveHandler(&stubVault{path: "/vault/Recipes/test.md"}, shutdown)

	body, _ := json.Marshal(map[string]any{
		"prompt":      "quick meals",
		"searchTerms": []string{"tacos", "soup"},
		"saved": []spoonacular.Recipe{
			{ID: 10, Title: "Tacos"},
			{ID: 11, Title: "Soup"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/save", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if resp["filePath"] != "/vault/Recipes/test.md" {
		t.Errorf("unexpected filePath: %v", resp["filePath"])
	}
	if resp["count"] != float64(2) {
		t.Errorf("unexpected count: %v", resp["count"])
	}

	// Drain shutdown to avoid test goroutine leak.
	select {
	case <-shutdown:
	case <-time.After(2 * time.Second):
	}
}
