package ai

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/genai"
)

// GeminiProvider calls the Google Gemini API.
// SDK: google.golang.org/genai
// Model: gemini-2.0-flash
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// NewGeminiProvider creates a new GeminiProvider with the given API key.
func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	return &GeminiProvider{client: client, model: "gemini-2.0-flash"}, nil
}

// ProcessPrompt calls Gemini with a 10-second timeout.
// On LLM failure: returns error "Failed to interpret your prompt — please try again"
func (p *GeminiProvider) ProcessPrompt(ctx context.Context, prompt string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := p.client.Models.GenerateContent(ctx, p.model, genai.Text(prompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
		MaxOutputTokens:   256,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to interpret your prompt — please try again: %w", err)
	}

	raw := result.Text()
	terms, err := parseTerms(raw)
	if err != nil {
		return nil, fmt.Errorf("Failed to interpret your prompt — please try again: %w", err)
	}
	return terms, nil
}
