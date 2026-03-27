package spoonacular

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// newTestClient creates a Client whose httpClient points at the given test server URL.
func newTestClient(serverURL string) *Client {
	c := NewClient("test-api-key")
	c.httpClient = &http.Client{}
	// We'll override the base URL in tests by using a custom transport that rewrites the host.
	// Simpler approach: just swap the http.Client transport.
	_ = serverURL
	return c
}

// makeServer creates an httptest.Server that serves the provided JSON body with the given status code.
func makeServer(t *testing.T, status int, body interface{}) *httptest.Server {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal test body: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}))
}

// clientForServer builds a Client that routes all requests to the test server,
// by replacing the base URL via a round-tripper that rewrites the host.
func clientForServer(srv *httptest.Server) *Client {
	c := NewClient("test-api-key")
	c.httpClient = srv.Client()
	// Replace transport with one that swaps the scheme+host to the test server.
	c.httpClient.Transport = &hostRewriter{
		target:    srv.URL,
		transport: http.DefaultTransport,
	}
	return c
}

// hostRewriter is an http.RoundTripper that rewrites the scheme and host of every
// request to the given target URL, so the client hits our test server instead of
// the real Spoonacular API.
type hostRewriter struct {
	target    string // e.g. "http://127.0.0.1:PORT"
	transport http.RoundTripper
}

func (rw *hostRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid mutating the original.
	clone := req.Clone(req.Context())
	// Parse target and overwrite scheme + host.
	parsed, _ := http.NewRequest(http.MethodGet, rw.target, nil)
	clone.URL.Scheme = parsed.URL.Scheme
	clone.URL.Host = parsed.URL.Host
	// Keep the original path + query so the client code is exercised normally.
	return rw.transport.RoundTrip(clone)
}

// --- Search tests ---

