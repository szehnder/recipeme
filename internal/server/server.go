package server

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"

	"github.com/szehnder/recipeme/internal/handlers"
)

// Server wraps the standard HTTP server with graceful shutdown support.
type Server struct {
	httpServer *http.Server
	shutdown   chan struct{}
}

// New creates and configures the HTTP server with all routes registered.
// staticFS is the embedded filesystem for serving the UI (may be nil).
// shutdown is closed by the save handler to trigger graceful shutdown.
func New(
	ai handlers.LLMProvider,
	sp handlers.SpoonacularClient,
	v handlers.VaultWriter,
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
	mux.HandleFunc("GET /api/recipes", handlers.RecipesHandler(ai, sp))
	mux.HandleFunc("POST /api/recipes/more", handlers.MoreHandler(sp))
	mux.HandleFunc("POST /api/save", handlers.SaveHandler(v, shutdown))

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

// Shutdown gracefully drains in-flight requests using the caller-provided context.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
