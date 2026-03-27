package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/szehnder/recipeme/internal/ai"
	"github.com/szehnder/recipeme/internal/browser"
	"github.com/szehnder/recipeme/internal/config"
	"github.com/szehnder/recipeme/internal/server"
	"github.com/szehnder/recipeme/internal/spoonacular"
	"github.com/szehnder/recipeme/internal/vault"
	"github.com/szehnder/recipeme/web"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "recipeme: %v\n", err)
		os.Exit(1)
	}
	if cfg.Prompt == "" {
		fmt.Fprintln(os.Stderr, "recipeme: usage: recipeme \"what you want to cook\"")
		os.Exit(1)
	}

	ctx := context.Background()

	llm, err := ai.NewProvider(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "recipeme: %v\n", err)
		os.Exit(1)
	}

	sp := spoonacular.NewClient(cfg.SpoonacularKey)
	vaultWriter := vault.NewWriter(cfg.VaultPath)

	shutdown := make(chan struct{})

	srv := server.New(llm, sp, vaultWriter, cfg.VaultPath, cfg.Port, web.FS(), shutdown)

	port, err := srv.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "recipeme: failed to start server: %v\n", err)
		os.Exit(1)
	}

	openURL := fmt.Sprintf("http://localhost:%d?q=%s", port, url.QueryEscape(cfg.Prompt))
	fmt.Printf("open %s\n", openURL)

	if !cfg.NoBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			if err := browser.Open(openURL); err != nil {
				fmt.Fprintf(os.Stderr, "recipeme: could not open browser: %v\n", err)
			}
		}()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-shutdown:
		fmt.Fprintln(os.Stderr, "recipeme: session saved, shutting down...")
	case <-sigChan:
		fmt.Fprintln(os.Stderr, "recipeme: shutting down...")
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		fmt.Fprintf(os.Stderr, "recipeme: shutdown error: %v\n", err)
	}
	os.Exit(0)
}
