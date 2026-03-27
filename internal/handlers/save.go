package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/szehnder/recipeme/internal/spoonacular"
	"github.com/szehnder/recipeme/internal/vault"
)

// VaultWriter abstracts writing recipe sessions to disk.
type VaultWriter interface {
	WriteSession(s vault.Session) (filePath string, err error)
}

// saveRequest is the JSON body expected by POST /api/save.
type saveRequest struct {
	Prompt      string               `json:"prompt"`
	SearchTerms []string             `json:"searchTerms"`
	Saved       []spoonacular.Recipe `json:"saved"`
}

// saveResponse is the JSON body returned on a successful save.
type saveResponse struct {
	FilePath string `json:"filePath"`
	Count    int    `json:"count"`
}

// toVaultRecipe converts a spoonacular.Recipe to a vault.Recipe.
func toVaultRecipe(r spoonacular.Recipe) vault.Recipe {
	return vault.Recipe{
		ID:               r.ID,
		Title:            r.Title,
		Image:            r.Image,
		ReadyInMinutes:   r.ReadyInMinutes,
		Servings:         r.Servings,
		SourceURL:        r.SourceURL,
		Summary:          r.Summary,
		Cuisines:         r.Cuisines,
		Diets:            r.Diets,
		SpoonacularScore: r.SpoonacularScore,
	}
}

// SaveHandler handles POST /api/save.
// It writes the saved recipes to the vault and schedules a graceful shutdown.
func SaveHandler(v VaultWriter, vaultPath string, shutdown chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request body")
			return
		}

		vaultRecipes := make([]vault.Recipe, len(req.Saved))
		for i, r := range req.Saved {
			vaultRecipes[i] = toVaultRecipe(r)
		}

		session := vault.Session{
			Prompt:      req.Prompt,
			SearchTerms: req.SearchTerms,
			Recipes:     vaultRecipes,
			VaultPath:   vaultPath,
		}

		filePath, err := v.WriteSession(session)
		if err != nil {
			errorJSON(w, http.StatusInternalServerError, "Failed to save recipes")
			return
		}

		writeJSON(w, http.StatusOK, saveResponse{
			FilePath: filePath,
			Count:    len(req.Saved),
		})

		// Trigger graceful shutdown after the response has been flushed.
		// The 1500ms delay ensures the HTTP response is fully sent before shutdown.
		go func() {
			time.Sleep(1500 * time.Millisecond)
			// Guard against double-close using a context.
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			select {
			case <-shutdown:
				// Already closed — nothing to do.
			case <-ctx.Done():
				// Timed out waiting — close anyway.
				close(shutdown)
			default:
				close(shutdown)
			}
		}()
	}
}
