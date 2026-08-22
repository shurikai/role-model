package fitgate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

// ErrSignalsRequired is returned when jd_signals have not been extracted for
// the application yet. Stage 1 must run before a fit evaluation can.
var ErrSignalsRequired = errors.New("jd_signals must be extracted before fit evaluation")

const narrativeMaxTokens = 512

// Service orchestrates the fit gate lifecycle: deterministic scoring plus an
// LLM-written narrative, persisted as a fit_reports row.
type Service struct {
	q      *db.Queries
	client *generation.Client
}

func NewService(q *db.Queries, client *generation.Client) *Service {
	return &Service{q: q, client: client}
}

// RunFitEvaluation loads an application's JD signals and the user's skills
// and preferences, scores capability and preference fit, and persists the
// result as a new fit_reports row.
//
// Every evaluation produces a complete report. Hard-gate preferences are
// recorded as their own list, but they do not block: a JD that trips one is
// still evaluated and still gets a narrative.
func (s *Service) RunFitEvaluation(ctx context.Context, userID, applicationID uuid.UUID) (*db.FitReport, error) {
	app, err := s.q.GetApplication(ctx, db.GetApplicationParams{ID: applicationID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: get application: %w", err)
	}
	if app.JdSignals == nil {
		return nil, ErrSignalsRequired
	}

	var signals JDSignals
	if err := json.Unmarshal(*app.JdSignals, &signals); err != nil {
		return nil, fmt.Errorf("fit evaluation: parse jd_signals: %w", err)
	}

	skillRows, err := s.q.ListActiveSkillMatchTermsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: list skills: %w", err)
	}
	skills := make([]SkillTerm, 0, len(skillRows))
	for _, r := range skillRows {
		skills = append(skills, SkillTerm{
			Name:            r.Name,
			Aliases:         r.Aliases,
			Category:        r.Category,
			CategoryAliases: r.CategoryAliases,
			Proficiency:     r.Proficiency,
		})
	}

	prefs, err := s.q.ListPreferencesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: list preferences: %w", err)
	}

	// The depth scale is the user's own vocabulary, not a fixed
	// novice/proficient/expert. An account with no rows falls back to the
	// shipped neutral scale inside LevelScale.Rank.
	levelRows, err := s.q.ListProficiencyLevelsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: list proficiency levels: %w", err)
	}
	levels := NewLevelScale(levelRows)

	screeningJSON, err := marshalScreeningSummary(signals.ScreeningSummary)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal screening summary: %w", err)
	}

	var (
		wg                               sync.WaitGroup
		capability                       CapabilityFit
		prefMatches, prefGaps, prefConfl []db.Preference
		gateHits                         []db.Preference
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		capability = ScoreCapabilityFit(skills, signals, levels)
	}()
	go func() {
		defer wg.Done()
		prefMatches, prefGaps, prefConfl, gateHits = ScorePreferenceFit(prefs, signals)
	}()
	wg.Wait()

	// The gate no longer runs as a separate pass. It used to, against its own
	// field-routing function, which is how preference scoring came to be blind
	// to the skills arrays the gate could see. Scoring now reports every
	// hard-gate row it matched and the boolean is derived from that, so the
	// two can no longer disagree about what a JD says.
	//
	// It remains non-blocking: a JD that trips a gate is still evaluated, still
	// gets a narrative, and still generates. The trip is reported by name in
	// gateHits rather than priced into a number, so this boolean is a summary
	// of that list and never the only record of it.
	dealbreakersClear := len(gateHits) == 0
	gateHitsJSON, err := marshalRawNonEmpty(gateHits)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal anti-pattern hits: %w", err)
	}

	narrative, err := s.generateNarrative(
		ctx, app, signals, dealbreakersClear, capability, prefMatches, prefGaps, prefConfl, gateHits)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: generate narrative: %w", err)
	}

	capabilityGapsJSON, err := marshalRawNonEmpty(capability.Gaps)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal capability gaps: %w", err)
	}
	capabilityMatchesJSON, err := marshalRawNonEmpty(capability.Matches)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal capability matches: %w", err)
	}
	capabilityPartialJSON, err := marshalRawNonEmpty(capability.Partial)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal capability partial matches: %w", err)
	}
	prefMatchesJSON, err := marshalRawNonEmpty(prefMatches)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal preference matches: %w", err)
	}
	prefGapsJSON, err := marshalRawNonEmpty(prefGaps)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal preference gaps: %w", err)
	}
	prefConflictsJSON, err := marshalRawNonEmpty(prefConfl)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal preference conflicts: %w", err)
	}

	report, err := s.q.CreateFitReport(ctx, db.CreateFitReportParams{
		ID:                  uuid.New(),
		UserID:              userID,
		ApplicationID:       &applicationID,
		DealbreakersClear:   dealbreakersClear,
		DealbreakerHits:     gateHitsJSON,
		CapabilityScore:     capabilityScoreColumn(capability),
		CapabilityGaps:      capabilityGapsJSON,
		CapabilityMatches:   capabilityMatchesJSON,
		CapabilityPartial:   capabilityPartialJSON,
		PreferenceMatches:   prefMatchesJSON,
		PreferenceGaps:      prefGapsJSON,
		PreferenceConflicts: prefConflictsJSON,
		ScreeningSummary:    screeningJSON,
		Narrative:           &narrative,
	})
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: store report: %w", err)
	}
	return &report, nil
}

