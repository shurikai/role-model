package generation

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/uuid"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	resumeschema "github.com/shurikai/role-model/schema"
)

// schemaAllowedKeys is what $defs.jd_signals declares. The projection must
// emit exactly this set — the schema sets additionalProperties: false, so
// anything extra fails validation, and the document is poorer for anything
// missing.
var schemaAllowedKeys = []string{
	"required_skills", "preferred_skills", "seniority",
	"domain", "work_type", "culture_signals",
}

// compileJDSignalsSchema compiles the jd_signals subschema straight out of the
// embedded document schema, so these tests track the real contract rather than
// a copy of it that can drift.
func compileJDSignalsSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	if err := c.AddResource("resume.v1.json", bytes.NewReader(resumeschema.ResumeV1JSON)); err != nil {
		t.Fatalf("load schema: %v", err)
	}
	sch, err := c.Compile("resume.v1.json#/$defs/jd_signals")
	if err != nil {
		t.Fatalf("compile jd_signals subschema: %v", err)
	}
	return sch
}

func validateProjection(t *testing.T, sch *jsonschema.Schema, s JDSignals) map[string]any {
	t.Helper()
	raw, err := json.Marshal(s.forDocument())
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal projection: %v", err)
	}
	if err := sch.Validate(v); err != nil {
		t.Fatalf("projection does not validate against $defs.jd_signals: %v\ngot: %s", err, raw)
	}
	return v.(map[string]any)
}

// The bug this projection exists for. A current-era signals blob carries
// screening_summary, which $defs.jd_signals forbids; assigning the stored
// blob into meta made every such application fail generation outright with a
// 502 and no resume version written.
func TestForDocumentDropsScreeningSummary(t *testing.T) {
	sch := compileJDSignalsSchema(t)

	got := validateProjection(t, sch, JDSignals{
		RequiredSkills:  []string{"CI/CD", "observability"},
		PreferredSkills: []string{"RAG"},
		Seniority:       "staff",
		Domain:          "healthcare",
		WorkType:        "remote",
		CultureSignals:  []string{"remote-first"},
		ScreeningSummary: ScreeningSummary{
			Location: "Remote", Travel: "5%", Industry: "healthcare AI",
		},
	})

	if _, ok := got["screening_summary"]; ok {
		t.Error("screening_summary leaked into the document projection")
	}
	assertKeySet(t, got)
}

// The projection invariant, exercised by the newest field to land on
// JDSignals. skill_levels is extraction's, not the document's — the renderer
// has no use for a depth requirement — and the schema sets
// additionalProperties: false, so a field that leaked here would fail
// validation for every application carrying one.
//
// This test is really about the rule rather than about skill_levels: adding a
// field to JDSignals must not change what the document emits. If it ever
// should, the schema declares it first and documentJDSignals follows.
func TestForDocumentDropsSkillLevels(t *testing.T) {
	sch := compileJDSignalsSchema(t)

	got := validateProjection(t, sch, JDSignals{
		RequiredSkills:  []string{"Go", "Kafka"},
		PreferredSkills: []string{"Redis"},
		Seniority:       "senior",
		Domain:          "saas",
		WorkType:        "remote",
		CultureSignals:  []string{"remote-first"},
		SkillLevels: []SkillLevel{
			{Requirement: "Go", Level: "expert", Signal: "expert-level Go"},
		},
		CoreCompetencies: []string{"distributed systems design"},
		PrimaryStack:     []string{"Go"},
	})

	for _, key := range []string{"skill_levels", "core_competencies", "primary_stack"} {
		if _, ok := got[key]; ok {
			t.Errorf("%s leaked into the document projection", key)
		}
	}
	assertKeySet(t, got)
}

// The other half of the same defect, and the larger one: 15 stored
// applications predate the required/preferred split and carry the deprecated
// fields, which the schema forbids just as firmly.
func TestForDocumentDropsDeprecatedFieldsAndFallsBack(t *testing.T) {
	sch := compileJDSignalsSchema(t)

	got := validateProjection(t, sch, JDSignals{
		Seniority:        "senior",
		PrioritySkills:   []string{"Java", "Kafka"},
		DomainVocabulary: []string{"payments"},
	})

	for _, k := range []string{"priority_skills", "domain_vocabulary"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s leaked into the document projection", k)
		}
	}
	assertKeySet(t, got)

	// The fallback buildSkillsChecklist already applies: the requirement list
	// is present under the old name, and must not be silently dropped.
	req, _ := got["required_skills"].([]any)
	if len(req) != 2 || req[0] != "Java" {
		t.Errorf("required_skills = %v, want the priority_skills contents", got["required_skills"])
	}
}

