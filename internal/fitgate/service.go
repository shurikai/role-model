package fitgate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// and preferences, runs the anti-pattern gate and scoring, and persists the
// result as a new fit_reports row.
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

	if passed, hits := RunAntiPatternGate(prefs, signals); !passed {
		hitsJSON, err := marshalRawNonEmpty(hits)
		if err != nil {
			return nil, fmt.Errorf("fit evaluation: marshal anti-pattern hits: %w", err)
		}
		report, err := s.q.CreateFitReport(ctx, db.CreateFitReportParams{
			ID:                uuid.New(),
			UserID:            userID,
			ApplicationID:     appIDParam,
			AntiPatternPassed: false,
			AntiPatternHits:   hitsJSON,
		})
		if err != nil {
			return nil, fmt.Errorf("fit evaluation: store gate failure: %w", err)
		}
		return &report, nil
	}

	var (
		wg                        sync.WaitGroup
		technicalScore, prefScore float64
		technicalGaps, prefGaps   []string
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		technicalScore, technicalGaps = ScoreTechnicalFit(skillNames, signals)
	}()
	go func() {
		defer wg.Done()
		prefScore, prefGaps = ScorePreferenceFit(prefs, signals)
	}()
	wg.Wait()

	narrative, err := s.generateNarrative(ctx, app, signals, technicalScore, technicalGaps, prefScore, prefGaps)
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

	report, err := s.q.CreateFitReport(ctx, db.CreateFitReportParams{
		ID:                uuid.New(),
		UserID:            userID,
		ApplicationID:     appIDParam,
		AntiPatternPassed: true,
		TechnicalScore:    numericFromScore(technicalScore),
		TechnicalGaps:     technicalGapsJSON,
		PreferenceScore:   numericFromScore(prefScore),
		PreferenceGaps:    prefGapsJSON,
		Narrative:         &narrative,
	})
	if err != nil {
		return nil, fmt.Errorf("fit evaluation: store report: %w", err)
	}
	return &report, nil
}

type narrativeInput struct {
	AntiPatternPassed bool     `json:"anti_pattern_passed"`
	TechnicalScore    float64  `json:"technical_score"`
	TechnicalGaps     []string `json:"technical_gaps"`
	PreferenceScore   float64  `json:"preference_score"`
	PreferenceGaps    []string `json:"preference_gaps"`
	JDSummary         string   `json:"jd_summary"`
}

func (s *Service) generateNarrative(
	ctx context.Context,
	app db.Application,
	signals JDSignals,
	technicalScore float64,
	technicalGaps []string,
	prefScore float64,
	prefGaps []string,
) (string, error) {
	prompt, err := generation.RawPrompt("fit_narrative.txt")
	if err != nil {
		return "", err
	}

	input := narrativeInput{
		AntiPatternPassed: true,
		TechnicalScore:    technicalScore,
		TechnicalGaps:     technicalGaps,
		PreferenceScore:   prefScore,
		PreferenceGaps:    prefGaps,
		JDSummary:         fmt.Sprintf("%s at %s (%s)", app.RoleTitle, app.CompanyName, signals.Domain),
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal narrative input: %w", err)
	}

	return s.client.Complete(ctx, prompt, string(inputJSON), narrativeMaxTokens)
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
