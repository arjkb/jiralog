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
