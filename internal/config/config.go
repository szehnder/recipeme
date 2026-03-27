package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	Prompt         string // positional args joined: recipeme "something my picky kid would love"
	SpoonacularKey string // env: SPOONACULAR_API_KEY (required)
	AnthropicKey   string // env: ANTHROPIC_API_KEY (optional)
	GeminiKey      string // env: GEMINI_API_KEY (optional)
	LLMProvider    string // env: RECIPEME_LLM_PROVIDER, values: "anthropic"|"gemini", default: auto-detect
	VaultPath      string // env: RECIPEME_VAULT_PATH, flag: --vault, default: /home/szehnder/Documents/Personal
	NoBrowser      bool   // env: RECIPEME_NO_BROWSER=1, flag: --no-browser
	Port           int    // env: RECIPEME_PORT, flag: --port, default: 0 (random)
}

// LoadConfig loads configuration from environment variables and command-line flags.
// Positional arguments are joined to form the Prompt.
// Returns an error if required fields are missing.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		VaultPath: "/home/szehnder/Documents/Personal", // default
		Port:      0,                                   // default (random)
	}

	// Define flags
	var vault string
	var port int
	var noBrowser bool

	flag.StringVar(&vault, "vault", "", "vault path (overrides RECIPEME_VAULT_PATH)")
	flag.IntVar(&port, "port", 0, "port to listen on (0 = random)")
	flag.BoolVar(&noBrowser, "no-browser", false, "do not open browser")

	// Parse flags (this consumes all flags from os.Args)
	flag.Parse()

	// Get remaining positional arguments and join them into a prompt
	cfg.Prompt = strings.Join(flag.Args(), " ")

	// Load from environment variables
	cfg.SpoonacularKey = os.Getenv("SPOONACULAR_API_KEY")
	cfg.AnthropicKey = os.Getenv("ANTHROPIC_API_KEY")
	cfg.GeminiKey = os.Getenv("GEMINI_API_KEY")
	cfg.LLMProvider = os.Getenv("RECIPEME_LLM_PROVIDER")
	cfg.NoBrowser = os.Getenv("RECIPEME_NO_BROWSER") == "1"

	// Load VaultPath from env (can be overridden by flag)
	if vaultEnv := os.Getenv("RECIPEME_VAULT_PATH"); vaultEnv != "" {
		cfg.VaultPath = vaultEnv
	}

	// Override VaultPath with flag if provided
	if vault != "" {
		cfg.VaultPath = vault
	}

	// Load Port from env (can be overridden by flag)
	if portEnv := os.Getenv("RECIPEME_PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil {
			cfg.Port = p
		}
	}

	// Override Port with flag if provided
	if port != 0 {
		cfg.Port = port
	}

	// Override NoBrowser with flag if provided
	if noBrowser {
		cfg.NoBrowser = true
	}

	// Validate required fields
	if cfg.SpoonacularKey == "" {
		return nil, fmt.Errorf("SPOONACULAR_API_KEY is required")
	}

	return cfg, nil
}