func TestSearch_ParsesResponse(t *testing.T) {
	body := searchResponse{
		TotalResults: 1,
		Results: []apiRecipe{
			{
				ID:               716429,
				Title:            "Pasta with Garlic",
				Image:            "https://example.com/pasta.jpg",
				ReadyInMinutes:   30,
				Servings:         4,
				SourceURL:        "https://example.com/recipe",
				Summary:          "A <b>delicious</b> pasta.",
				Cuisines:         []string{"Italian"},
				Diets:            []string{"vegetarian"},
				SpoonacularScore: 83.2,
			},
		},
	}

	srv := makeServer(t, http.StatusOK, body)
	defer srv.Close()

	c := clientForServer(srv)
	recipes, err := c.Search(context.Background(), "pasta", 0, 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(recipes) != 1 {
		t.Fatalf("expected 1 recipe, got %d", len(recipes))
	}

	r := recipes[0]
	if r.ID != 716429 {
		t.Errorf("ID: want 716429, got %d", r.ID)
	}
	if r.Title != "Pasta with Garlic" {
		t.Errorf("Title: want %q, got %q", "Pasta with Garlic", r.Title)
	}
	if r.ReadyInMinutes != 30 {
		t.Errorf("ReadyInMinutes: want 30, got %d", r.ReadyInMinutes)
	}
	if r.Servings != 4 {
		t.Errorf("Servings: want 4, got %d", r.Servings)
	}
	if r.SourceURL != "https://example.com/recipe" {
		t.Errorf("SourceURL: want %q, got %q", "https://example.com/recipe", r.SourceURL)
	}
	if r.SpoonacularScore != 83.2 {
		t.Errorf("SpoonacularScore: want 83.2, got %f", r.SpoonacularScore)
	}
	if len(r.Cuisines) != 1 || r.Cuisines[0] != "Italian" {
		t.Errorf("Cuisines: want [Italian], got %v", r.Cuisines)
	}
	if len(r.Diets) != 1 || r.Diets[0] != "vegetarian" {
		t.Errorf("Diets: want [vegetarian], got %v", r.Diets)
	}
}

func TestSearch_EmptyResults(t *testing.T) {
	body := searchResponse{TotalResults: 0, Results: []apiRecipe{}}
	srv := makeServer(t, http.StatusOK, body)
	defer srv.Close()

	c := clientForServer(srv)
	recipes, err := c.Search(context.Background(), "xyzzy", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 0 {
		t.Errorf("expected 0 recipes, got %d", len(recipes))
	}
}

func TestSearch_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := clientForServer(srv)
	_, err := c.Search(context.Background(), "pasta", 0, 10)
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

func TestSearch_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := clientForServer(srv)
	_, err := c.Search(context.Background(), "pasta", 0, 10)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSearch_QueryParamsIncluded(t *testing.T) {
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
		body := searchResponse{Results: []apiRecipe{}}
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := clientForServer(srv)
	_, err := c.Search(context.Background(), "chicken soup", 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q := captured.URL.Query()
	if got := q.Get("query"); got != "chicken soup" {
		t.Errorf("query param: want %q, got %q", "chicken soup", got)
	}
	if got := q.Get("offset"); got != "10" {
		t.Errorf("offset param: want %q, got %q", "10", got)
	}
	if got := q.Get("number"); got != "5" {
		t.Errorf("number param: want %q, got %q", "5", got)
	}
	if got := q.Get("addRecipeInformation"); got != "true" {
		t.Errorf("addRecipeInformation param: want %q, got %q", "true", got)
	}
	if got := q.Get("sort"); got != "popularity" {
		t.Errorf("sort param: want %q, got %q", "popularity", got)
	}
	if got := q.Get("apiKey"); got != "test-api-key" {
		t.Errorf("apiKey param: want %q, got %q", "test-api-key", got)
	}
}

// --- FanOut tests ---

// multiTermServer returns a server that serves different recipe sets based on the "query" param.
func multiTermServer(t *testing.T, responses map[string][]apiRecipe) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		recipes, ok := responses[query]
		if !ok {
			recipes = []apiRecipe{}
		}
		body := searchResponse{Results: recipes, TotalResults: len(recipes)}
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
}

func TestFanOut_DeduplicationByID(t *testing.T) {
	// Recipe 1 appears in both term results; should only appear once.
	sharedRecipe := apiRecipe{ID: 1, Title: "Shared", Cuisines: []string{}, Diets: []string{}}
	uniqueA := apiRecipe{ID: 2, Title: "Only A", Cuisines: []string{}, Diets: []string{}}
	uniqueB := apiRecipe{ID: 3, Title: "Only B", Cuisines: []string{}, Diets: []string{}}

	srv := multiTermServer(t, map[string][]apiRecipe{
		"termA": {sharedRecipe, uniqueA},
		"termB": {sharedRecipe, uniqueB},
	})
	defer srv.Close()

	c := clientForServer(srv)
	recipes, err := c.FanOut(context.Background(), []string{"termA", "termB"}, 0, 10, map[int]bool{})
	if err != nil {
		t.Fatalf("FanOut returned error: %v", err)
	}

	// We expect 3 unique recipes (IDs 1, 2, 3).
	seen := map[int]bool{}
	for _, r := range recipes {
		if seen[r.ID] {
			t.Errorf("duplicate recipe ID %d in FanOut results", r.ID)
		}
		seen[r.ID] = true
	}
	if len(recipes) != 3 {
		t.Errorf("expected 3 unique recipes, got %d", len(recipes))
	}
}

func TestFanOut_ExcludesSeenIDs(t *testing.T) {
	recipe1 := apiRecipe{ID: 10, Title: "Recipe 10", Cuisines: []string{}, Diets: []string{}}
	recipe2 := apiRecipe{ID: 20, Title: "Recipe 20", Cuisines: []string{}, Diets: []string{}}

	srv := multiTermServer(t, map[string][]apiRecipe{
		"pasta": {recipe1, recipe2},
	})
	defer srv.Close()

	c := clientForServer(srv)
	// Caller marks ID 10 as already seen.
	preSeenIDs := map[int]bool{10: true}
	recipes, err := c.FanOut(context.Background(), []string{"pasta"}, 0, 10, preSeenIDs)
	if err != nil {
		t.Fatalf("FanOut returned error: %v", err)
	}
	for _, r := range recipes {
		if r.ID == 10 {
			t.Error("recipe with pre-seen ID 10 should have been excluded")
		}
	}
	if len(recipes) != 1 || recipes[0].ID != 20 {
		t.Errorf("expected only recipe 20, got %+v", recipes)
	}
}

func TestFanOut_TrimsToTarget(t *testing.T) {
	var many []apiRecipe
	for i := 1; i <= 10; i++ {
		many = append(many, apiRecipe{ID: i, Title: "Recipe", Cuisines: []string{}, Diets: []string{}})
	}

	srv := multiTermServer(t, map[string][]apiRecipe{
		"food": many,
	})
	defer srv.Close()

	c := clientForServer(srv)
	recipes, err := c.FanOut(context.Background(), []string{"food"}, 0, 5, map[int]bool{})
	if err != nil {
		t.Fatalf("FanOut returned error: %v", err)
	}
	if len(recipes) != 5 {
		t.Errorf("expected results trimmed to 5, got %d", len(recipes))
	}
}

func TestFanOut_AllTermsError_ReturnsError(t *testing.T) {
	// Server returns 500 for all requests.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := clientForServer(srv)
	recipes, err := c.FanOut(context.Background(), []string{"termA", "termB"}, 0, 10, map[int]bool{})
	if err == nil {
		t.Fatal("expected error when all terms fail, got nil")
	}
	if recipes != nil {
		t.Errorf("expected nil recipes when all terms fail, got %v", recipes)
	}
}

func TestFanOut_PartialErrors_ReturnsResults(t *testing.T) {
	// "good" returns results; "bad" returns a 500.
	goodRecipe := apiRecipe{ID: 42, Title: "Good Recipe", Cuisines: []string{}, Diets: []string{}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		if q == "bad" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body := searchResponse{Results: []apiRecipe{goodRecipe}, TotalResults: 1}
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := clientForServer(srv)
	recipes, err := c.FanOut(context.Background(), []string{"good", "bad"}, 0, 10, map[int]bool{})
	if err != nil {
		t.Fatalf("expected no error for partial success, got: %v", err)
	}
	if len(recipes) != 1 || recipes[0].ID != 42 {
		t.Errorf("expected recipe 42 from partial success, got %+v", recipes)
	}
}

func TestFanOut_EmptyTerms(t *testing.T) {
	c := NewClient("test-key")
	recipes, err := c.FanOut(context.Background(), []string{}, 0, 10, map[int]bool{})
	if err != nil {
		t.Fatalf("unexpected error for empty terms: %v", err)
	}
	if recipes != nil {
		t.Errorf("expected nil for empty terms, got %v", recipes)
	}
}

func TestFanOut_PerTermCeilingDivision(t *testing.T) {
	// 3 terms, target 10 => perTerm = ceil(10/3) = 4
	// Verify via the offset logic: page=1 => offset = 1*4 = 4
	var capturedOffsets []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedOffsets = append(capturedOffsets, r.URL.Query().Get("offset"))
		mu.Unlock()
		body := searchResponse{Results: []apiRecipe{}}
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := clientForServer(srv)
	_, _ = c.FanOut(context.Background(), []string{"a", "b", "c"}, 1, 10, map[int]bool{})

	// Each goroutine should pass offset = page * perTerm = 1 * 4 = 4
	for _, off := range capturedOffsets {
		if off != "4" {
			t.Errorf("expected offset 4, got %q", off)
		}
	}
}
