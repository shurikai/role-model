package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

// careerExtractionMaxTokens caps the response. A whole career is the largest
// thing this pipeline asks a model for -- every employer, position,
// contribution and skill in one object -- and a truncated response is invalid
// JSON rather than a short career, so the limit exists to make hitting it a
// loud failure rather than a corrupt import.
const careerExtractionMaxTokens = 16384

// Extractor is the half of career extraction that talks to a model. An
// interface so the staging path can be exercised against a canned response:
// the model half is untestable without spending money and getting a different
// answer each time, and every decision worth testing lives on the other side
// of it in PlanDrafts.
type Extractor interface {
	Complete(ctx context.Context, systemPrompt, userContent string, maxTokens int64) (string, error)
}

// ExtractCareer turns pasted career text into staged drafts, ready for review.
//
// This is the path that did not exist. Stage 0's extractor produces
// contribution_drafts, and approving one requires a position_id that already
// exists — so an account with no employers and no positions could not use the
// import at all. This produces the employers and the positions too, with the
// dependency edges that let them be written in the right order.
func (s *Service) ExtractCareer(
	ctx context.Context, client Extractor, userID, batchID uuid.UUID, careerText string,
) ([]db.EntityDraft, error) {
	if strings.TrimSpace(careerText) == "" {
		return nil, fmt.Errorf("extract career: no text to read")
	}

	// The depth scale the extractor is told to choose from is the user's own,
	// for the same reason the JD prompt's seniority list is: the scale a skill
	// is recorded on and the scale the fit gate ranks against have to be one
	// scale. An account with no rows falls back to the shipped neutral set.
	levels, err := s.q.ListProficiencyLevelsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("extract career: list proficiency levels: %w", err)
	}
	values := make([]string, 0, len(levels))
	for _, l := range levels {
		values = append(values, `"`+l.Value+`"`)
	}
	if len(values) == 0 {
		values = []string{`"novice"`, `"proficient"`, `"expert"`}
	}

	prompt, err := generation.RenderCareerExtractionPrompt(generation.CareerExtractionPromptData{
		CareerText:        careerText,
		ProficiencyValues: strings.Join(values, ", "),
	})
	if err != nil {
		return nil, fmt.Errorf("extract career: render prompt: %w", err)
	}

	raw, err := client.Complete(ctx, prompt, careerText, careerExtractionMaxTokens)
	if err != nil {
		return nil, fmt.Errorf("extract career: model call: %w", err)
	}

	var x CareerExtraction
	if err := json.Unmarshal([]byte(stripFence(raw)), &x); err != nil {
		return nil, fmt.Errorf("extract career: parse response: %w (raw: %s)", err, raw)
	}

	planned, err := PlanDrafts(x)
	if err != nil {
		return nil, fmt.Errorf("extract career: %w", err)
	}
	return s.StageDrafts(ctx, userID, batchID, planned)
}

// stripFence removes a markdown code fence the model was asked not to emit.
// Asking is the first line of defence and this is the recovery: a fence turns
// the whole import into a parse error, which is a poor trade for three lines.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSuffix(strings.TrimSpace(s), "```")
}
