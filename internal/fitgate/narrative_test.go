package fitgate

import (
	"encoding/json"
	"testing"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

// The case this file exists for: a staff posting that names no technology at
// all. Both skill lists extract correctly empty and every requirement the
// posting actually stated lives in core_competencies. When those did not reach
// the prompt, the model saw an empty report and wrote that the posting "does
// not state requirements specific enough to score against" — a false claim
// about a JD carrying ten of them.
//
// The competencies are scored now, so this is no longer the unscored case. The
// property it guards outlived that: the phrasing the posting used has to reach
// the narrative intact, because the score alone cannot say what was asked for.
func TestNarrativeInputCarriesCoreCompetencies(t *testing.T) {
	signals := JDSignals{
		ScreeningSummary: generation.ScreeningSummary{Industry: "e-commerce"},
		CoreCompetencies: []string{
			"designing and scaling distributed backend systems",
			"API design at scale",
		},
	}
	app := db.Application{RoleTitle: "Staff Software Engineer", CompanyName: "Airbnb"}

	// No skills at all: the posting's competencies are asked for and answered
	// by nothing, which is a real score of zero rather than an absent one.
	input := buildNarrativeInput(
		app, signals, true, ScoreCapabilityFit(nil, signals, testLevels), nil, nil, nil, nil)

	if input.CapabilityScore == nil {
		t.Error("a JD stating two competencies was not scored; competencies are scoring input now")
	} else if *input.CapabilityScore != 0 {
		t.Errorf("capability_score = %v, want 0 — nothing answered either competency", *input.CapabilityScore)
	}
	if len(input.CoreCompetencies) != 2 {
		t.Fatalf("core competencies did not reach the narrative input: %v", input.CoreCompetencies)
	}

	// The prompt reads JSON, not the struct, so assert on what it actually
	// receives: the competencies present and phrased as the posting phrased
	// them.
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal narrative input: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal narrative input: %v", err)
	}
	got, ok := decoded["core_competencies"].([]any)
	if !ok || len(got) != 2 {
		t.Fatalf("core_competencies missing from marshalled input: %s", raw)
	}
	if got[0] != "designing and scaling distributed backend systems" {
		t.Errorf("core competency phrasing altered: %v", got[0])
	}
}

// The unscored case still exists, and is now much narrower: a posting has to
// state no requirement of ANY shape to reach it. That is what makes an absent
// score meaningful — it says the posting asked for nothing, rather than that
// nobody looked at the part of it that did the asking.
func TestUnscoredRequiresNoRequirementsOfAnyShape(t *testing.T) {
	skills := []SkillTerm{{Name: "Go", Category: "Languages"}}

	if fit := ScoreCapabilityFit(skills, JDSignals{}, testLevels); fit.Scored {
		t.Error("a JD stating nothing at all was scored")
	}

	fit := ScoreCapabilityFit(skills, JDSignals{
		CoreCompetencies: []string{"designing and scaling distributed backend systems"},
	}, testLevels)
	if !fit.Scored {
		t.Fatal("a JD stating only competencies was not scored (#72)")
	}
	if fit.Score != 0 {
		t.Errorf("score = %v, want 0 — a bare Go skill does not answer that competency", fit.Score)
	}

	// An unanswered competency is not a gap. Gaps are the JD's named
	// requirements; reporting a sentence of prose among them reads as a
	// missing tool.
	if len(fit.Gaps) != 0 {
		t.Errorf("gaps = %v, want none — an unmet competency costs its point and says nothing", fit.Gaps)
	}
}

// A matched competency is filed with Origin: competency, so the narrative can
// tell "answered a named requirement" from "answered a stated capability".
// Kind stays orthogonal: a competency is answered directly, by alias, or by
// category exactly as a named skill is.
func TestMatchedCompetencyCarriesItsOrigin(t *testing.T) {
	skills := []SkillTerm{
		{Name: "Splunk", Category: "Observability", CategoryAliases: []string{"observability"}},
	}
	signals := JDSignals{CoreCompetencies: []string{"observability"}}

	fit := ScoreCapabilityFit(skills, signals, testLevels)

	if len(fit.Matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(fit.Matches))
	}
	if fit.Matches[0].Origin != OriginCompetency {
		t.Errorf("origin = %q, want %q", fit.Matches[0].Origin, OriginCompetency)
	}
	if fit.Matches[0].Kind != MatchCategory {
		t.Errorf("kind = %q, want %q — origin and kind are separate axes",
			fit.Matches[0].Kind, MatchCategory)
	}
}

// A competency the posting also stated as a named skill is one ask, not two.
// jd_extraction.tmpl forbids the duplication; this is the backstop.
func TestCompetencyRestatedAsSkillIsNotCountedTwice(t *testing.T) {
	skills := []SkillTerm{{Name: "Kafka"}}

	withDuplicate := ScoreCapabilityFit(skills, JDSignals{
		RequiredSkills:   []string{"Kafka"},
		CoreCompetencies: []string{"Kafka"},
	}, testLevels)

	withoutDuplicate := ScoreCapabilityFit(skills, JDSignals{
		RequiredSkills: []string{"Kafka"},
	}, testLevels)

	if withDuplicate.Score != withoutDuplicate.Score {
		t.Errorf("duplicate scored %v, plain requirement scored %v; extraction noise changed the number",
			withDuplicate.Score, withoutDuplicate.Score)
	}
	if len(withDuplicate.Matches) != 1 {
		t.Errorf("got %d matches, want 1 — the same ask was recorded twice", len(withDuplicate.Matches))
	}
}

// A JD carrying no competencies must score byte-for-byte what it scored before
// they were scoring input. The feature is additive or it is a rescoring of
// every application in the database.
func TestNoCompetenciesMeansNoChange(t *testing.T) {
	skills := []SkillTerm{{Name: "Go"}, {Name: "Kafka"}}
	signals := JDSignals{
		RequiredSkills:  []string{"Go", "Rust"},
		PreferredSkills: []string{"Kafka"},
	}

	fit := ScoreCapabilityFit(skills, signals, testLevels)

	// 2 of 4 required points, 1 of 1 preferred: 3/5.
	if want := 3.0 / 5 * 100; fit.Score != want {
		t.Errorf("score = %v, want %v", fit.Score, want)
	}
}
