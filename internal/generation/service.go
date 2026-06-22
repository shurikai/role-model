package generation

import (
	"github.com/google/uuid"
	"github.com/shurikai/role-model/internal/db"
)

// Service holds dependencies for the resume generation pipeline.
type Service struct {
	q      *db.Queries
	client *Client
}

func NewService(q *db.Queries, client *Client) *Service {
	return &Service{q: q, client: client}
}

// ResumeContext is the assembled career data passed to the generation prompt.
// All active contributions are included; relevance selection is the LLM's job.
type ResumeContext struct {
	Employers []EmployerView `json:"employers"`
}

type EmployerView struct {
	Name      string         `json:"name"`
	Positions []PositionView `json:"positions"`
}

type PositionView struct {
	ID               uuid.UUID          `json:"id"`
	Title            string             `json:"title"`
	IndustryLevel    *string            `json:"industry_level,omitempty"`
	IndustryRole     *string            `json:"industry_role,omitempty"`
	ContextNarrative *string            `json:"context_narrative,omitempty"`
	Location         *string            `json:"location,omitempty"`
	StartedOn        string             `json:"started_on"`
	EndedOn          *string            `json:"ended_on,omitempty"`
	Contributions    []ContributionView `json:"contributions"`
}

type ContributionView struct {
	ID              uuid.UUID `json:"id"`
	Summary         string    `json:"summary"`
	FullDescription string    `json:"full_description"`
	Outcomes        *string   `json:"outcomes,omitempty"`
	ScaleContext    *string   `json:"scale_context,omitempty"`
	Tags            []TagView `json:"tags"`
}

type TagView struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}
