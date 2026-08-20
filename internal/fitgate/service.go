package fitgate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
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
// and preferences, scores technical and preference fit, and persists the
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

	screeningJSON, err := marshalScreeningSummary(signals.ScreeningSummary)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal screening summary: %w", err)
	}

	var (
		wg                               sync.WaitGroup
		technical                        TechnicalFit
		prefMatches, prefGaps, prefConfl []db.Preference
		gateHits                         []db.Preference
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		technical = ScoreTechnicalFit(skills, signals)
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
	gatePassed := len(gateHits) == 0
	gateHitsJSON, err := marshalRawNonEmpty(gateHits)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal anti-pattern hits: %w", err)
	}

	narrative, err := s.generateNarrative(
		ctx, app, signals, gatePassed, technical, prefMatches, prefGaps, prefConfl, gateHits)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: generate narrative: %w", err)
	}

	technicalGapsJSON, err := marshalRawNonEmpty(technical.Gaps)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal technical gaps: %w", err)
	}
	technicalMatchesJSON, err := marshalRawNonEmpty(technical.Matches)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal technical matches: %w", err)
	}
	technicalPartialJSON, err := marshalRawNonEmpty(technical.Partial)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal technical partial matches: %w", err)
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
		AntiPatternPassed:   gatePassed,
		AntiPatternHits:     gateHitsJSON,
		TechnicalScore:      technicalScoreColumn(technical),
		TechnicalGaps:       technicalGapsJSON,
		TechnicalMatches:    technicalMatchesJSON,
		TechnicalPartial:    technicalPartialJSON,
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
	AntiPatternPassed bool `json:"anti_pattern_passed"`
	// Absent when the JD stated no technical requirements. The narrative
	// prompt reads an absent score as "nothing was assessed" — emitting 0
	// instead would read as "covers nothing", the opposite finding.
	TechnicalScore   *float64     `json:"technical_score,omitempty"`
	TechnicalGaps    []string     `json:"technical_gaps"`
	TechnicalMatches []SkillMatch `json:"technical_matches"`
	// Requirements answered below the depth the JD asked for. Usually empty —
	// only a posting that states a depth can produce one.
	TechnicalPartial []SkillMatch `json:"technical_partial"`
	// The four preference lists, each a projection rather than the db rows —
	// see narrativePreference.
	PreferenceMatches   []narrativePreference       `json:"preference_matches"`
	PreferenceGaps      []narrativePreference       `json:"preference_gaps"`
	PreferenceConflicts []narrativePreference       `json:"preference_conflicts"`
	AntiPatternHits     []narrativePreference       `json:"anti_pattern_hits"`
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
	gatePassed bool,
	technical TechnicalFit,
	prefMatches []db.Preference,
	prefGaps []db.Preference,
	prefConflicts []db.Preference,
	gateHits []db.Preference,
) (string, error) {
	prompt, err := generation.RawPrompt("fit_narrative.txt")
	if err != nil {
		return "", err
	}

	input := narrativeInput{
		AntiPatternPassed:   gatePassed,
		TechnicalScore:      narrativeScore(technical),
		TechnicalGaps:       technical.Gaps,
		TechnicalMatches:    technical.Matches,
		TechnicalPartial:    technical.Partial,
		PreferenceMatches:   projectPreferences(prefMatches),
		PreferenceGaps:      projectPreferences(prefGaps),
		PreferenceConflicts: projectPreferences(prefConflicts),
		AntiPatternHits:     projectPreferences(gateHits),
		JDSummary:           fmt.Sprintf("%s at %s (%s)", app.RoleTitle, app.CompanyName, signals.Domain),
		ScreeningSummary:    signals.ScreeningSummary,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal narrative input: %w", err)
	}

	return s.client.Complete(ctx, prompt, string(inputJSON), narrativeMaxTokens)
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

// technicalScoreColumn stores NULL when nothing was scored. fit_reports
// .technical_score is nullable and the UI already renders a null as "—", so
// the honest value is representable end to end; the previous 100 was not
// merely wrong but confidently wrong.
func technicalScoreColumn(t TechnicalFit) pgtype.Numeric {
	if !t.Scored {
		return pgtype.Numeric{} // Valid: false — SQL NULL
	}
	return numericFromScore(t.Score)
}

// narrativeScore omits the score from the narrative input when nothing was
// scored, rather than passing a number the prompt would feel obliged to
// characterize.
func narrativeScore(t TechnicalFit) *float64 {
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
