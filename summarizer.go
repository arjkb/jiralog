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

func getProvider(config Config) (Summarizer, error) {
	if config.Aikey == "" {
		return nil, fmt.Errorf("missing key; check config file")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("missing model; check config file")
	}

	switch config.Provider {
	case "openai":
		return &OpenAIProvider{key: config.Aikey, model: config.Model}, nil
	}

	return nil, fmt.Errorf("invalid provider; check config file")
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
