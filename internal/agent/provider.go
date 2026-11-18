package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/anthropicmodel"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"
)

func provider(ctx context.Context, name, modelName string) (model.LLM, error) {
	if strings.TrimSpace(modelName) == "" {
		return nil, errors.New("an explicit model name is required")
	}
	switch name {
	case "openai":
		if os.Getenv("OPENAI_API_KEY") == "" {
			return nil, errors.New("openai requires OPENAI_API_KEY")
		}
		return openaimodel.NewModel(ctx, modelName, &openaimodel.ClientConfig{
			API: openaimodel.APIChatCompletions,
		})
	case "gemini":
		key := os.Getenv("GOOGLE_API_KEY")
		if key == "" {
			key = os.Getenv("GEMINI_API_KEY")
		}
		if key == "" {
			return nil, errors.New("gemini requires GOOGLE_API_KEY or GEMINI_API_KEY")
		}
		return gemini.NewModel(ctx, modelName, &genai.ClientConfig{
			APIKey: key, Backend: genai.BackendGeminiAPI,
		})
	case "anthropic":
		key, token := os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_AUTH_TOKEN")
		if key == "" && token == "" {
			return nil, errors.New("anthropic requires ANTHROPIC_API_KEY or ANTHROPIC_AUTH_TOKEN")
		}
		return anthropicmodel.NewModel(ctx, modelName, anthropicmodel.ClientConfig{
			APIKey: key, AuthToken: token, BaseURL: os.Getenv("ANTHROPIC_BASE_URL"),
		})
	default:
		return nil, fmt.Errorf("unsupported planner provider %q; use openai, gemini, or anthropic", name)
	}
}
