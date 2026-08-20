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

// SkillView is one claimed skill with its depth signal, as the generation
// prompt sees it.
//
// A TagView and a SkillView are not the same thing and must not be used
// interchangeably. A tag is vocabulary attached to a contribution; a skill is
// a claim the user makes about themselves, carrying proficiency and duration.
// Building the resume's Skills section out of tags treated every technology
// anyone ever touched as an equal claim, and left the prompt no way to tell a
// 25-year expert from a weekend prototype.
type SkillView struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Proficiency string `json:"proficiency"`
	// Nil where the duration was never recorded. Absent, not zero — an
	// unrecorded duration is not evidence of a short one.
	YearsExperience *float64 `json:"years_experience,omitempty"`
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
