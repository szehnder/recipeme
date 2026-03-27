package vault

import (
	"fmt"
	"os"
	"testing"
)

func TestExampleMarkdownOutput(t *testing.T) {
	tmpDir := t.TempDir()

	session := Session{
		Prompt:      "something my picky kid would love",
		SearchTerms: []string{"chicken", "pasta", "easy"},
		Recipes: []Recipe{
			{
				ID:             1,
				Title:          "Creamy Chicken Pasta",
				Image:          "https://spoonacular.com/recipeImages/12345.jpg",
				ReadyInMinutes: 30,
				Servings:       4,
				SourceURL:      "https://spoonacular.com/recipes/12345",
				Summary:        "A <b>delicious</b> and easy chicken pasta that kids love",
				Cuisines:       []string{"Italian", "Mediterranean"},
				Diets:          []string{"Gluten-Free"},
			},
		},
		VaultPath: tmpDir,
	}

	filePath, err := WriteSession(session)
	if err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	fmt.Print("\n=== GENERATED MARKDOWN ===\n")
	fmt.Print(string(content))
	fmt.Print("=== END ===\n")
}
