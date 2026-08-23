package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
)

// Draft kinds the resolver knows how to turn into rows. Deliberately not a
// database CHECK: an extractor proposing a kind nothing can resolve should show
// up as one unresolvable draft in a review queue, not as a migration that has
// to ship before the extractor can be changed.
const (
	KindEmployer     = "employer"
	KindPosition     = "position"
	KindContribution = "contribution"
	KindSkill        = "skill"
	KindPreference   = "preference"
)

// ErrUnresolved is returned when a batch still holds pending drafts that could
// not be resolved — a dependency that was rejected, a cycle, or an unknown kind.
var ErrUnresolved = errors.New("batch has drafts that could not be resolved")

// Service turns approved drafts into rows.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewService(pool *pgxpool.Pool, q *db.Queries) *Service {
	return &Service{pool: pool, q: q}
}

// Result reports what one resolution pass did.
type Result struct {
	Resolved   map[uuid.UUID]uuid.UUID // draft id -> created row id
	Unresolved map[uuid.UUID]string    // draft id -> why
}

// ResolveBatch writes every pending draft in the batch, in dependency order,
// inside one transaction.
//
// # Why an order has to be computed rather than assumed
//
// A position names its employer, a contribution names its position, a skill
// names its tag and the tag names its category — and none of those parents has
// an id until the moment it is approved. contribution_drafts stored
// employer_name and position_title as plain text precisely because of this, and
// then required the caller to have already created both by hand.
//
// The order differs per batch: one import is a whole career from scratch,
// another is three contributions against employers that have existed for years.
// A fixed sequence gets the second case wrong by re-creating what is already
// there, and a wrong guess writes an orphan that nothing reports.
//
// # All or nothing
//
// One transaction, because a half-resolved batch is the worst outcome: the
// employer exists, the positions do not, and the drafts that named them are
// marked approved with no row behind them. A failure leaves the batch exactly
// as it was, for a human to look at.
func (s *Service) ResolveBatch(ctx context.Context, userID, batchID uuid.UUID) (Result, error) {
	drafts, err := s.q.ListEntityDraftsByBatch(ctx, db.ListEntityDraftsByBatchParams{
		BatchID: batchID,
		UserID:  userID,
	})
	if err != nil {
		return Result{}, fmt.Errorf("resolve batch: list drafts: %w", err)
	}

	result := Result{
		Resolved:   map[uuid.UUID]uuid.UUID{},
		Unresolved: map[uuid.UUID]string{},
	}

	// Drafts resolved by an earlier pass are already parents for this one.
	byID := make(map[uuid.UUID]db.EntityDraft, len(drafts))
	for _, d := range drafts {
		byID[d.ID] = d
		if d.Status == "approved" && d.ResolvedID != nil {
			result.Resolved[d.ID] = *d.ResolvedID
		}
	}

	order, unresolvable := topoOrder(drafts, result.Resolved)
	for id, why := range unresolvable {
		result.Unresolved[id] = why
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("resolve batch: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit
	qtx := s.q.WithTx(tx)

	for _, d := range order {
		rowID, err := s.resolveOne(ctx, qtx, userID, d, result.Resolved)
		if err != nil {
			return Result{}, fmt.Errorf("resolve draft %s (%s): %w", d.ID, d.Kind, err)
		}
		if _, err := qtx.MarkEntityDraftResolved(ctx, db.MarkEntityDraftResolvedParams{
			ID:         d.ID,
			UserID:     userID,
			ResolvedID: &rowID,
		}); err != nil {
			return Result{}, fmt.Errorf("resolve batch: mark %s resolved: %w", d.ID, err)
		}
		result.Resolved[d.ID] = rowID
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("resolve batch: commit: %w", err)
	}

	if len(result.Unresolved) > 0 {
		return result, ErrUnresolved
	}
	return result, nil
}

// topoOrder returns the pending drafts in an order where every dependency comes
// first, plus the ones that can never be reached and why.
//
// Kahn's algorithm over depends_on, with already-resolved drafts counting as
// satisfied. What is left over after no more progress is possible is either a
// cycle or a draft depending on something rejected — and both are reported
// rather than silently dropped, because a draft that vanishes from a review
// queue without becoming a row is the failure this whole substrate exists to
// avoid.
func topoOrder(drafts []db.EntityDraft, alreadyResolved map[uuid.UUID]uuid.UUID) ([]db.EntityDraft, map[uuid.UUID]string) {
	pending := map[uuid.UUID]db.EntityDraft{}
	for _, d := range drafts {
		if d.Status == "pending" {
			pending[d.ID] = d
		}
	}

	satisfied := map[uuid.UUID]bool{}
	for id := range alreadyResolved {
		satisfied[id] = true
	}

	var order []db.EntityDraft
	for {
		progressed := false
		// Iterate the input slice rather than the map so the order is stable
		// across runs: a batch that resolves must resolve the same way twice.
		for _, d := range drafts {
			if _, ok := pending[d.ID]; !ok {
				continue
			}
			ready := true
			for _, dep := range d.DependsOn {
				if !satisfied[dep] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			order = append(order, d)
			satisfied[d.ID] = true
			delete(pending, d.ID)
			progressed = true
		}
		if !progressed {
			break
		}
	}

	// Classify what is left. A draft whose unmet dependency is ITSELF stuck is
	// in a cycle; one whose dependency was rejected or never existed is an
	// orphan. Both are reported, and they are reported differently because the
	// fixes differ: a cycle is an extractor bug, an orphan is a review
	// decision with a consequence the reviewer did not see.
	stuck := map[uuid.UUID]string{}
	for id, d := range pending {
		var inCycle, missing []string
		for _, dep := range d.DependsOn {
			if satisfied[dep] {
				continue
			}
			if _, alsoStuck := pending[dep]; alsoStuck {
				inCycle = append(inCycle, dep.String())
			} else {
				missing = append(missing, dep.String())
			}
		}
		switch {
		case len(missing) > 0:
			stuck[id] = "depends on drafts that were not resolved: " + strings.Join(missing, ", ")
		case len(inCycle) > 0:
			stuck[id] = "part of a dependency cycle with: " + strings.Join(inCycle, ", ")
		default:
			stuck[id] = "unreachable with no unmet dependency, which should be impossible"
		}
	}
	return order, stuck
}

func (s *Service) resolveOne(
	ctx context.Context, q *db.Queries, userID uuid.UUID,
	d db.EntityDraft, resolved map[uuid.UUID]uuid.UUID,
) (uuid.UUID, error) {
	switch d.Kind {
	case KindEmployer:
		return resolveEmployer(ctx, q, userID, d)
	case KindPosition:
		return resolvePosition(ctx, q, userID, d, resolved)
	case KindContribution:
		return resolveContribution(ctx, q, userID, d, resolved)
	case KindSkill:
		return resolveSkill(ctx, q, userID, d)
	case KindPreference:
		return resolvePreference(ctx, q, userID, d)
	default:
		return uuid.Nil, fmt.Errorf("no resolver for kind %q", d.Kind)
	}
}

// payloadOf decodes a draft's payload into v.
func payloadOf(d db.EntityDraft, v any) error {
	if d.Payload == nil {
		return fmt.Errorf("draft %s has no payload", d.ID)
	}
	if err := json.Unmarshal(*d.Payload, v); err != nil {
		return fmt.Errorf("draft %s: parse payload: %w", d.ID, err)
	}
	return nil
}

// parentOf returns the row id a named dependency resolved to.
func parentOf(field string, id *uuid.UUID, resolved map[uuid.UUID]uuid.UUID) (uuid.UUID, error) {
	if id == nil {
		return uuid.Nil, fmt.Errorf("%s is required", field)
	}
	rowID, ok := resolved[*id]
	if !ok {
		return uuid.Nil, fmt.Errorf("%s names draft %s, which has not resolved", field, *id)
	}
	return rowID, nil
}

type employerPayload struct {
	Name     string  `json:"name"`
	Industry *string `json:"industry"`
	Notes    *string `json:"notes"`
}

func resolveEmployer(ctx context.Context, q *db.Queries, userID uuid.UUID, d db.EntityDraft) (uuid.UUID, error) {
	var p employerPayload
	if err := payloadOf(d, &p); err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(p.Name) == "" {
		return uuid.Nil, errors.New("employer name is required")
	}

	// An employer the account already has is reused rather than duplicated.
	// The alternative splits one career across two rows that look identical in
	// the UI and share nothing, which is unrecoverable without a merge tool.
	existing, err := q.GetEmployers(ctx, userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("list employers: %w", err)
	}
	for _, e := range existing {
		if strings.EqualFold(e.Name, p.Name) {
			return e.ID, nil
		}
	}

	row, err := q.CreateEmployer(ctx, db.CreateEmployerParams{
		ID: uuid.New(), UserID: userID, Name: p.Name, Industry: p.Industry, Notes: p.Notes,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create employer: %w", err)
	}
	return row.ID, nil
}

type positionPayload struct {
	EmployerDraft    *uuid.UUID `json:"employer_draft"`
	Title            string     `json:"title"`
	IndustryLevel    *string    `json:"industry_level"`
	IndustryRole     *string    `json:"industry_role"`
	Location         *string    `json:"location"`
	StartedOn        string     `json:"started_on"`
	EndedOn          *string    `json:"ended_on"`
	ContextNarrative *string    `json:"context_narrative"`
}

func resolvePosition(
	ctx context.Context, q *db.Queries, userID uuid.UUID,
	d db.EntityDraft, resolved map[uuid.UUID]uuid.UUID,
) (uuid.UUID, error) {
	var p positionPayload
	if err := payloadOf(d, &p); err != nil {
		return uuid.Nil, err
	}
	employerID, err := parentOf("employer_draft", p.EmployerDraft, resolved)
	if err != nil {
		return uuid.Nil, err
	}
	started, err := parseDate(p.StartedOn)
	if err != nil {
		return uuid.Nil, fmt.Errorf("started_on: %w", err)
	}
	var ended pgtype.Date
	if p.EndedOn != nil && *p.EndedOn != "" {
		ended, err = parseDate(*p.EndedOn)
		if err != nil {
			return uuid.Nil, fmt.Errorf("ended_on: %w", err)
		}
	}

	row, err := q.CreatePosition(ctx, db.CreatePositionParams{
		ID: uuid.New(), UserID: userID, EmployerID: employerID,
		Title: p.Title, IndustryLevel: p.IndustryLevel, IndustryRole: p.IndustryRole,
		Location: p.Location, StartedOn: started, EndedOn: ended,
		ContextNarrative: p.ContextNarrative,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create position: %w", err)
	}
	return row.ID, nil
}

type contributionPayload struct {
	PositionDraft   *uuid.UUID `json:"position_draft"`
	Summary         string     `json:"summary"`
	FullDescription string     `json:"full_description"`
	Outcomes        *string    `json:"outcomes"`
	ScaleContext    *string    `json:"scale_context"`
	Tags            []tagRef   `json:"tags"`
}

type tagRef struct {
	Category string `json:"category"`
	Name     string `json:"name"`
}

func resolveContribution(
	ctx context.Context, q *db.Queries, userID uuid.UUID,
	d db.EntityDraft, resolved map[uuid.UUID]uuid.UUID,
) (uuid.UUID, error) {
	var p contributionPayload
	if err := payloadOf(d, &p); err != nil {
		return uuid.Nil, err
	}
	positionID, err := parentOf("position_draft", p.PositionDraft, resolved)
	if err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(p.Summary) == "" || strings.TrimSpace(p.FullDescription) == "" {
		return uuid.Nil, errors.New("summary and full_description are both required")
	}

	row, err := q.CreateContribution(ctx, db.CreateContributionParams{
		ID: uuid.New(), UserID: userID, PositionID: &positionID,
		Summary: p.Summary, FullDescription: p.FullDescription,
		Outcomes: p.Outcomes, ScaleContext: p.ScaleContext, IsActive: true,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create contribution: %w", err)
	}

	for _, ref := range p.Tags {
		tag, err := ResolveOrCreateTag(ctx, q, userID, ref.Category, ref.Name)
		if err != nil {
			return uuid.Nil, err
		}
		if err := q.AssignTagToContribution(ctx, db.AssignTagToContributionParams{
			ContributionID: row.ID, TagID: tag.ID,
		}); err != nil {
			return uuid.Nil, fmt.Errorf("tag contribution with %q: %w", ref.Name, err)
		}
	}
	return row.ID, nil
}

type skillPayload struct {
	Category        string   `json:"category"`
	Tag             string   `json:"tag"`
	Proficiency     string   `json:"proficiency"`
	YearsExperience *float64 `json:"years_experience"`
}

func resolveSkill(ctx context.Context, q *db.Queries, userID uuid.UUID, d db.EntityDraft) (uuid.UUID, error) {
	var p skillPayload
	if err := payloadOf(d, &p); err != nil {
		return uuid.Nil, err
	}
	tag, err := ResolveOrCreateTag(ctx, q, userID, p.Category, p.Tag)
	if err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(p.Proficiency) == "" {
		return uuid.Nil, errors.New("proficiency is required")
	}

	var years pgtype.Numeric
	if p.YearsExperience != nil {
		if err := years.Scan(fmt.Sprintf("%.1f", *p.YearsExperience)); err != nil {
			return uuid.Nil, fmt.Errorf("years_experience: %w", err)
		}
	}

	row, err := q.CreateSkill(ctx, db.CreateSkillParams{
		ID: uuid.New(), UserID: userID, TagID: tag.ID,
		Proficiency: p.Proficiency, YearsExperience: years, IsActive: true,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create skill %q: %w", p.Tag, err)
	}
	return row.ID, nil
}

type preferencePayload struct {
	PreferenceType string   `json:"preference_type"`
	Label          string   `json:"label"`
	Aliases        []string `json:"aliases"`
	Sentiment      string   `json:"sentiment"`
	Weight         int16    `json:"weight"`
	IsHardGate     bool     `json:"is_hard_gate"`
	Notes          *string  `json:"notes"`
}

func resolvePreference(ctx context.Context, q *db.Queries, userID uuid.UUID, d db.EntityDraft) (uuid.UUID, error) {
	var p preferencePayload
	if err := payloadOf(d, &p); err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(p.Label) == "" {
		return uuid.Nil, errors.New("label is required")
	}

	row, err := q.CreatePreference(ctx, db.CreatePreferenceParams{
		ID: uuid.New(), UserID: userID,
		PreferenceType: p.PreferenceType, Label: p.Label,
		Sentiment: p.Sentiment, Weight: p.Weight,
		IsHardGate: p.IsHardGate, Notes: p.Notes,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create preference %q: %w", p.Label, err)
	}
	return row.ID, nil
}

// parseDate accepts "2006-01" and "2006-01-02". Extraction writes the first,
// the CRUD API writes the second, and a draft may carry either.
func parseDate(s string) (pgtype.Date, error) {
	var out pgtype.Date
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", "2006-01"} {
		if t, err := time.Parse(layout, s); err == nil {
			out.Time, out.Valid = t, true
			return out, nil
		}
	}
	return out, fmt.Errorf("%q is not YYYY-MM or YYYY-MM-DD", s)
}
