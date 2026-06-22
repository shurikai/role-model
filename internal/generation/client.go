package generation

import (
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
