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

	// Screening facts a human scans for before considering a role at all.
	ScreeningSummary ScreeningSummary `json:"screening_summary"`

	// Deprecated: retained for backward compatibility with existing jd_signals rows.
	// Do not use in new code. Will be removed in a future cleanup.
	PrioritySkills   []string `json:"priority_skills,omitempty"`
	DomainVocabulary []string `json:"domain_vocabulary,omitempty"`
}

// documentJDSignals is the jd_signals projection embedded in the resume
// document's meta block. It mirrors $defs.jd_signals in schema/resume.v1.json
// exactly — those six fields, no others.
//
// It exists because the document schema is a strict contract
// (additionalProperties: false) while JDSignals is owned by extraction and
// evolves on its own schedule. Assigning the stored jd_signals blob straight
// into meta coupled the two, and broke in both directions at once: 15 stored
// applications carry the deprecated priority_skills/domain_vocabulary and 5
// carry screening_summary, none of which the schema allows. 20 of 31
// applications could not generate at all, each failing on a field the
// document never needed.
//
// The invariant this type buys: adding a field to JDSignals cannot change
// what the document emits. If the document should carry something new, the
// schema declares it first and this struct follows — never the reverse.
type documentJDSignals struct {
	RequiredSkills  []string `json:"required_skills"`
	PreferredSkills []string `json:"preferred_skills"`
	Seniority       string   `json:"seniority"`
	Domain          string   `json:"domain"`
	WorkType        string   `json:"work_type"`
	CultureSignals  []string `json:"culture_signals"`
}

// forDocument projects the signals down to what the document schema declares.
func (s JDSignals) forDocument() documentJDSignals {
	// The same fallback buildSkillsChecklist applies. Pre-migration rows carry
	// the requirement list under priority_skills, so reading RequiredSkills
	// alone would hand 15 stored applications an empty signal block — a
	// silent downgrade, where the data is merely named differently.
	required := s.RequiredSkills
	if len(required) == 0 {
		required = s.PrioritySkills
	}

	return documentJDSignals{
		RequiredSkills:  nonNilStrings(required),
		PreferredSkills: nonNilStrings(s.PreferredSkills),
		Seniority:       s.Seniority,
		Domain:          s.Domain,
		WorkType:        s.WorkType,
		CultureSignals:  nonNilStrings(s.CultureSignals),
	}
}

// nonNilStrings keeps an absent list marshalling as [] rather than null. The
// schema types these as arrays without listing them as required, so an
// omitted key validates but an explicit null does not.
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// ScreeningSummary holds plain-language facts a human would scan a JD for
// before seriously considering it — criteria that don't relate to skills
// match but often decide whether a role is worth pursuing at all. This is
// deliberately descriptive, not classificatory: no fixed enums, because the
// set of things worth flagging is open-ended and person-specific.
//
// It exists because the classificatory fields above could not represent most
// real exclude criteria — audience, timezone, travel, clearance — and every
// new category needed a new field. Describing the role and letting a human
// judge it scales where an exclude taxonomy does not.
type ScreeningSummary struct {
	// e.g. "Providence, RI", "Remote, US", "not stated"
	Location string `json:"location"`
	// e.g. "fully remote", "hybrid, 2 days/week Minneapolis", "onsite"
	WorkArrangement string `json:"work_arrangement"`
	// e.g. "occasional customer site visits", "not mentioned"
	Travel string `json:"travel"`
	// Plain-language description, not constrained to a fixed list — e.g.
	// "defense/autonomous systems", "themed entertainment",
	// "developer telemetry SaaS".
	Industry string `json:"industry"`
	// e.g. "U.S. citizenship + active clearance required", "not mentioned"
	ClearanceCitizenship string `json:"clearance_citizenship"`
	// Free text: anything else identifying or discriminating that isn't a
	// skills match — military-coded language, FedRAMP mentions, "serves
	// internal engineering teams" framing, anonymous postings, unusual comp
	// structure, notable red flags. Empty array if nothing notable.
	OtherFlags []string `json:"other_flags"`
}

type extractPromptData struct {
	JobDescription string
}

// ExtractSignals runs JD signal extraction against the given job description text.
func (s *Service) ExtractSignals(ctx context.Context, jdText string) (*JDSignals, error) {
	prompt, err := renderPrompt(jdExtractionPrompt, extractPromptData{
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
