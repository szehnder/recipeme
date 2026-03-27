package ai

import (
	"testing"
)

func TestParseTerms(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTerms []string
		wantErr   bool
	}{
		{
			name:      "valid JSON array",
			input:     `["mac and cheese", "pepperoni pizza", "spaghetti bolognese"]`,
			wantTerms: []string{"mac and cheese", "pepperoni pizza", "spaghetti bolognese"},
			wantErr:   false,
		},
		{
			name:      "valid JSON array with whitespace",
			input:     `  ["chicken soup", "beef stew"]  `,
			wantTerms: []string{"chicken soup", "beef stew"},
			wantErr:   false,
		},
		{
			name:      "single element JSON array",
			input:     `["tacos"]`,
			wantTerms: []string{"tacos"},
			wantErr:   false,
		},
		{
			name:      "comma-separated fallback",
			input:     "mac and cheese, pepperoni pizza, spaghetti bolognese",
			wantTerms: []string{"mac and cheese", "pepperoni pizza", "spaghetti bolognese"},
			wantErr:   false,
		},
		{
			name:      "comma-separated with extra whitespace",
			input:     "  chicken soup ,  beef stew  , tacos  ",
			wantTerms: []string{"chicken soup", "beef stew", "tacos"},
			wantErr:   false,
		},
		{
			name:      "single term comma fallback",
			input:     "chicken noodle soup",
			wantTerms: []string{"chicken noodle soup"},
			wantErr:   false,
		},
		{
			name:    "empty string returns error",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only returns error",
			input:   "   ",
			wantErr: true,
		},
		{
			name:      "JSON array with 5 elements",
			input:     `["mac and cheese", "pepperoni pizza", "spaghetti bolognese", "chicken alfredo", "grilled cheese"]`,
			wantTerms: []string{"mac and cheese", "pepperoni pizza", "spaghetti bolognese", "chicken alfredo", "grilled cheese"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTerms(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTerms(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.wantTerms) {
					t.Errorf("parseTerms(%q) returned %d terms, want %d: got %v, want %v",
						tt.input, len(got), len(tt.wantTerms), got, tt.wantTerms)
					return
				}
				for i, term := range got {
					if term != tt.wantTerms[i] {
						t.Errorf("parseTerms(%q)[%d] = %q, want %q", tt.input, i, term, tt.wantTerms[i])
					}
				}
			}
		})
	}
}
