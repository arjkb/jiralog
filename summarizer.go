// This file contains the method to make the
// request to LLM service provider to summarize the text.

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

// Get the worklog summary
func getWorklogSummary(key string, model string, prompt string, rawDescription string) (string, error) {
	if key == "" || model == "" || prompt == "" || rawDescription == "" {
		var missingCredential string
		if key == "" {
			missingCredential = "key"
		} else if model == "" {
			missingCredential = "model"
		} else if prompt == "" {
			missingCredential = "prompt"
		} else if rawDescription == "" {
			missingCredential = "description"
		} else {
			missingCredential = "(unknown)"
		}

		return "", fmt.Errorf("missing credential: %s", missingCredential)
	}

	query := fmt.Sprintf("%s:\n%q", prompt, rawDescription)

	client := openai.NewClient(
		option.WithAPIKey(key),
	)

	resp, err := client.Responses.New(context.TODO(), responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(query)},
		Model: model,
	})

	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(resp.OutputText(), "\n\n", "\n"), nil
}
