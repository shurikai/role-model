package fitgate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// The fit-gate eval harness.
//
// scorer_test.go covers matching mechanics — word boundaries, alternatives
// splitting, gate field routing. This file asks the different question: given
// a known career profile and known JD signals, does the gate reach the right
// conclusion? Without it, reweighting the scorer to fix one issue can silently
// reopen another, which is the situation #43 through #46 are in.
//
// Everything here is deterministic and offline. ScoreCapabilityFit,
// ScorePreferenceFit, and RunAntiPatternGate are pure functions, so the harness
// needs no database and no API key and runs inside `make test`.
//
// This is Layer A: fixed jd_signals in, scoring outcome out. Layer B — running
// raw JD markdown through Stage 1 extraction and checking it produces these
// signals — needs an API key and a tolerance model for LLM variance, and is
// deliberately out of scope. The `jd_fixture` field on each case names the JD
// the signals were authored from, so Layer B has somewhere to attach.
//
// Run `go test ./internal/fitgate -run TestFitEval -v` for the scorecard.

const (
	profilePath = "testdata/profile-sample.json"
	casesDir    = "testdata/cases"
)

// statusKnownGap marks a case that asserts correct behavior the scorer does not
// yet produce. Known gaps are reported but do not fail the build — they carry a
// defect's definition of done rather than pretending it isn't there.
//
// A known gap that starts PASSING does fail, and deliberately: it means someone
// fixed the underlying defect, and the marker and its issue need closing out.
const statusKnownGap = "known_gap"

type evalProfile struct {
	Name        string           `json:"name"`
	Source      string           `json:"source"`
	Skills      []evalSkill      `json:"skills"`
	Preferences []evalPreference `json:"preferences"`
}

// evalSkill carries proficiency and years. Proficiency now reaches the scorer;
// years still does not, and the fixture holds it anyway so the harness keeps
// modelling exactly what the production query selects and no more.
type evalSkill struct {
	Name        string   `json:"name"`
	Proficiency string   `json:"proficiency"`
	Years       *float64 `json:"years_experience"`
	IsActive    bool     `json:"is_active"`

	// Aliases and category mirror what ListActiveSkillMatchTermsByUser now
	// returns. Both are optional in the fixture: an entry that omits them
	// behaves exactly as it did when the query selected t.name alone.
	Aliases         []string `json:"aliases"`
	Category        string   `json:"category"`
	CategoryAliases []string `json:"category_aliases"`
}

// evalPreference mirrors the preferences table. Weight is a value, not a
// pointer, because the column is NOT NULL — a fixture row that forgot its
// weight would otherwise score as zero and quietly contribute nothing, which
// is the class of silence this harness exists to break.
type evalPreference struct {
	PreferenceType string   `json:"preference_type"`
	Label          string   `json:"label"`
	Aliases        []string `json:"aliases"`
	Sentiment      string   `json:"sentiment"`
	Weight         int16    `json:"weight"`
	IsHardGate     bool     `json:"is_hard_gate"`
}

type evalCase struct {
	Name        string      `json:"name"`
	Status      string      `json:"status"`
	Issue       string      `json:"issue"`
	Reason      string      `json:"reason"`
	JDFixture   string      `json:"jd_fixture"`
	Description string      `json:"description"`
	Signals     JDSignals   `json:"signals"`
	Expect      expectation `json:"expect"`
}

type expectation struct {
	CapabilityScore   *scoreBand  `json:"capability_score"`
	CapabilityGaps    *listExpect `json:"capability_gaps"`
	CapabilityPartial *listExpect `json:"capability_partial"`
	// Preference fit has no score to band. All four lists assert membership,
	// which is a stronger statement than a range was: a case can now name the
	// preference it expects rather than a floor chosen to sit above a ceiling.
	PreferenceMatches   *listExpect `json:"preference_matches"`
	PreferenceGaps      *listExpect `json:"preference_gaps"`
	PreferenceConflicts *listExpect `json:"preference_conflicts"`
	GatePassed          *bool       `json:"gate_passed"`
	GateHits            *listExpect `json:"gate_hits"`
}

// scoreBand asserts a range rather than an exact score. Exact assertions would
// fail on every legitimate reweighting — which #43 and #44 both require — and
// would be deleted within a week of being written. A band says what the score
// has to mean without freezing how it is computed.
type scoreBand struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

type listExpect struct {
	MustInclude []string `json:"must_include"`
	MustExclude []string `json:"must_exclude"`
	Empty       *bool    `json:"empty"`
}

// toPreferences converts the fixture profile into the db rows the scorer takes.
func (p evalProfile) toPreferences() []db.Preference {
	out := make([]db.Preference, 0, len(p.Preferences))
	for _, pref := range p.Preferences {
		out = append(out, db.Preference{
			ID:             uuid.New(),
			PreferenceType: pref.PreferenceType,
			Label:          pref.Label,
			Aliases:        pref.Aliases,
			Sentiment:      pref.Sentiment,
			Weight:         pref.Weight,
			IsHardGate:     pref.IsHardGate,
		})
	}
	return out
}

