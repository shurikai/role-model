package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// CareerExtraction is what stage0_career_extraction.tmpl returns: a whole
// career, with parents named by a label local to the response.
type CareerExtraction struct {
	Employers     []ExtractedEmployer     `json:"employers"`
	Positions     []ExtractedPosition     `json:"positions"`
	Contributions []ExtractedContribution `json:"contributions"`
	Skills        []ExtractedSkill        `json:"skills"`
}

type ExtractedEmployer struct {
	Ref      string  `json:"ref"`
	Name     string  `json:"name"`
	Industry *string `json:"industry"`
}

type ExtractedPosition struct {
	Ref              string  `json:"ref"`
	EmployerRef      string  `json:"employer_ref"`
	Title            string  `json:"title"`
	IndustryRole     *string `json:"industry_role"`
	IndustryLevel    *string `json:"industry_level"`
	Location         *string `json:"location"`
	StartedOn        string  `json:"started_on"`
	EndedOn          *string `json:"ended_on"`
	ContextNarrative *string `json:"context_narrative"`
}

type ExtractedContribution struct {
	PositionRef     string   `json:"position_ref"`
	Summary         string   `json:"summary"`
	FullDescription string   `json:"full_description"`
	Outcomes        *string  `json:"outcomes"`
	ScaleContext    *string  `json:"scale_context"`
	Tags            []tagRef `json:"tags"`
}

type ExtractedSkill struct {
	Category        string   `json:"category"`
	Tag             string   `json:"tag"`
	Proficiency     string   `json:"proficiency"`
	YearsExperience *float64 `json:"years_experience"`
}

// PlannedDraft is one row destined for entity_drafts, with its dependencies
// already resolved from response-local refs to draft ids.
type PlannedDraft struct {
	ID        uuid.UUID
	Kind      string
	Payload   json.RawMessage
	DependsOn []uuid.UUID
}

// PlanDrafts turns an extraction into the drafts that represent it.
//
// It is a pure function on purpose. The half of Stage 0 that talks to a model
// is untestable without spending money and getting a different answer each
// time; the half that decides what rows a response becomes is where the bugs
// that matter live — a contribution attached to the wrong position, a
// dependency edge pointing at nothing, a whole employer silently dropped. That
// half is here, and it is exercised against canned responses.
//
// A reference to a parent that does not exist is an ERROR, not a skipped row.
// Extraction is asked for a whole career at once; a position whose employer_ref
// is a typo would otherwise become an orphan that resolves against nothing, and
// the person would find out by noticing a job missing from their resume months
// later.
func PlanDrafts(x CareerExtraction) ([]PlannedDraft, error) {
	employerIDs := map[string]uuid.UUID{}
	positionIDs := map[string]uuid.UUID{}
	var out []PlannedDraft

	add := func(kind string, id uuid.UUID, payload any, deps ...uuid.UUID) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal %s payload: %w", kind, err)
		}
		if deps == nil {
			deps = []uuid.UUID{}
		}
		out = append(out, PlannedDraft{ID: id, Kind: kind, Payload: raw, DependsOn: deps})
		return nil
	}

	for _, e := range x.Employers {
		if strings.TrimSpace(e.Name) == "" {
			return nil, fmt.Errorf("employer %q has no name", e.Ref)
		}
		if _, dup := employerIDs[e.Ref]; dup {
			return nil, fmt.Errorf("employer ref %q is used twice", e.Ref)
		}
		id := uuid.New()
		employerIDs[e.Ref] = id
		if err := add(KindEmployer, id, employerPayload{
			Name: e.Name, Industry: e.Industry,
		}); err != nil {
			return nil, err
		}
	}

	for _, p := range x.Positions {
		employerID, ok := employerIDs[p.EmployerRef]
		if !ok {
			return nil, fmt.Errorf("position %q names employer %q, which the extraction does not contain",
				p.Title, p.EmployerRef)
		}
		if _, dup := positionIDs[p.Ref]; dup {
			return nil, fmt.Errorf("position ref %q is used twice", p.Ref)
		}
		id := uuid.New()
		positionIDs[p.Ref] = id
		if err := add(KindPosition, id, positionPayload{
			EmployerDraft: &employerID, Title: p.Title,
			IndustryLevel: p.IndustryLevel, IndustryRole: p.IndustryRole,
			Location: p.Location, StartedOn: p.StartedOn, EndedOn: p.EndedOn,
			ContextNarrative: p.ContextNarrative,
		}, employerID); err != nil {
			return nil, err
		}
	}

	for _, c := range x.Contributions {
		positionID, ok := positionIDs[c.PositionRef]
		if !ok {
			return nil, fmt.Errorf("contribution %q names position %q, which the extraction does not contain",
				c.Summary, c.PositionRef)
		}
		if err := add(KindContribution, uuid.New(), contributionPayload{
			PositionDraft: &positionID, Summary: c.Summary,
			FullDescription: c.FullDescription, Outcomes: c.Outcomes,
			ScaleContext: c.ScaleContext, Tags: c.Tags,
		}, positionID); err != nil {
			return nil, err
		}
	}

	// Skills depend on nothing: ResolveOrCreateTag builds the
	// tag -> tag_category chain itself, and a skill is a claim about the
	// person rather than about any one job.
	for _, s := range x.Skills {
		if strings.TrimSpace(s.Tag) == "" || strings.TrimSpace(s.Category) == "" {
			return nil, fmt.Errorf("skill %q/%q needs both a category and a name", s.Category, s.Tag)
		}
		if err := add(KindSkill, uuid.New(), skillPayload{
			Category: s.Category, Tag: s.Tag,
			Proficiency: s.Proficiency, YearsExperience: s.YearsExperience,
		}); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// StageDrafts writes planned drafts to entity_drafts and computes review flags
// for each, in one transaction.
//
// The flags are computed here, at draft time, rather than at approval: a
// collision found at approval is found after the reviewer has already decided.
func (s *Service) StageDrafts(
	ctx context.Context, userID, batchID uuid.UUID, planned []PlannedDraft,
) ([]db.EntityDraft, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("stage drafts: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit
	qtx := s.q.WithTx(tx)

	out := make([]db.EntityDraft, 0, len(planned))
	for _, p := range planned {
		payload := json.RawMessage(p.Payload)
		row, err := qtx.CreateEntityDraft(ctx, db.CreateEntityDraftParams{
			ID: p.ID, UserID: userID, BatchID: batchID, Kind: p.Kind,
			Payload: &payload, DependsOn: p.DependsOn, Status: "pending",
		})
		if err != nil {
			return nil, fmt.Errorf("stage drafts: create %s draft: %w", p.Kind, err)
		}
		out = append(out, row)
	}

	// Flagging runs after every draft is written, so a preference checked
	// against the account also sees the ones drafted alongside it. A batch
	// proposing the same label twice is exactly the case a per-draft pass
	// would miss.
	for i, row := range out {
		if _, err := FlagDraft(ctx, qtx, userID, row); err != nil {
			return nil, fmt.Errorf("stage drafts: flag %s: %w", row.ID, err)
		}
		refreshed, err := qtx.GetEntityDraft(ctx, db.GetEntityDraftParams{ID: row.ID, UserID: userID})
		if err != nil {
			return nil, fmt.Errorf("stage drafts: reload %s: %w", row.ID, err)
		}
		out[i] = refreshed
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("stage drafts: commit: %w", err)
	}
	return out, nil
}
