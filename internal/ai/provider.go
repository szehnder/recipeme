package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/szehnder/recipeme/internal/config"
)

// LLMProvider is the single interface both backends implement.
type LLMProvider interface {
	ProcessPrompt(ctx context.Context, prompt string) ([]string, error)
}

// systemPrompt is the shared prompt used by both backends.
const systemPrompt = `You are a recipe search assistant. Given a natural language description of food preferences, generate 3 to 5 specific recipe search terms that would surface great matching recipes on a cooking website. Be creative — vary cuisines, cooking styles, and specific dishes to give broad but relevant results.

Return ONLY a valid JSON array of strings, no explanation, no markdown, no other text.

Example input: "something my picky 8-year-old would love, not spicy, maybe Italian or American comfort food"
Example output: ["mac and cheese", "pepperoni pizza", "spaghetti bolognese", "chicken alfredo", "grilled cheese"]`

// NewProvider constructs the correct backend from config.
// Selection order:
//  1. RECIPEME_LLM_PROVIDER=anthropic|gemini (explicit)
//  2. Auto-detect: ANTHROPIC_API_KEY present → Anthropic; GEMINI_API_KEY present → Gemini
//  3. Error if neither key is set
func NewProvider(ctx context.Context, cfg *config.Config) (LLMProvider, error) {
	switch cfg.LLMProvider {
	case "anthropic":
		if cfg.AnthropicKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is required when RECIPEME_LLM_PROVIDER=anthropic")
		}
		return NewAnthropicProvider(cfg.AnthropicKey), nil
	case "gemini":
		if cfg.GeminiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is required when RECIPEME_LLM_PROVIDER=gemini")
		}
		return NewGeminiProvider(ctx, cfg.GeminiKey)
	case "":
		// Auto-detect
		if cfg.AnthropicKey != "" {
			return NewAnthropicProvider(cfg.AnthropicKey), nil
		}
		if cfg.GeminiKey != "" {
			return NewGeminiProvider(ctx, cfg.GeminiKey)
		}
		return nil, fmt.Errorf("no LLM API key set: provide ANTHROPIC_API_KEY or GEMINI_API_KEY")
	default:
		return nil, fmt.Errorf("unknown LLM provider %q, use 'anthropic' or 'gemini'", cfg.LLMProvider)
	}
}

// parseTerms parses the LLM response into []string.
// Primary: encoding/json into []string
// Fallback: comma-split with whitespace trimming
// Returns error only if fewer than 1 term is recovered
func parseTerms(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)

	// Primary: try JSON array parse
	var terms []string
	if err := json.Unmarshal([]byte(raw), &terms); err == nil {
		// JSON parsed successfully
		if len(terms) == 0 {
			return nil, fmt.Errorf("LLM returned empty search terms")
		}
		return terms, nil
	}

	// Fallback: comma-split (JSON unmarshal failed)
	parts := strings.Split(raw, ",")
	terms = []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			terms = append(terms, p)
		}
	}

	if len(terms) < 1 {
		return nil, fmt.Errorf("failed to parse any search terms from LLM response")
	}
	return terms, nil
}