type narrativeInput struct {
	DealbreakersClear bool `json:"dealbreakers_clear"`
	// Absent when the JD stated no capability requirements. The narrative
	// prompt reads an absent score as "nothing was assessed" — emitting 0
	// instead would read as "covers nothing", the opposite finding.
	CapabilityScore   *float64     `json:"capability_score,omitempty"`
	CapabilityGaps    []string     `json:"capability_gaps"`
	CapabilityMatches []SkillMatch `json:"capability_matches"`
	// Requirements answered below the depth the JD asked for. Usually empty —
	// only a posting that states a depth can produce one.
	CapabilityPartial []SkillMatch `json:"capability_partial"`
	// The capability-level requirements the JD stated in prose. Passed for
	// context only: ScoreCapabilityFit does not read them, so nothing here is
	// scored, matched, or reported as a gap.
	//
	// It is here because the unscored case is otherwise indistinguishable
	// from a vague posting. A staff JD naming no technology scores nothing
	// and produces no matches and no gaps, and the narrative — seeing an
	// empty report — wrote that the posting "does not state requirements
	// specific enough to score against" about a JD that had stated ten of
	// them. The prompt can now say what was asked for and that it went
	// unassessed, which are different claims.
	CoreCompetencies []string `json:"core_competencies"`
	// The four preference lists, each a projection rather than the db rows —
	// see narrativePreference.
	PreferenceMatches   []narrativePreference       `json:"preference_matches"`
	PreferenceGaps      []narrativePreference       `json:"preference_gaps"`
	PreferenceConflicts []narrativePreference       `json:"preference_conflicts"`
	DealbreakerHits     []narrativePreference       `json:"dealbreaker_hits"`
	JDSummary           string                      `json:"jd_summary"`
	ScreeningSummary    generation.ScreeningSummary `json:"screening_summary"`
}

// narrativePreference is what a preference row looks like to the narrative
// prompt: what it says, and which kind of thing it is about.
//
// It is a projection and not db.Preference on purpose, for two reasons. The
// row carries ids, timestamps, and a user_id that cost tokens and tell the
// model nothing. More importantly it carries weight, and the whole point of
// replacing preference_score was that the narrative interprets findings rather
// than doing arithmetic over them — handing it the weights would invite it
// straight back into ranking rows by number. Which list an entry arrived in is
// the severity signal, and there are four lists precisely so it can be.
//
// The fit_reports columns still store the complete rows; only the prompt sees
// this narrower shape.
type narrativePreference struct {
	Label string `json:"label"`
	Type  string `json:"preference_type"`
}

// projectPreferences maps db rows to the prompt's view of them, and returns an
// empty slice rather than nil so every list marshals as [] instead of null. A
// null list reads to the model as "unknown"; an empty one reads as "none",
// which is the true statement.
func projectPreferences(prefs []db.Preference) []narrativePreference {
	out := make([]narrativePreference, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, narrativePreference{Label: p.Label, Type: p.PreferenceType})
	}
	return out
}

