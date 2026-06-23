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

type ProjectView struct {
	ID            uuid.UUID          `json:"id"`
	Name          string             `json:"name"`
	Tagline       *string            `json:"tagline,omitempty"`
	Role          string             `json:"role"`
	Status        string             `json:"status"`
	StartedOn     *string            `json:"started_on,omitempty"`
	EndedOn       *string            `json:"ended_on,omitempty"`
	RepoURL       *string            `json:"repo_url,omitempty"`
	LiveURL       *string            `json:"live_url,omitempty"`
	WriteupURL    *string            `json:"writeup_url,omitempty"`
	ForceInclude  bool               `json:"force_include"`
	ForceExclude  bool               `json:"force_exclude"`
	Contributions []ContributionView `json:"contributions"`
	Tags          []TagView          `json:"tags"`
}

type EducationView struct {
	Institution  string  `json:"institution"`
	Degree       *string `json:"degree,omitempty"`
	FieldOfStudy *string `json:"field_of_study,omitempty"`
	Graduated    *string `json:"graduated,omitempty"`
	Notes        *string `json:"notes,omitempty"`
}

type CredentialView struct {
	Name          string  `json:"name"`
	Issuer        *string `json:"issuer,omitempty"`
	IssuedOn      *string `json:"issued_on,omitempty"`
	ExpiresOn     *string `json:"expires_on,omitempty"`
	CredentialURL *string `json:"credential_url,omitempty"`
}
