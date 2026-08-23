package intake

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// A canned extraction of a career with no software in it, which is the point:
// the planner reads refs and shapes, never vocabulary.
const clinicalExtraction = `{
  "employers": [
    {"ref": "e1", "name": "County Health Network", "industry": "public health"},
    {"ref": "e2", "name": "St. Brigid Hospital", "industry": "acute care"}
  ],
  "positions": [
    {"ref": "p1", "employer_ref": "e2", "title": "Staff Nurse", "started_on": "2014-06", "ended_on": "2019-02"},
    {"ref": "p2", "employer_ref": "e1", "title": "Charge Nurse", "industry_level": "senior", "started_on": "2019-03"}
  ],
  "contributions": [
    {"position_ref": "p2", "summary": "Rebuilt referral intake across six sites.",
     "full_description": "Cut average wait from three weeks to four days.",
     "scale_context": "six sites, roughly 400 referrals a month",
     "tags": [{"category": "Clinical", "name": "Triage"}]},
    {"position_ref": "p1", "summary": "Ran the night float rotation.",
     "full_description": "Covered admissions and rapid response overnight.",
     "tags": [{"category": "Clinical", "name": "Rapid Response"}]}
  ],
  "skills": [
    {"category": "Certifications", "tag": "ACLS", "proficiency": "expert", "years_experience": 8},
    {"category": "Charting Systems", "tag": "Epic", "proficiency": "proficient", "years_experience": null}
  ]
}`

func parse(t *testing.T, s string) CareerExtraction {
	t.Helper()
	var x CareerExtraction
	if err := json.Unmarshal([]byte(s), &x); err != nil {
		t.Fatalf("parse extraction: %v", err)
	}
	return x
}

// The bug this planner exists to prevent: a contribution attached to the wrong
// position, which nobody notices until a job is missing from a resume months
// later. The dependency edges have to point at the drafts the refs named.
func TestPlanDraftsWiresDependenciesToTheNamedParents(t *testing.T) {
	drafts, err := PlanDrafts(parse(t, clinicalExtraction))
	if err != nil {
		t.Fatalf("PlanDrafts: %v", err)
	}
	if len(drafts) != 8 {
		t.Fatalf("planned %d drafts, want 8 (2 employers, 2 positions, 2 contributions, 2 skills)", len(drafts))
	}

	byKind := map[string][]PlannedDraft{}
	byID := map[string]PlannedDraft{}
	for _, d := range drafts {
		byKind[d.Kind] = append(byKind[d.Kind], d)
		byID[d.ID.String()] = d
	}

	// Every position points at an employer draft, every contribution at a
	// position draft — and the target must be a draft of the right KIND, not
	// merely a uuid that exists.
	for _, p := range byKind[KindPosition] {
		if len(p.DependsOn) != 1 {
			t.Fatalf("position draft has %d dependencies, want 1", len(p.DependsOn))
		}
		if parent, ok := byID[p.DependsOn[0].String()]; !ok || parent.Kind != KindEmployer {
			t.Errorf("position depends on %v, which is not an employer draft", p.DependsOn[0])
		}
	}
	for _, c := range byKind[KindContribution] {
		if len(c.DependsOn) != 1 {
			t.Fatalf("contribution draft has %d dependencies, want 1", len(c.DependsOn))
		}
		if parent, ok := byID[c.DependsOn[0].String()]; !ok || parent.Kind != KindPosition {
			t.Errorf("contribution depends on %v, which is not a position draft", c.DependsOn[0])
		}
	}

	// Skills depend on nothing: ResolveOrCreateTag builds their chain, and a
	// skill is a claim about the person rather than about any one job.
	for _, s := range byKind[KindSkill] {
		if len(s.DependsOn) != 0 {
			t.Errorf("skill draft depends on %v; skills have no parents", s.DependsOn)
		}
	}

	// The refs actually distinguish: the Charge Nurse contribution must hang
	// off the Charge Nurse position, not off whichever came first.
	var chargeNurse PlannedDraft
	for _, p := range byKind[KindPosition] {
		if strings.Contains(string(p.Payload), "Charge Nurse") {
			chargeNurse = p
		}
	}
	for _, c := range byKind[KindContribution] {
		if strings.Contains(string(c.Payload), "referral intake") {
			if c.DependsOn[0] != chargeNurse.ID {
				t.Error("the referral-intake contribution was attached to the wrong position")
			}
		}
	}
}

