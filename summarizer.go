package main

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

// Get the worklog summary
func getWorklogSummary(key string, rawDescription string) (string, error) {
	query := "Summarize into a short paragraph, for adding worklog (just the summary – no headings or formatting or anything. Just a short concise paragraph): " + rawDescription

	client := openai.NewClient(
		option.WithAPIKey(key),
	)

	resp, err := client.Responses.New(context.TODO(), responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(query)},
		Model: openai.ChatModelGPT5_4Nano,
	})

	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}
