// This file contains the method to make the
// request to LLM service provider to summarize the text.

package main

import (
	"context"
	"fmt"
)

type Summarizer interface {
	Summarize(ctx context.Context, query string) (string, error)
}

// Get the worklog summary
func getWorklogSummary(ctx context.Context, key string, model string, prompt string, rawDescription string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("missing key")
	}
	if model == "" {
		return "", fmt.Errorf("missing model")
	}
	if prompt == "" {
		return "", fmt.Errorf("missing prompt")
	}
	if rawDescription == "" {
		return "", fmt.Errorf("missing description")
	}

	query := fmt.Sprintf("%s:\n%q", prompt, rawDescription)

	provider := OpenAIProvider{
		key:   key,
		model: model,
	}

	return provider.Summarize(ctx, query)
}
