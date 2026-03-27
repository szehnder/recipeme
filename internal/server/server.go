package server

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/szehnder/recipeme/internal/handlers"
	"github.com/szehnder/recipeme/internal/spoonacular"
	"github.com/szehnder/recipeme/internal/vault"
)

// LLMProvider abstracts the AI backend used for prompt interpretation.
type LLMProvider interface {
	ProcessPrompt(ctx context.Context, prompt string) ([]string, error)
}

// SpoonacularClient abstracts the recipe search client.
type SpoonacularClient interface {
	FanOut(ctx context.Context, terms []string, page, target int, seen map[int]bool) ([]spoonacular.Recipe, error)
}

// VaultWriter abstracts writing recipe sessions to disk.
type VaultWriter interface {
	WriteSession(s vault.Session) (filePath string, err error)
}

// Server wraps the standard HTTP server with graceful shutdown support.
type Server struct {
	httpServer *http.Server
	shutdown   chan struct{}
}

// New creates and configures the HTTP server with all routes registered.
// staticFS is the embedded filesystem for serving the UI (may be nil).
// shutdown is closed by the save handler to trigger graceful shutdown.
func New(
	ai LLMProvider,
	sp SpoonacularClient,
	v VaultWriter,
	vaultPath string,
	port int,
	staticFS fs.FS,
	shutdown chan struct{},
) *Server {
	mux := http.NewServeMux()

	// Static file routes.
	if staticFS != nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.ServeFileFS(w, r, staticFS, "index.html")
		})
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "UI not available", http.StatusNotFound)
		})
	}

	// API routes.
	mux.Handle("/api/recipes", handlers.RecipesHandler(ai, sp))
	mux.Handle("/api/recipes/more", handlers.MoreHandler(sp))
	mux.Handle("/api/save", handlers.SaveHandler(v, vaultPath, shutdown))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &Server{
		httpServer: srv,
		shutdown:   shutdown,
	}
}

// Start begins listening and returns the actual port (useful when port=0).
func (s *Server) Start() (int, error) {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return 0, fmt.Errorf("server: listen: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	s.httpServer.Addr = fmt.Sprintf(":%d", port)

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			_ = err
		}
	}()

	return port, nil
}

// Shutdown gracefully drains in-flight requests with a 5-second timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(shutdownCtx)
}
