package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// JDSignals is the structured output of signal extraction.
type JDSignals struct {
	// Skills
	RequiredSkills  []string `json:"required_skills"`
	PreferredSkills []string `json:"preferred_skills"`

	// Role classification
	Seniority string `json:"seniority"`
	Domain    string `json:"domain"`
	WorkType  string `json:"work_type"`

	// Culture and preference signals
	CultureSignals []string `json:"culture_signals"`

	// Deprecated: retained for backward compatibility with existing jd_signals rows.
	// Do not use in new code. Will be removed in a future cleanup.
	PrioritySkills   []string `json:"priority_skills,omitempty"`
	DomainVocabulary []string `json:"domain_vocabulary,omitempty"`
}

type extractPromptData struct {
	JobDescription string
}

// ExtractSignals runs JD signal extraction against the given job description text.
func (s *Service) ExtractSignals(ctx context.Context, jdText string) (*JDSignals, error) {
	prompt, err := renderPrompt("jd_extraction.v1.tmpl", extractPromptData{
		JobDescription: jdText,
	})
	if err != nil {
		return nil, err
	}

	msg, err := s.client.api.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 1024,
		Model:     s.client.model,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic messages: %w", err)
	}

	raw, err := extractText(msg)
	if err != nil {
		return nil, err
	}

	var signals JDSignals
	if err := json.Unmarshal([]byte(stripCodeFence(raw)), &signals); err != nil {
		return nil, fmt.Errorf("parse jd signals: %w (raw: %s)", err, raw)
	}

	return &signals, nil
}

// extractText pulls the concatenated text from a message response.
func extractText(msg *anthropic.Message) (string, error) {
	var b strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("no text content in response")
	}
	return b.String(), nil
}

// stripCodeFence removes markdown code fences if the model wrapped its JSON.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