// activeSkills returns what ListActiveSkillMatchTermsByUser returns today:
// name, aliases, category, and proficiency, for active rows only.
//
// years_experience is still dropped here, in the one place that models the
// production query, so the loss stays visible rather than buried in the
// fixture. Proficiency is no longer dropped — it is compared against a level
// the JD asked for, where the JD asks for one. Nothing compares against years,
// so selecting it would put a column in front of the scorer with no use for it.
func (p evalProfile) activeSkills() []SkillTerm {
	var out []SkillTerm
	for _, s := range p.Skills {
		if s.IsActive {
			out = append(out, SkillTerm{
				Name:            s.Name,
				Aliases:         s.Aliases,
				Category:        s.Category,
				CategoryAliases: s.CategoryAliases,
				Proficiency:     s.Proficiency,
			})
		}
	}
	return out
}

// evalLabels reduces a preference list to the labels the expectations assert
// on. ScorePreferenceFit returns whole rows; the fixtures name preferences by
// label, the way a person reading the scorecard would.
func evalLabels(prefs []db.Preference) []string {
	out := make([]string, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, p.Label)
	}
	return out
}

// result is one case's computed outcome, kept together for the scorecard.
type result struct {
	capabilityScore   float64
	capabilityGaps    []string
	capabilityPartial []string
	prefMatches       []string
	preferenceGaps    []string
	prefConflicts     []string
	gatePassed        bool
	gateHits          []string
}

func TestFitEval(t *testing.T) {
	profile := loadProfile(t)
	cases := loadCases(t)

	prefs := profile.toPreferences()
	skills := profile.activeSkills()

	type row struct {
		name    string
		known   bool
		issue   string
		res     result
		failing []string
	}
	var scorecard []row

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var res result
			capability := ScoreCapabilityFit(skills, c.Signals, testLevels)
			res.capabilityScore, res.capabilityGaps = capability.Score, capability.Gaps
			for _, m := range capability.Partial {
				res.capabilityPartial = append(res.capabilityPartial, m.Requirement)
			}

			// The additive invariant, asserted rather than assumed. Skill
			// levels are opt-in per requirement: a case whose signals state no
			// depth anywhere must score exactly what it scored before levels
			// existed, and must produce no partial verdict. Every fixture here
			// predates the feature, so this holds the whole corpus as a
			// regression net against the common path changing under it.
			if len(c.Signals.SkillLevels) == 0 && len(capability.Partial) > 0 {
				t.Errorf("case states no skill_levels but produced %d partial match(es): %v\n"+
					"a requirement with no stated depth must take the original "+
					"full-credit path",
					len(capability.Partial), res.capabilityPartial)
			}
			matches, gaps, conflicts, hits := ScorePreferenceFit(prefs, c.Signals)
			res.prefMatches = evalLabels(matches)
			res.preferenceGaps = evalLabels(gaps)
			res.prefConflicts = evalLabels(conflicts)
			res.gateHits = evalLabels(hits)

			// Mirrors RunFitEvaluation: the gate boolean is derived from the
			// hard-gate rows scoring matched, not computed by a second pass.
			res.gatePassed = len(hits) == 0

			failures := checkExpectations(c.Expect, res)

			known := c.Status == statusKnownGap
			scorecard = append(scorecard, row{
				name: c.Name, known: known, issue: c.Issue, res: res, failing: failures,
			})

			switch {
			case known && len(failures) > 0:
				// The defect is still present. Report it; do not fail.
				t.Logf("KNOWN GAP (%s) — still failing, as expected:\n  %s\n  reason: %s",
					c.Issue, strings.Join(failures, "\n  "), c.Reason)

			case known && len(failures) == 0:
				t.Errorf("known gap %s now PASSES.\n"+
					"The underlying defect appears to be fixed. Remove the "+
					"\"status\": \"known_gap\" marker from %s/%s.json and close out %s.",
					c.Name, casesDir, c.Name, c.Issue)

			case len(failures) > 0:
				t.Errorf("%s:\n  %s", c.Description, strings.Join(failures, "\n  "))
			}
		})
	}

	t.Cleanup(func() {
		sort.Slice(scorecard, func(i, j int) bool {
			if scorecard[i].known != scorecard[j].known {
				return !scorecard[i].known
			}
			return scorecard[i].name < scorecard[j].name
		})

		var b strings.Builder
		fmt.Fprintf(&b, "\nfit gate scorecard — profile %q (%d active skills, %d preferences)\n\n",
			profile.Name, len(skills), len(prefs))
		// PREF is m/g/c — matched, unmentioned, conflicting — rather than a
		// score. Three counts read as what they are; a single number here was
		// the thing that could not be read back to the rows behind it.
		fmt.Fprintf(&b, "  %-42s %6s %9s %5s  %s\n", "CASE", "TECH", "PREF m/g/c", "GATE", "VERDICT")
		for _, r := range scorecard {
			gate := "pass"
			if !r.res.gatePassed {
				gate = "FAIL"
			}
			verdict := "ok"
			switch {
			case r.known && len(r.failing) > 0:
				verdict = "known gap " + r.issue
			case r.known:
				verdict = "known gap " + r.issue + " NOW PASSES"
			case len(r.failing) > 0:
				verdict = fmt.Sprintf("%d assertion(s) failed", len(r.failing))
			}
			pref := fmt.Sprintf("%d/%d/%d",
				len(r.res.prefMatches), len(r.res.preferenceGaps), len(r.res.prefConflicts))
			fmt.Fprintf(&b, "  %-42s %6.1f %9s %5s  %s\n",
				r.name, r.res.capabilityScore, pref, gate, verdict)
		}
		t.Log(b.String())
	})
}

