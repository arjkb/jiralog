package main

import (
	"context"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type OpenAIProvider struct {
	key   string
	model string
}

func (p *OpenAIProvider) Summarize(ctx context.Context, query string) (string, error) {
	client := openai.NewClient(option.WithAPIKey(p.key))
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(query)},
		Model: p.model,
	})

	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}
