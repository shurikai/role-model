package fitgate

import (
	"encoding/json"
	"testing"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

// The case this file exists for: a staff posting that names no technology at
// all. Both skill lists extract correctly empty, ScoreCapabilityFit returns
// Scored: false, and every requirement the posting actually stated lives in
// core_competencies. When those did not reach the prompt, the model saw an
// empty report and wrote that the posting "does not state requirements
// specific enough to score against" — a false claim about a JD carrying ten
// of them.
func TestNarrativeInputCarriesCoreCompetenciesWhenUnscored(t *testing.T) {
	signals := JDSignals{
		ScreeningSummary: generation.ScreeningSummary{Industry: "e-commerce"},
		CoreCompetencies: []string{
			"designing and scaling distributed backend systems",
			"API design at scale",
		},
	}
	app := db.Application{RoleTitle: "Staff Software Engineer", CompanyName: "Airbnb"}

	input := buildNarrativeInput(
		app, signals, true, ScoreCapabilityFit(nil, signals, testLevels), nil, nil, nil, nil)

	if input.CapabilityScore != nil {
		t.Errorf("unscored JD carried a capability_score: %v", *input.CapabilityScore)
	}
	if len(input.CoreCompetencies) != 2 {
		t.Fatalf("core competencies did not reach the narrative input: %v", input.CoreCompetencies)
	}

	// The prompt reads JSON, not the struct, so assert on what it actually
	// receives: the score absent entirely rather than zero, the competencies
	// present.
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal narrative input: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal narrative input: %v", err)
	}
	if _, present := decoded["capability_score"]; present {
		t.Error("capability_score present on an unscored JD; absent is what the prompt reads as unassessed")
	}
	got, ok := decoded["core_competencies"].([]any)
	if !ok || len(got) != 2 {
		t.Fatalf("core_competencies missing from marshalled input: %s", raw)
	}
	if got[0] != "designing and scaling distributed backend systems" {
		t.Errorf("core competency phrasing altered: %v", got[0])
	}
}

// Competencies are context, never findings. A JD stating them must still
// report no matches and no gaps, or the narrative would have grounds to
// describe coverage that was never scored.
func TestCoreCompetenciesAreNotScoredAsMatchesOrGaps(t *testing.T) {
	signals := JDSignals{
		CoreCompetencies: []string{"designing and scaling distributed backend systems"},
	}
	skills := []SkillTerm{{Name: "Go", Category: "Languages"}}

	technical := ScoreCapabilityFit(skills, signals, testLevels)
	if technical.Scored {
		t.Error("a JD stating only competencies was scored; competencies are not scoring input")
	}
	if len(technical.Matches) != 0 || len(technical.Gaps) != 0 || len(technical.Partial) != 0 {
		t.Errorf("competencies produced findings: matches=%v gaps=%v partial=%v",
			technical.Matches, technical.Gaps, technical.Partial)
	}
}
