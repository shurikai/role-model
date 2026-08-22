package renderer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// Client calls the docx-renderer service's /render endpoint, which takes the
// intermediate resume JSON document (see /schema/resume.v2.json) and returns
// a rendered .docx file. The renderer is stateless and owns document output
// only; this client does not interpret or validate the JSON it sends.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// RenderError wraps a non-2xx response from the renderer, so callers can
// distinguish "renderer reachable but rejected the input" from network/
// connection failures.
type RenderError struct {
	StatusCode int
	Body       string
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("renderer returned status %d: %s", e.StatusCode, e.Body)
}

// Render POSTs resumeJSON (the structured_output column of a resume_versions
// row) to the renderer and returns the resulting .docx bytes.
func (c *Client) Render(ctx context.Context, resumeJSON []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/render", bytes.NewReader(resumeJSON))
	if err != nil {
		return nil, fmt.Errorf("build render request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call renderer: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read renderer response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &RenderError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return body, nil
}
