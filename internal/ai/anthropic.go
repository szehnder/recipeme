package ai

import (
	"context"
	"fmt"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider calls the Anthropic Messages API.
// SDK: github.com/anthropics/anthropic-sdk-go
// Model: claude-sonnet-4-6
type AnthropicProvider struct {
	client *anthropic.Client
}

// NewAnthropicProvider creates a new AnthropicProvider with the given API key.
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{client: &c}
}

// ProcessPrompt calls Anthropic with a 10-second timeout.
// On LLM failure: returns error "Failed to interpret your prompt — please try again"
func (p *AnthropicProvider) ProcessPrompt(ctx context.Context, prompt string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 256,
		System: []anthropic.TextBlockParam{
			{Type: "text", Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to interpret your prompt — please try again: %w", err)
	}

	if len(msg.Content) == 0 {
		return nil, fmt.Errorf("Failed to interpret your prompt — please try again")
	}

	raw := msg.Content[0].AsText().Text
	terms, err := parseTerms(raw)
	if err != nil {
		return nil, fmt.Errorf("Failed to interpret your prompt — please try again: %w", err)
	}
	return terms, nil
}
