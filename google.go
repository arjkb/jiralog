package main

import (
	"context"

	"google.golang.org/genai"
)

type GoogleAIProvider struct {
	client *genai.Client
	model  string
}

func NewGoogleProvider(ctx context.Context, key string, model string) (*GoogleAIProvider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  key,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	return &GoogleAIProvider{
		client: client,
		model:  model,
	}, nil

}

func (p *GoogleAIProvider) Summarize(ctx context.Context, query string) (string, error) {
	result, err := p.client.Models.GenerateContent(
		ctx,
		p.model,
		genai.Text(query),
		nil,
	)
	if err != nil {
		return "", err
	}

	return result.Text(), nil
}
