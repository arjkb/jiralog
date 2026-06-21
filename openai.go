package main

import (
	"context"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type OpenAIProvider struct {
	client openai.Client
	model  string
}

func NewOpenAIProvider(key string, model string) *OpenAIProvider {
	return &OpenAIProvider{
		client: openai.NewClient(option.WithAPIKey(key)),
		model:  model,
	}
}

func (p *OpenAIProvider) Summarize(ctx context.Context, query string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := p.client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(query)},
		Model: p.model,
	})

	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}
