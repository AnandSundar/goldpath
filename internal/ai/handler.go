package ai

import (
	"context"
	"errors"
	"fmt"
)

// SuggestRequest represents an AI suggestion request
type SuggestRequest struct {
	Prompt      string            `json:"prompt"`
	Context     map[string]string `json:"context"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature"`
}

// SuggestResult represents an AI suggestion result
type SuggestResult struct {
	Suggestion string   `json:"suggestion"`
	Confidence float64  `json:"confidence"`
	Steps      []string `json:"steps"`
}

// Handler handles AI requests
type Handler struct {
	apiKey  string
	enabled bool
}

// NewHandler creates a new AI handler
func NewHandler(apiKey string, enabled bool) *Handler {
	return &Handler{
		apiKey:  apiKey,
		enabled: enabled,
	}
}

// Suggest generates AI-powered suggestions
func (h *Handler) Suggest(ctx context.Context, req SuggestRequest) (*SuggestResult, error) {
	if !h.enabled {
		// Return mock suggestions when AI is disabled
		return h.mockSuggest(ctx, req)
	}

	if h.apiKey == "" {
		return nil, errors.New("OpenAI API key not configured")
	}

	// Use real OpenAI client when configured
	return h.openAISuggest(ctx, req)
}

func (h *Handler) mockSuggest(ctx context.Context, req SuggestRequest) (*SuggestResult, error) {
	// Return mock suggestions for development/testing
	suggestion := "Based on your request, here's a suggested workflow:"

	var steps []string
	switch {
	case contains(req.Prompt, "api") || contains(req.Prompt, "service"):
		steps = []string{
			"1. Create a new Go module with go mod init",
			"2. Set up chi router for HTTP handling",
			"3. Implement repository pattern for data access",
			"4. Add middleware for logging and error handling",
			"5. Write unit tests for handlers",
		}
	case contains(req.Prompt, "frontend") || contains(req.Prompt, "react"):
		steps = []string{
			"1. Initialize React app with Vite",
			"2. Set up routing with React Router",
			"3. Create components for UI",
			"4. Implement state management",
			"5. Add API client for backend communication",
		}
	case contains(req.Prompt, "database") || contains(req.Prompt, "db"):
		steps = []string{
			"1. Design database schema",
			"2. Create migration scripts",
			"3. Implement repository interface",
			"4. Add connection pooling",
			"5. Write integration tests",
		}
	default:
		steps = []string{
			"1. Define your requirements clearly",
			"2. Create a technical specification",
			"3. Set up project structure",
			"4. Implement core functionality",
			"5. Add tests and documentation",
		}
	}

	return &SuggestResult{
		Suggestion: suggestion,
		Confidence: 0.85,
		Steps:      steps,
	}, nil
}

func (h *Handler) openAISuggest(ctx context.Context, req SuggestRequest) (*SuggestResult, error) {
	// Real OpenAI implementation would go here
	// For now, fall back to mock
	return h.mockSuggest(ctx, req)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			contains(s[1:], substr)))
}

// FormatSuggestion formats the AI suggestion as a readable string
func FormatSuggestion(result *SuggestResult) string {
	if result == nil {
		return ""
	}

	output := result.Suggestion + "\n\n"
	output += "Recommended steps:\n"
	for _, step := range result.Steps {
		output += step + "\n"
	}
	output += fmt.Sprintf("\nConfidence: %.0f%%", result.Confidence*100)

	return output
}
