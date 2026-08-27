package intake

import (
	"bytes"
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
	KindEducation    = "education"
	KindCredential   = "credential"
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
	case KindEducation:
		return resolveEducation(ctx, q, userID, d)
	case KindCredential:
		return resolveCredential(ctx, q, userID, d)
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
	if err := p.validate(); err != nil {
		return uuid.Nil, err
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
	if err := p.validate(); err != nil {
		return uuid.Nil, err
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
	if err := p.validate(); err != nil {
		return uuid.Nil, err
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
	if err := p.validate(); err != nil {
		return uuid.Nil, err
	}

	row, err := q.CreatePreference(ctx, db.CreatePreferenceParams{
		ID: uuid.New(), UserID: userID,
		PreferenceType: p.PreferenceType, Label: p.Label,
		Sentiment: p.Sentiment, Weight: p.Weight,
		IsHardGate: p.IsHardGate, Notes: p.Notes,
		Aliases: p.Aliases,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create preference %q: %w", p.Label, err)
	}
	return row.ID, nil
}

// Education and credentials are the simplest kind of draft: no dependency on
// anything else in the batch, and no vocabulary to resolve. They exist because
// extraction was dropping them (#89) — a licensed professional's CREDENTIALS
// section rendered empty because the certifications she listed were filed as
// skills, which is the failure the renderer fix closed downstream and this
// reintroduced upstream.

type educationPayload struct {
	Institution  string  `json:"institution"`
	Degree       *string `json:"degree"`
	FieldOfStudy *string `json:"field_of_study"`
	StartedOn    *string `json:"started_on"`
	EndedOn      *string `json:"ended_on"`
	Notes        *string `json:"notes"`
}

func resolveEducation(ctx context.Context, q *db.Queries, userID uuid.UUID, d db.EntityDraft) (uuid.UUID, error) {
	var p educationPayload
	if err := payloadOf(d, &p); err != nil {
		return uuid.Nil, err
	}
	if err := p.validate(); err != nil {
		return uuid.Nil, err
	}
	started, err := optionalDate(p.StartedOn)
	if err != nil {
		return uuid.Nil, fmt.Errorf("started_on: %w", err)
	}
	ended, err := optionalDate(p.EndedOn)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ended_on: %w", err)
	}

	row, err := q.CreateEducation(ctx, db.CreateEducationParams{
		ID: uuid.New(), UserID: userID,
		Institution: p.Institution, Degree: p.Degree, FieldOfStudy: p.FieldOfStudy,
		StartedOn: started, EndedOn: ended, Notes: p.Notes,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create education %q: %w", p.Institution, err)
	}
	return row.ID, nil
}

type credentialPayload struct {
	Name          string  `json:"name"`
	Issuer        *string `json:"issuer"`
	IssuedOn      *string `json:"issued_on"`
	ExpiresOn     *string `json:"expires_on"`
	CredentialURL *string `json:"credential_url"`
}

func resolveCredential(ctx context.Context, q *db.Queries, userID uuid.UUID, d db.EntityDraft) (uuid.UUID, error) {
	var p credentialPayload
	if err := payloadOf(d, &p); err != nil {
		return uuid.Nil, err
	}
	if err := p.validate(); err != nil {
		return uuid.Nil, err
	}
	issued, err := optionalDate(p.IssuedOn)
	if err != nil {
		return uuid.Nil, fmt.Errorf("issued_on: %w", err)
	}
	expires, err := optionalDate(p.ExpiresOn)
	if err != nil {
		return uuid.Nil, fmt.Errorf("expires_on: %w", err)
	}

	row, err := q.CreateCredential(ctx, db.CreateCredentialParams{
		ID: uuid.New(), UserID: userID,
		Name: p.Name, Issuer: p.Issuer,
		IssuedOn: issued, ExpiresOn: expires, CredentialUrl: p.CredentialURL,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create credential %q: %w", p.Name, err)
	}
	return row.ID, nil
}

// optionalDate parses a date that may legitimately be absent. An absent date is
// a NULL column, not an error: a credential with no stated issue date is
// ordinary, and refusing it would push the reviewer into inventing one.
func optionalDate(s *string) (pgtype.Date, error) {
	var out pgtype.Date
	if s == nil || strings.TrimSpace(*s) == "" {
		return out, nil
	}
	return parseDate(*s)
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

// ErrDependencyNotResolved is returned when a single draft is approved before
// something it depends on. Distinct from ErrUnresolved, which is a batch-level
// report: this one names a specific missing parent and is recoverable by
// approving that parent first.
var ErrDependencyNotResolved = errors.New("draft depends on a draft that has not been resolved")

// ErrDraftNotPending is returned when a draft is approved, rejected or edited
// from a status other than pending. Mirrors stage0.ErrDraftNotPending, which
// the narrow contribution path uses for the same situation.
var ErrDraftNotPending = errors.New("draft is not pending")

// ApproveDraft resolves exactly one draft, in its own transaction.
//
// The resolver was always designed for this. ResolveBatch's own contract is
// that "drafts resolved by an earlier pass are already parents for this one",
// so a single-draft approve is another pass with one draft in it rather than a
// second code path — it reuses resolveOne and marks the same way.
//
// # A missing parent is refused, never created
//
// If the draft depends on something still pending, this returns
// ErrDependencyNotResolved naming that draft, and writes nothing. Resolving the
// parent on the reviewer's behalf would be the one thing this review queue
// exists to prevent: approving a row the person never looked at, discovered
// later as an employer they would have rejected. It is the same rule the
// reject side follows from the other direction — rejecting a draft with
// dependents warns about them rather than cascading.
func (s *Service) ApproveDraft(ctx context.Context, userID, draftID uuid.UUID) (uuid.UUID, error) {
	draft, err := s.q.GetEntityDraft(ctx, db.GetEntityDraftParams{ID: draftID, UserID: userID})
	if err != nil {
		return uuid.Nil, fmt.Errorf("approve draft: get draft: %w", err)
	}
	if draft.Status != "pending" {
		return uuid.Nil, ErrDraftNotPending
	}

	// The batch's other drafts are what a dependency id resolves through —
	// the same map ResolveBatch seeds before ordering anything.
	siblings, err := s.q.ListEntityDraftsByBatch(ctx, db.ListEntityDraftsByBatchParams{
		BatchID: draft.BatchID,
		UserID:  userID,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("approve draft: list batch drafts: %w", err)
	}
	resolved := map[uuid.UUID]uuid.UUID{}
	for _, d := range siblings {
		if d.Status == "approved" && d.ResolvedID != nil {
			resolved[d.ID] = *d.ResolvedID
		}
	}

	for _, dep := range draft.DependsOn {
		if _, ok := resolved[dep]; ok {
			continue
		}
		return uuid.Nil, fmt.Errorf("%w: %s", ErrDependencyNotResolved, describeDependency(dep, siblings))
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("approve draft: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit
	qtx := s.q.WithTx(tx)

	rowID, err := s.resolveOne(ctx, qtx, userID, draft, resolved)
	if err != nil {
		return uuid.Nil, fmt.Errorf("approve draft %s (%s): %w", draft.ID, draft.Kind, err)
	}
	if _, err := qtx.MarkEntityDraftResolved(ctx, db.MarkEntityDraftResolvedParams{
		ID: draft.ID, UserID: userID, ResolvedID: &rowID,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("approve draft: mark resolved: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("approve draft: commit: %w", err)
	}
	return rowID, nil
}

// describeDependency names an unresolved parent in words a reviewer can act on.
// A bare uuid tells them nothing about which card to approve first, and the
// batch listing is already loaded, so the kind and status cost nothing.
func describeDependency(dep uuid.UUID, siblings []db.EntityDraft) string {
	for _, d := range siblings {
		if d.ID == dep {
			return fmt.Sprintf("%s draft %s is %s", d.Kind, d.ID, d.Status)
		}
	}
	return fmt.Sprintf("draft %s is not in this batch", dep)
}

// ValidatePayload reports whether raw is a usable payload for kind.
//
// Called when a reviewer edits a draft, so a payload that cannot become a row
// fails at edit time — on the form, next to the field — rather than at resolve
// time, where it surfaces as one draft mysteriously refusing to become a row
// after the reviewer has moved on.
//
// Unknown fields are an error rather than being dropped. A dropped field means
// an edit the person typed and watched save is silently not there, which is
// worse than a rejected save telling them the name they used.
//
// Each kind's checks are the SAME function the resolver runs — not a second
// copy of the rules. A validator that drifts from the resolver is worse than no
// validator: it passes what resolution then refuses.
func ValidatePayload(kind string, raw json.RawMessage) error {
	decode := func(v any) error {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			return fmt.Errorf("payload does not fit a %s draft: %w", kind, err)
		}
		return nil
	}

	switch kind {
	case KindEmployer:
		var p employerPayload
		if err := decode(&p); err != nil {
			return err
		}
		return p.validate()
	case KindPosition:
		var p positionPayload
		if err := decode(&p); err != nil {
			return err
		}
		return p.validate()
	case KindContribution:
		var p contributionPayload
		if err := decode(&p); err != nil {
			return err
		}
		return p.validate()
	case KindSkill:
		var p skillPayload
		if err := decode(&p); err != nil {
			return err
		}
		return p.validate()
	case KindPreference:
		var p preferencePayload
		if err := decode(&p); err != nil {
			return err
		}
		return p.validate()
	case KindEducation:
		var p educationPayload
		if err := decode(&p); err != nil {
			return err
		}
		return p.validate()
	case KindCredential:
		var p credentialPayload
		if err := decode(&p); err != nil {
			return err
		}
		return p.validate()
	default:
		return fmt.Errorf("no resolver for kind %q", kind)
	}
}

// The per-kind rules, in one place each so the edit-time validator and the
// resolver cannot disagree about what a usable payload is.
//
// Each one encodes exactly what its resolver already enforced, and nothing
// more — a position's title is not checked here because resolvePosition never
// checked it, and tightening resolution is a separate decision from making
// editing safe.

func (p employerPayload) validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("employer name is required")
	}
	return nil
}

func (p positionPayload) validate() error {
	if _, err := parseDate(p.StartedOn); err != nil {
		return fmt.Errorf("started_on: %w", err)
	}
	if p.EndedOn != nil && *p.EndedOn != "" {
		if _, err := parseDate(*p.EndedOn); err != nil {
			return fmt.Errorf("ended_on: %w", err)
		}
	}
	return nil
}

func (p contributionPayload) validate() error {
	if strings.TrimSpace(p.Summary) == "" || strings.TrimSpace(p.FullDescription) == "" {
		return errors.New("summary and full_description are both required")
	}
	return nil
}

func (p skillPayload) validate() error {
	if strings.TrimSpace(p.Proficiency) == "" {
		return errors.New("proficiency is required")
	}
	return nil
}

func (p preferencePayload) validate() error {
	if strings.TrimSpace(p.Label) == "" {
		return errors.New("label is required")
	}
	return nil
}

func (p educationPayload) validate() error {
	if strings.TrimSpace(p.Institution) == "" {
		return errors.New("institution is required")
	}
	if _, err := optionalDate(p.StartedOn); err != nil {
		return fmt.Errorf("started_on: %w", err)
	}
	if _, err := optionalDate(p.EndedOn); err != nil {
		return fmt.Errorf("ended_on: %w", err)
	}
	return nil
}

func (p credentialPayload) validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("name is required")
	}
	if _, err := optionalDate(p.IssuedOn); err != nil {
		return fmt.Errorf("issued_on: %w", err)
	}
	if _, err := optionalDate(p.ExpiresOn); err != nil {
		return fmt.Errorf("expires_on: %w", err)
	}
	return nil
}