func (s *Service) generateNarrative(
	ctx context.Context,
	app db.Application,
	signals JDSignals,
	dealbreakersClear bool,
	capability CapabilityFit,
	prefMatches []db.Preference,
	prefGaps []db.Preference,
	prefConflicts []db.Preference,
	gateHits []db.Preference,
) (string, error) {
	prompt, err := generation.RawPrompt("fit_narrative.txt")
	if err != nil {
		return "", err
	}

	input := buildNarrativeInput(
		app, signals, dealbreakersClear, capability, prefMatches, prefGaps, prefConflicts, gateHits)
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal narrative input: %w", err)
	}

	return s.client.Complete(ctx, prompt, string(inputJSON), narrativeMaxTokens)
}

// buildNarrativeInput assembles what the narrative prompt sees. Split out from
// generateNarrative so the assembly is testable without an LLM call — the
// wiring is where a signal gets silently dropped, which is how the unscored
// case came to be narrated as a vague posting.
func buildNarrativeInput(
	app db.Application,
	signals JDSignals,
	dealbreakersClear bool,
	capability CapabilityFit,
	prefMatches []db.Preference,
	prefGaps []db.Preference,
	prefConflicts []db.Preference,
	gateHits []db.Preference,
) narrativeInput {
	return narrativeInput{
		DealbreakersClear:   dealbreakersClear,
		CapabilityScore:     narrativeScore(capability),
		CapabilityGaps:      capability.Gaps,
		CapabilityMatches:   capability.Matches,
		CapabilityPartial:   capability.Partial,
		CoreCompetencies:    signals.CoreCompetencies,
		PreferenceMatches:   projectPreferences(prefMatches),
		PreferenceGaps:      projectPreferences(prefGaps),
		PreferenceConflicts: projectPreferences(prefConflicts),
		DealbreakerHits:     projectPreferences(gateHits),
		JDSummary:           narrativeJDSummary(app, signals),
		ScreeningSummary:    signals.ScreeningSummary,
	}
}

// narrativeJDSummary is the one-line role identifier at the top of the
// narrative input. The parenthetical used to be signals.Domain, which was an
// enum that regularly said "saas" or "platform" about a posting whose actual
// industry was freight logistics or municipal IT. It reads the free-text
// industry now, and drops the parenthetical entirely when the posting did not
// state one — "Staff Engineer at Acme ()" reads as a bug rather than as an
// absent fact.
func narrativeJDSummary(app db.Application, signals JDSignals) string {
	industry := strings.TrimSpace(signals.ScreeningSummary.Industry)
	if industry == "" {
		return fmt.Sprintf("%s at %s", app.RoleTitle, app.CompanyName)
	}
	return fmt.Sprintf("%s at %s (%s)", app.RoleTitle, app.CompanyName, industry)
}

// marshalScreeningSummary marshals the summary to a *json.RawMessage,
// returning nil (NULL in the DB) for the zero value. Signals extracted before
// the screening_summary prompt field existed unmarshal to an empty struct;
// storing that as an object of empty strings would be indistinguishable from
// a JD that genuinely stated nothing.
func marshalScreeningSummary(s generation.ScreeningSummary) (*json.RawMessage, error) {
	if reflect.ValueOf(s).IsZero() {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(b)
	return &raw, nil
}

// marshalRawNonEmpty marshals v to a *json.RawMessage, returning nil (NULL
// in the DB) when v has no elements rather than an empty JSON array.
func marshalRawNonEmpty[T any](v []T) (*json.RawMessage, error) {
	if len(v) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(b)
	return &raw, nil
}

// capabilityScoreColumn stores NULL when nothing was scored. fit_reports
// .capability_score is nullable and the UI already renders a null as "—", so
// the honest value is representable end to end; the previous 100 was not
// merely wrong but confidently wrong.
func capabilityScoreColumn(t CapabilityFit) pgtype.Numeric {
	if !t.Scored {
		return pgtype.Numeric{} // Valid: false — SQL NULL
	}
	return numericFromScore(t.Score)
}

// narrativeScore omits the score from the narrative input when nothing was
// scored, rather than passing a number the prompt would feel obliged to
// characterize.
func narrativeScore(t CapabilityFit) *float64 {
	if !t.Scored {
		return nil
	}
	score := t.Score
	return &score
}

func numericFromScore(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', 2, 64))
	return n
}