// A ref that names nothing is an ERROR, never a skipped row. An orphaned
// position resolves against nothing and the person finds out by noticing a job
// missing from their resume.
func TestPlanDraftsRejectsDanglingRefs(t *testing.T) {
	for _, tt := range []struct{ name, body, want string }{
		{
			name: "position names an employer that does not exist",
			body: `{"employers":[{"ref":"e1","name":"Acme"}],
			        "positions":[{"ref":"p1","employer_ref":"e9","title":"Nurse","started_on":"2020-01"}]}`,
			want: "e9",
		},
		{
			name: "contribution names a position that does not exist",
			body: `{"contributions":[{"position_ref":"p9","summary":"x","full_description":"y"}]}`,
			want: "p9",
		},
		{
			name: "a ref is reused",
			body: `{"employers":[{"ref":"e1","name":"A"},{"ref":"e1","name":"B"}]}`,
			want: "twice",
		},
		{
			name: "an employer has no name",
			body: `{"employers":[{"ref":"e1","name":"  "}]}`,
			want: "no name",
		},
		{
			name: "a skill has no category",
			body: `{"skills":[{"tag":"ACLS","proficiency":"expert"}]}`,
			want: "category",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PlanDrafts(parse(t, tt.body))
			if err == nil {
				t.Fatal("planned drafts from a malformed extraction")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

// An empty extraction is not an error. A batch of text that turns out to
// describe no career is a real answer, and it must not look like a failure.
func TestPlanDraftsAcceptsAnEmptyExtraction(t *testing.T) {
	drafts, err := PlanDrafts(CareerExtraction{})
	if err != nil {
		t.Fatalf("PlanDrafts on an empty extraction: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("planned %d drafts from nothing", len(drafts))
	}
}

// The planner's output has to feed the resolver's ordering. Planned drafts
// arrive parents-first already, but topoOrder must not depend on that — and
// this is the test that connects the two halves.
func TestPlannedDraftsResolveInDependencyOrder(t *testing.T) {
	drafts, err := PlanDrafts(parse(t, clinicalExtraction))
	if err != nil {
		t.Fatalf("PlanDrafts: %v", err)
	}

	// Hand them to topoOrder REVERSED, so passing cannot be an accident of the
	// planner's emission order.
	var rows []db.EntityDraft
	for i := len(drafts) - 1; i >= 0; i-- {
		d := drafts[i]
		payload := json.RawMessage(d.Payload)
		rows = append(rows, db.EntityDraft{
			ID: d.ID, Kind: d.Kind, Status: "pending",
			DependsOn: d.DependsOn, Payload: &payload,
		})
	}

	order, stuck := topoOrder(rows, map[uuid.UUID]uuid.UUID{})
	if len(stuck) != 0 {
		t.Fatalf("stuck drafts: %v", stuck)
	}
	seen := map[string]bool{}
	for _, d := range order {
		for _, dep := range d.DependsOn {
			if !seen[dep.String()] {
				t.Fatalf("draft %s (%s) resolved before its dependency", d.ID, d.Kind)
			}
		}
		seen[d.ID.String()] = true
	}
}

// stubExtractor returns a canned response, so the staging path is exercised
// without a model call. The point of the Extractor interface.
type stubExtractor struct{ response string }

func (s stubExtractor) Complete(_ context.Context, _, _ string, _ int64) (string, error) {
	return s.response, nil
}

// A model asked not to emit a code fence sometimes emits one anyway, and a
// fence turns the whole import into a parse error. Asking is the first line of
// defence; this is the recovery.
func TestStripFence(t *testing.T) {
	want := `{"employers":[]}`
	for _, in := range []string{
		want,
		"```json\n" + want + "\n```",
		"```\n" + want + "\n```",
		"  \n```json\n" + want + "\n```  ",
	} {
		if got := stripFence(in); strings.TrimSpace(got) != want {
			t.Errorf("stripFence(%q) = %q, want %q", in, got, want)
		}
	}
}

// Empty text is refused before a model call rather than after. Sending an
// empty prompt costs money to be told nothing.
func TestExtractCareerRefusesEmptyText(t *testing.T) {
	svc := &Service{}
	for _, text := range []string{"", "   ", "\n\t"} {
		if _, err := svc.ExtractCareer(context.Background(), stubExtractor{}, uuid.New(), uuid.New(), text); err == nil {
			t.Errorf("ExtractCareer accepted %q", text)
		}
	}
}
