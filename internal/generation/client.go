package generation

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Client struct {
	api   anthropic.Client
	model anthropic.Model
}

func NewClient(apiKey string) *Client {
	return &Client{
		api:   anthropic.NewClient(option.WithAPIKey(apiKey)),
		model: anthropic.ModelClaudeSonnet4_5_20250929,
	}
}

func (c *Client) ModelName() string {
	return string(c.model)
}

// Complete sends a system prompt plus a single user message and returns the
// response text with any markdown code fence stripped.
func (c *Client) Complete(ctx context.Context, systemPrompt, userContent string, maxTokens int64) (string, error) {
	msg, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: maxTokens,
		Model:     c.model,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userContent)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic messages: %w", err)
	}

	raw, err := extractText(msg)
	if err != nil {
		return "", err
	}

	return stripCodeFence(raw), nil
}