// checkExpectations returns one message per unmet assertion. It collects every
// failure rather than stopping at the first, so a single run shows the whole
// picture for a case.
func checkExpectations(e expectation, r result) []string {
	var out []string
	out = append(out, checkBand("capability_score", e.CapabilityScore, r.capabilityScore)...)
	out = append(out, checkList("capability_gaps", e.CapabilityGaps, r.capabilityGaps)...)
	out = append(out, checkList("capability_partial", e.CapabilityPartial, r.capabilityPartial)...)
	out = append(out, checkList("preference_matches", e.PreferenceMatches, r.prefMatches)...)
	out = append(out, checkList("preference_gaps", e.PreferenceGaps, r.preferenceGaps)...)
	out = append(out, checkList("preference_conflicts", e.PreferenceConflicts, r.prefConflicts)...)
	out = append(out, checkList("gate_hits", e.GateHits, r.gateHits)...)

	if e.GatePassed != nil && *e.GatePassed != r.gatePassed {
		out = append(out, fmt.Sprintf("gate_passed: want %v, got %v (hits: %v)",
			*e.GatePassed, r.gatePassed, r.gateHits))
	}
	return out
}

func checkBand(field string, band *scoreBand, got float64) []string {
	if band == nil {
		return nil
	}
	var out []string
	if band.Min != nil && got < *band.Min {
		out = append(out, fmt.Sprintf("%s: %.1f is below the floor of %.1f", field, got, *band.Min))
	}
	if band.Max != nil && got > *band.Max {
		out = append(out, fmt.Sprintf("%s: %.1f is above the ceiling of %.1f", field, got, *band.Max))
	}
	return out
}

func checkList(field string, want *listExpect, got []string) []string {
	if want == nil {
		return nil
	}
	var out []string
	if want.Empty != nil && *want.Empty && len(got) > 0 {
		out = append(out, fmt.Sprintf("%s: want empty, got %v", field, got))
	}
	for _, w := range want.MustInclude {
		if !listContains(got, w) {
			out = append(out, fmt.Sprintf("%s: missing %q (got %v)", field, w, got))
		}
	}
	for _, w := range want.MustExclude {
		if listContains(got, w) {
			out = append(out, fmt.Sprintf("%s: %q should not be present (got %v)", field, w, got))
		}
	}
	return out
}

// listContains reports whether want appears in list, either as a whole entry or
// as one alternative inside a grouped entry. The grouped case matters because a
// capability gap holds the JD's original string: a gap of
// "TimescaleDB | ClickHouse" is a gap for TimescaleDB, and an assertion naming
// either alternative should see it.
func listContains(list []string, want string) bool {
	for _, entry := range list {
		if strings.EqualFold(entry, want) {
			return true
		}
		for _, alt := range splitAlternatives(entry) {
			if strings.EqualFold(alt, want) {
				return true
			}
		}
	}
	return false
}

func loadProfile(t *testing.T) evalProfile {
	t.Helper()
	var p evalProfile
	readJSON(t, profilePath, &p)
	if len(p.Skills) == 0 || len(p.Preferences) == 0 {
		t.Fatalf("%s: profile is missing skills or preferences", profilePath)
	}
	return p
}

func loadCases(t *testing.T) []evalCase {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(casesDir, "*.json"))
	if err != nil {
		t.Fatalf("glob %s: %v", casesDir, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no eval cases found in %s", casesDir)
	}

	cases := make([]evalCase, 0, len(paths))
	seen := make(map[string]string, len(paths))
	for _, p := range paths {
		var c evalCase
		readJSON(t, p, &c)
		if c.Name == "" {
			t.Fatalf("%s: case is missing a name", p)
		}
		if prev, dup := seen[c.Name]; dup {
			t.Fatalf("%s: duplicate case name %q, already used by %s", p, c.Name, prev)
		}
		seen[c.Name] = p

		switch c.Status {
		case "", statusKnownGap:
		default:
			t.Fatalf("%s: unknown status %q (want %q or omitted)", p, c.Status, statusKnownGap)
		}
		if c.Status == statusKnownGap && (c.Issue == "" || c.Reason == "") {
			t.Fatalf("%s: a known_gap case must carry both an issue and a reason", p)
		}
		cases = append(cases, c)
	}
	return cases
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
