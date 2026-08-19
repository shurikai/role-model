package generation

import (
	"encoding/json"
	"strings"
	"testing"
)

func rawSignals(t *testing.T, s string) *json.RawMessage {
	t.Helper()
	raw := json.RawMessage(s)
	return &raw
}

func TestBuildSkillsChecklist(t *testing.T) {
	for _, tc := range []struct {
		name    string
		signals string
		want    string
	}{
		{
			name: "all three sections populated",
			signals: `{
				"required_skills": ["Go", "PostgreSQL"],
				"preferred_skills": ["Kafka"],
				"core_competencies": ["production ownership of services"]
			}`,
			want: "Required skills:\n- Go\n- PostgreSQL\n" +
				"Preferred skills:\n- Kafka\n" +
				"Core competencies:\n- production ownership of services\n",
		},
		{
			// The AirBnB staff posting: named no technology at all, so both
			// skill lists are correctly empty and the competencies carry the
			// entire requirement set. Before core_competencies existed this
			// rendered as "(none listed)" twice and disabled every relevance
			// rule in the 2a prompt.
			name: "competencies only",
			signals: `{
				"required_skills": [],
				"preferred_skills": [],
				"core_competencies": ["payments and settlement systems", "setting technical direction"]
			}`,
			want: "Required skills:\n(none listed)\n" +
				"Preferred skills:\n(none listed)\n" +
				"Core competencies:\n- payments and settlement systems\n- setting technical direction\n",
		},
		{
			// Pre-split stored rows carry the requirement list under
			// priority_skills. The fallback must survive the addition of a
			// third section.
			name:    "deprecated priority_skills fallback",
			signals: `{"priority_skills": ["Java", "Spring Boot"]}`,
			want: "Required skills:\n- Java\n- Spring Boot\n" +
				"Preferred skills:\n(none listed)\n" +
				"Core competencies:\n(none listed)\n",
		},
		{
			// Every section empty still prints all three headings, so the
			// prompt sees one stable shape and an absence reads as a fact
			// about this JD rather than as a missing block.
			name:    "nothing extracted",
			signals: `{}`,
			want: "Required skills:\n(none listed)\n" +
				"Preferred skills:\n(none listed)\n" +
				"Core competencies:\n(none listed)\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildSkillsChecklist(rawSignals(t, tc.signals))
			if err != nil {
				t.Fatalf("buildSkillsChecklist: %v", err)
			}
			if got != tc.want {
				t.Errorf("checklist mismatch\ngot:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestBuildSkillsChecklistNilSignals(t *testing.T) {
	got, err := buildSkillsChecklist(nil)
	if err != nil {
		t.Fatalf("buildSkillsChecklist: %v", err)
	}
	if !strings.Contains(got, "no jd_signals") {
		t.Errorf("got %q, want a no-signals notice", got)
	}
}

// core_competencies is not declared in $defs.jd_signals, so it must not reach
// the document. This is the invariant documentJDSignals exists for, exercised
// against the field that motivated writing this test.
func TestForDocumentDropsCoreCompetencies(t *testing.T) {
	sch := compileJDSignalsSchema(t)

	got := validateProjection(t, sch, JDSignals{
		RequiredSkills:   []string{"Go"},
		CoreCompetencies: []string{"setting technical direction"},
		Seniority:        "staff",
	})

	if _, present := got["core_competencies"]; present {
		t.Error("core_competencies leaked into the document projection")
	}
	assertKeySet(t, got)
}
