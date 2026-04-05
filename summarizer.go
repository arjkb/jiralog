// This file contains the method to make the
// request to LLM service provider to summarize the text.

package main

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

// Get the worklog summary
func getWorklogSummary(key string, model string, prompt string, rawDescription string) (string, error) {
	if key == "" || model == "" || prompt == "" || rawDescription == "" {
		return "", nil
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

	return resp.OutputText(), nil
}
