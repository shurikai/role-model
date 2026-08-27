package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// RequestTimeout bounds a single model call.
//
// There was none: the SDK was constructed with an API key and nothing else,
// and Complete passes a request context that carried no deadline of its own.
//
// Sized for the slowest legitimate call rather than the typical one. Career
// extraction is capped at 16384 output tokens, and the fit narrative and the
// 2a/2b generation pair are smaller but still measured in tens of seconds. A
// tight timeout here does not protect anything — it just fails the pipeline's
// normal path. It must stay below the server's WriteTimeout so a stalled call
// surfaces as an error from here.
const RequestTimeout = 3 * time.Minute

type Client struct {
	api   anthropic.Client
	model anthropic.Model
}

func NewClient(apiKey string) *Client {
	return &Client{
		api: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(RequestTimeout),
		),
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
