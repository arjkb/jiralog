// This file contains the method to make the
// request to LLM service provider to summarize the text.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

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

	client := openai.NewClient(
		option.WithAPIKey(key),
	)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(query)},
		Model: model,
	})

	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}