// An explicit null fails "type": "array" where an omitted key would pass, so
// absent lists must marshal as [].
func TestForDocumentEmitsEmptyArraysNotNull(t *testing.T) {
	sch := compileJDSignalsSchema(t)
	got := validateProjection(t, sch, JDSignals{Seniority: "mid"})

	for _, k := range []string{"required_skills", "preferred_skills", "culture_signals"} {
		v, ok := got[k]
		if !ok {
			t.Errorf("%s missing", k)
			continue
		}
		if v == nil {
			t.Errorf("%s marshalled as null, want []", k)
		}
	}
}

// Real required_skills must win over the deprecated field when both are
// present, rather than the fallback shadowing current data.
func TestForDocumentPrefersRequiredOverPrioritySkills(t *testing.T) {
	got := JDSignals{
		RequiredSkills: []string{"Go"},
		PrioritySkills: []string{"COBOL"},
	}.forDocument()

	if !slices.Equal(got.RequiredSkills, []string{"Go"}) {
		t.Errorf("RequiredSkills = %v, want [Go]", got.RequiredSkills)
	}
}

// The wiring test, and the one that would actually have caught the shipped
// bug. The four above exercise forDocument in isolation; this one asserts the
// meta block the generator really emits validates against the real
// $defs.meta_block, given signals of the shape that broke production.
//
// Without this, a call site that assigned the stored blob into meta would pass
// every other test in this file.
func TestBuildMetaBlockValidatesAgainstSchema(t *testing.T) {
	c := jsonschema.NewCompiler()
	if err := c.AddResource("resume.v1.json", bytes.NewReader(resumeschema.ResumeV1JSON)); err != nil {
		t.Fatalf("load schema: %v", err)
	}
	sch, err := c.Compile("resume.v1.json#/$defs/meta_block")
	if err != nil {
		t.Fatalf("compile meta_block subschema: %v", err)
	}

	for _, tc := range []struct {
		name    string
		signals JDSignals
	}{
		{
			// Current era: the exact shape that produced a 502 in production.
			name: "signals carrying screening_summary",
			signals: JDSignals{
				RequiredSkills:   []string{"CI/CD"},
				PreferredSkills:  []string{"RAG"},
				Seniority:        "staff",
				Domain:           "healthcare",
				WorkType:         "remote",
				CultureSignals:   []string{"remote-first"},
				ScreeningSummary: ScreeningSummary{Location: "Remote", Travel: "5%"},
			},
		},
		{
			// Pre-split era: 15 stored applications look like this.
			name: "signals carrying deprecated fields",
			signals: JDSignals{
				Seniority:        "senior",
				PrioritySkills:   []string{"Java"},
				DomainVocabulary: []string{"payments"},
			},
		},
		{
			name:    "empty signals",
			signals: JDSignals{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := buildMetaBlock(uuid.New(), "Acme", "Staff Engineer", "test-model", tc.signals)

			raw, err := json.Marshal(meta)
			if err != nil {
				t.Fatalf("marshal meta: %v", err)
			}
			var v any
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatalf("unmarshal meta: %v", err)
			}
			if err := sch.Validate(v); err != nil {
				t.Fatalf("meta block does not validate: %v\ngot: %s", err, raw)
			}

			// And specifically: nothing from the wider signals type leaked in.
			m := v.(map[string]any)
			sig, ok := m["jd_signals"].(map[string]any)
			if !ok {
				t.Fatalf("jd_signals missing or not an object: %v", m["jd_signals"])
			}
			assertKeySet(t, sig)
		})
	}
}

func assertKeySet(t *testing.T, got map[string]any) {
	t.Helper()
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := slices.Clone(schemaAllowedKeys)
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Errorf("projection keys = %v, want %v", keys, want)
	}
}
