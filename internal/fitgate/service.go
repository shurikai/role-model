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
// recorded and priced into preference_score, but they do not block: a JD that
// trips one is still scored and still gets a narrative.
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

	skillNames, err := s.q.ListActiveSkillTagNamesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: list skills: %w", err)
	}

	prefs, err := s.q.ListPreferencesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: list preferences: %w", err)
	}

	appIDParam := pgtype.UUID{Bytes: [16]byte(applicationID), Valid: true}

	screeningJSON, err := marshalScreeningSummary(signals.ScreeningSummary)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal screening summary: %w", err)
	}

	var (
		wg                                 sync.WaitGroup
		technicalScore, prefScore          float64
		technicalGaps, prefGaps, prefConfl []string
		gateHits                           []db.Preference
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		technicalScore, technicalGaps = ScoreTechnicalFit(skillNames, signals)
	}()
	go func() {
		defer wg.Done()
		prefScore, prefGaps, prefConfl, gateHits = ScorePreferenceFit(prefs, signals)
	}()
	wg.Wait()

	// The gate no longer runs as a separate pass. It used to, against its own
	// field-routing function, which is how preference scoring came to be blind
	// to the skills arrays the gate could see. Scoring now reports every
	// hard-gate row it matched and the boolean is derived from that, so the
	// two can no longer disagree about what a JD says.
	//
	// It remains non-blocking: a JD that trips a gate is still scored, still
	// gets a narrative, and still generates. What changed is that the trip is
	// now priced into preference_score instead of living only in this boolean.
	gatePassed := len(gateHits) == 0
	gateHitsJSON, err := marshalRawNonEmpty(gateHits)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal anti-pattern hits: %w", err)
	}

	narrative, err := s.generateNarrative(ctx, app, signals, gatePassed, technicalScore, technicalGaps, prefScore, prefGaps, prefConfl)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: generate narrative: %w", err)
	}

	technicalGapsJSON, err := marshalRawNonEmpty(technicalGaps)
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: marshal technical gaps: %w", err)
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
		ApplicationID:       appIDParam,
		AntiPatternPassed:   gatePassed,
		AntiPatternHits:     gateHitsJSON,
		TechnicalScore:      numericFromScore(technicalScore),
		TechnicalGaps:       technicalGapsJSON,
		PreferenceScore:     numericFromScore(prefScore),
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
	AntiPatternPassed   bool                        `json:"anti_pattern_passed"`
	TechnicalScore      float64                     `json:"technical_score"`
	TechnicalGaps       []string                    `json:"technical_gaps"`
	PreferenceScore     float64                     `json:"preference_score"`
	PreferenceGaps      []string                    `json:"preference_gaps"`
	PreferenceConflicts []string                    `json:"preference_conflicts"`
	JDSummary           string                      `json:"jd_summary"`
	ScreeningSummary    generation.ScreeningSummary `json:"screening_summary"`
}

func (s *Service) generateNarrative(
	ctx context.Context,
	app db.Application,
	signals JDSignals,
	gatePassed bool,
	technicalScore float64,
	technicalGaps []string,
	prefScore float64,
	prefGaps []string,
	prefConflicts []string,
) (string, error) {
	prompt, err := generation.RawPrompt("fit_narrative.txt")
	if err != nil {
		return "", err
	}

	input := narrativeInput{
		AntiPatternPassed:   gatePassed,
		TechnicalScore:      technicalScore,
		TechnicalGaps:       technicalGaps,
		PreferenceScore:     prefScore,
		PreferenceGaps:      prefGaps,
		PreferenceConflicts: prefConflicts,
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

func numericFromScore(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', 2, 64))
	return n
}
