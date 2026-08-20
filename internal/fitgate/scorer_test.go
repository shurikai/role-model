package fitgate

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

// Real label text from the seeded preference rows. The exact wording is what
// produces the collisions these tests exist to catch, so approximations would
// defeat the point.
const (
	consultingLabel = "IT consulting / staff augmentation model"
	defenseLabel    = "defense / aerospace"
	pythonLabel     = "Python as a primary language"
	typescriptLabel = "TypeScript / Node.js as primary language"
	cultureLabel    = "Big Four consulting culture"
	angularLabel    = "Angular as co-equal frontend requirement"
)

// hardGate builds a gate row the way the schema now stores one: a heavy
// negative that additionally carries is_hard_gate. Setting Sentiment alone
// would leave IsHardGate false and silently disarm every case below.
func hardGate(label, prefType string) db.Preference {
	return db.Preference{
		ID:             uuid.New(),
		Label:          label,
		PreferenceType: prefType,
		Sentiment:      "negative",
		Weight:         10,
		IsHardGate:     true,
	}
}

// labelsOf reduces a preference list to its labels. Every list ScorePreferenceFit
// returns is []db.Preference now — the full row is what the report stores and
// what the frontend renders — but a test only ever asserts on the label, and
// comparing whole rows would make a failure message unreadable.
func labelsOf(prefs []db.Preference) []string {
	out := make([]string, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, p.Label)
	}
	return out
}

// gateHitLabels runs the scorer and reports which hard-gate rows matched.
// The gate is no longer a separate pass — this is how RunFitEvaluation
// derives anti_pattern_passed, so the tests exercise the same path.
func gateHitLabels(prefs []db.Preference, signals JDSignals) (passed bool, labels []string) {
	_, _, _, hits := ScorePreferenceFit(prefs, signals)
	return len(hits) == 0, labelsOf(hits)
}

func TestHardGateMatching(t *testing.T) {
	tests := []struct {
		name       string
		pref       db.Preference
		signals    JDSignals
		wantPassed bool
	}{
		{
			// The regression case. "staff" is a whole word inside the
			// consulting label, so word-boundary matching alone does not save
			// this — only routing the preference away from the seniority
			// field does. Under the old behavior every staff-level JD tripped
			// here, and while the gate was blocking that killed the
			// application outright with a nonsense explanation.
			name:       "staff seniority does not trip the consulting exclude",
			pref:       hardGate(consultingLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", WorkType: "remote", Seniority: "staff"},
			wantPassed: true,
		},
		{
			name:       "genuine consulting culture signal still trips it",
			pref:       hardGate(consultingLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", CultureSignals: []string{"IT consulting / staff augmentation model"}},
			wantPassed: false,
		},
		{
			name:       "defense domain still trips the domain exclude",
			pref:       hardGate(defenseLabel, "domain"),
			signals:    JDSignals{Domain: "defense", Seniority: "principal"},
			wantPassed: false,
		},
		{
			// A domain exclude must not reach into the skills arrays.
			name:       "domain exclude ignores skills",
			pref:       hardGate(defenseLabel, "domain"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"defense"}},
			wantPassed: true,
		},
		{
			// Task 2: skills-shaped excludes could never fire before, because
			// nothing in the gate read the skills arrays at all. They fire
			// now, but against primary_stack rather than required_skills — the
			// label claims Python is what the role is BUILT ON, and only
			// primary_stack answers that question.
			name:       "a python-primary role trips the python exclude",
			pref:       hardGate(pythonLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", PrimaryStack: []string{"Python", "Django"}},
			wantPassed: false,
		},
		{
			// #68. The whole defect in one case: Python is required, but the
			// posting does not say the role is built on it. Under the old
			// routing this required skill alone tripped a hard gate about
			// EXPERT Python as a PRIMARY requirement.
			name:       "python as a required skill only does not trip the python exclude",
			pref:       hardGate(pythonLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"Python", "Django"}},
			wantPassed: true,
		},
		{
			// Inverted deliberately. A nice-to-have mention is not a
			// requirement, so it must not trip a hard exclude — see
			// prefFieldsFor.
			name:       "python preferred skill does not trip the python exclude",
			pref:       hardGate(pythonLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", PreferredSkills: []string{"Python"}},
			wantPassed: true,
		},
		{
			// The Citi case. Angular appeared once, in a "nice to have"
			// bullet, and that was enough to disqualify the role with a
			// narrative asserting a requirement the JD never stated.
			name: "angular as a preferred skill only does not trip the angular exclude",
			pref: hardGate(angularLabel, "primary_stack"),
			signals: JDSignals{
				Domain:          "fintech",
				RequiredSkills:  []string{"Java", "Spring Boot"},
				PreferredSkills: []string{"React", "Angular"},
			},
			wantPassed: true,
		},
		{
			// The other half: routing away from required_skills must not
			// disable the row. A posting built co-equally on Angular still
			// trips it.
			name: "angular in the primary stack still trips the angular exclude",
			pref: hardGate(angularLabel, "primary_stack"),
			signals: JDSignals{
				Domain:       "fintech",
				PrimaryStack: []string{"Java", "Angular"},
			},
			wantPassed: false,
		},
		{
			// #68, the OR-group half. This case previously asserted the
			// OPPOSITE, on the reasoning that an exclude should reach a single
			// option buried in a group. That reasoning is right for
			// anti_pattern and wrong here: a group means the posting offers
			// substitutes, and a language you can be excused from is not what
			// the role is built on. "Proficiency in Java and/or Python" is the
			// posting that made this concrete.
			name:       "an or-group in the primary stack does not trip an exclude on one member",
			pref:       hardGate(pythonLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", PrimaryStack: []string{"Java | Python | Go"}},
			wantPassed: true,
		},
		{
			// The converse, so the rule above is a distinction and not a
			// blanket disarm: no substitute is offered, so the claim stands.
			name:       "an ungrouped primary language still trips an exclude",
			pref:       hardGate(pythonLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", PrimaryStack: []string{"Python"}},
			wantPassed: false,
		},
		{
			// Extraction restating a grouped choice as a bare entry must not
			// reopen #68. collapseSubsumed drops the restatement.
			name:       "a bare restatement of a grouped member does not trip an exclude",
			pref:       hardGate(pythonLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", PrimaryStack: []string{"Java | Python", "Python", "Java"}},
			wantPassed: true,
		},
		{
			name:       "typescript in the primary stack trips the typescript exclude",
			pref:       hardGate(typescriptLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", PrimaryStack: []string{"TypeScript"}},
			wantPassed: false,
		},
		{
			name:       "unrelated stack does not trip a skills exclude",
			pref:       hardGate(pythonLabel, "primary_stack"),
			signals:    JDSignals{Domain: "saas", PrimaryStack: []string{"Go", "Kubernetes"}},
			wantPassed: true,
		},
		{
			// A bare-label exclude keeps presence semantics and stays on
			// anti_pattern. Splitting the types must not disarm these.
			name:       "a bare technology exclude still fires on a required skill",
			pref:       hardGate("C# / .NET", "anti_pattern"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"C#", "Azure"}},
			wantPassed: false,
		},
		{
			name:       "culture exclude matches a culture signal",
			pref:       hardGate(cultureLabel, "culture"),
			signals:    JDSignals{Domain: "saas", CultureSignals: []string{"Big Four consulting culture"}},
			wantPassed: false,
		},
		{
			// An ordinary negative still scores as a conflict, but it is not a
			// gate: it costs its weight and nothing more.
			name:       "a plain negative preference is not a gate hit",
			pref:       db.Preference{ID: uuid.New(), Label: defenseLabel, PreferenceType: "domain", Sentiment: "negative", Weight: 6},
			signals:    JDSignals{Domain: "defense"},
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passed, hits := gateHitLabels([]db.Preference{tt.pref}, tt.signals)
			if passed != tt.wantPassed {
				t.Errorf("passed = %v, want %v (hits: %v)", passed, tt.wantPassed, hits)
			}
			if !passed && len(hits) != 1 {
				t.Errorf("a failing gate should report exactly one hit, got %d", len(hits))
			}
		})
	}
}

// TestPreferenceReporting covers how the four lists divide the preference rows
// up. Every case here was an arithmetic hazard back when this function
// returned a score; each one survives as the list-shaped statement of the same
// property, because the property was never really about the number.
func TestPreferenceReporting(t *testing.T) {
	positive := func(label string, w int16) db.Preference {
		return db.Preference{ID: uuid.New(), Label: label, PreferenceType: "domain", Sentiment: "positive", Weight: w}
	}

	t.Run("a matched gate is reported as a gate and not as a conflict", func(t *testing.T) {
		// The gate list and the conflict list are structurally separate: a
		// disqualifying match and an ordinary matched negative are different
		// kinds of finding, and the narrative has to be able to say which it
		// is looking at. A gate hit used to be copied into both, which is
		// meaningful only when a single score is collapsing them anyway.
		prefs := []db.Preference{hardGate(pythonLabel, "anti_pattern")}
		signals := JDSignals{Domain: "saas", RequiredSkills: []string{"Python"}}

		_, _, conflicts, hits := ScorePreferenceFit(prefs, signals)
		if !slices.Contains(labelsOf(hits), pythonLabel) {
			t.Errorf("gateHits = %v, want the Python gate", labelsOf(hits))
		}
		if slices.Contains(labelsOf(conflicts), pythonLabel) {
			t.Errorf("conflicts = %v: a gate hit must not also appear here",
				labelsOf(conflicts))
		}
	})

	t.Run("an unmatched gate is reported nowhere", func(t *testing.T) {
		// The mirror property. Avoiding a stated dislike is the ideal outcome
		// and there is nothing to say about it — this is the behavior that,
		// under the old average, silently paid out a bonus instead.
		signals := JDSignals{Domain: "observability", RequiredSkills: []string{"Go"}}

		bare := []db.Preference{positive("observability", 8)}
		withGates := append(append([]db.Preference{}, bare...),
			hardGate(pythonLabel, "anti_pattern"),
			hardGate(typescriptLabel, "anti_pattern"),
		)

		wantM, wantG, wantC, wantH := ScorePreferenceFit(bare, signals)
		gotM, gotG, gotC, gotH := ScorePreferenceFit(withGates, signals)

		for _, tc := range []struct {
			name      string
			want, got []db.Preference
		}{
			{"matches", wantM, gotM},
			{"gaps", wantG, gotG},
			{"conflicts", wantC, gotC},
			{"gateHits", wantH, gotH},
		} {
			if !slices.Equal(labelsOf(tc.want), labelsOf(tc.got)) {
				t.Errorf("adding unmatched gates changed %s: %v -> %v",
					tc.name, labelsOf(tc.want), labelsOf(tc.got))
			}
		}
		if len(gotH) != 0 {
			t.Errorf("gateHits = %v, want none", labelsOf(gotH))
		}
	})

	t.Run("multiple matched gates all accumulate", func(t *testing.T) {
		// The hazard: the old gate returned on its first hit. Reusing that
		// loop would report one label no matter how many matched.
		prefs := []db.Preference{
			hardGate(pythonLabel, "anti_pattern"),
			hardGate(typescriptLabel, "anti_pattern"),
		}
		signals := JDSignals{Domain: "saas", RequiredSkills: []string{"Python", "TypeScript"}}

		_, _, _, hits := ScorePreferenceFit(prefs, signals)
		if len(hits) != 2 {
			t.Errorf("hits = %v, want both — the scorer must not short-circuit",
				labelsOf(hits))
		}
	})

	t.Run("a gate hit does not disturb the other lists", func(t *testing.T) {
		// This replaces the ceiling test. The old shape clamped the score on a
		// gate hit, and the hazard was clamping in the wrong direction and
		// raising a bad JD. With no score there is no clamp, and the property
		// that mattered underneath it survives: a gate hit is one finding
		// among several, and the positives it sits beside are still reported
		// honestly rather than being swallowed by it.
		prefs := []db.Preference{
			positive("distributed systems", 9),
			positive("observability", 8),
			hardGate(pythonLabel, "anti_pattern"),
		}
		// Nothing positive matches, and the gate does.
		signals := JDSignals{Domain: "saas", RequiredSkills: []string{"Python"}}

		matches, gaps, _, hits := ScorePreferenceFit(prefs, signals)
		if len(hits) != 1 {
			t.Fatalf("expected the gate to match, hits = %v", labelsOf(hits))
		}
		if len(matches) != 0 {
			t.Errorf("matches = %v, want none", labelsOf(matches))
		}
		for _, want := range []string{"distributed systems", "observability"} {
			if !slices.Contains(labelsOf(gaps), want) {
				t.Errorf("gaps = %v, want %q — an unmatched positive is still a gap "+
					"on a JD that trips a gate", labelsOf(gaps), want)
			}
		}
	})

	t.Run("a culture preference reads work arrangement", func(t *testing.T) {
		// Remoteness is recorded in work_type and nowhere else. Routing
		// culture preferences at culture_signals alone scored "remote-first"
		// as an unmet gap against a genuinely remote JD.
		remote := db.Preference{
			ID: uuid.New(), Label: "remote-first", PreferenceType: "culture",
			Sentiment: "positive", Weight: 8,
		}
		matches, gaps, _, _ := ScorePreferenceFit(
			[]db.Preference{remote},
			JDSignals{Domain: "observability", WorkType: "remote"},
		)
		if slices.Contains(labelsOf(gaps), remote.Label) {
			t.Errorf("remote-first reported as a gap on a remote JD")
		}
		if !slices.Contains(labelsOf(matches), remote.Label) {
			t.Errorf("matches = %v, want remote-first", labelsOf(matches))
		}
	})

	t.Run("a technology negative fires against the primary stack", func(t *testing.T) {
		// #49: ScorePreferenceFit matched against domain/work_type/culture
		// only, so this row could never fire — and because an unmatched
		// negative earned its weight, it paid out a bonus on every JD instead.
		// The field it fires against changed with #68 (primary_stack, not
		// required_skills); the property under test did not. What changed here
		// is only how the miss shows up: as an empty conflicts list rather
		// than as a score that failed to drop.
		jenkins := db.Preference{
			ID: uuid.New(), Label: "Jenkins administration as primary responsibility",
			PreferenceType: "primary_stack", Sentiment: "negative", Weight: 8,
		}
		prefs := []db.Preference{jenkins}

		_, _, hitConflicts, _ := ScorePreferenceFit(prefs, JDSignals{
			Domain: "platform", PrimaryStack: []string{"Jenkins", "Groovy"},
		})
		_, _, cleanConflicts, _ := ScorePreferenceFit(prefs, JDSignals{
			Domain: "observability", PrimaryStack: []string{"Go", "Kafka"},
		})

		if !slices.Contains(labelsOf(hitConflicts), jenkins.Label) {
			t.Errorf("conflicts = %v, want the Jenkins label", labelsOf(hitConflicts))
		}
		if len(cleanConflicts) != 0 {
			t.Errorf("conflicts = %v on a JD with no Jenkins in its stack, want none",
				labelsOf(cleanConflicts))
		}
	})

	t.Run("an unmatched negative is reported nowhere", func(t *testing.T) {
		// Not a conflict (the JD does not signal it), and not a gap either —
		// gaps are unmet *positives*. Reporting it as a gap would put "the
		// posting doesn't mention something you dislike" in front of a reader
		// as though it were a shortcoming of the role.
		disliked := db.Preference{
			ID: uuid.New(), Label: "on-call heavy", PreferenceType: "culture",
			Sentiment: "negative", Weight: 7,
		}
		matches, gaps, conflicts, hits := ScorePreferenceFit(
			[]db.Preference{disliked},
			JDSignals{Domain: "observability", WorkType: "remote"},
		)
		for name, got := range map[string][]db.Preference{
			"matches": matches, "gaps": gaps, "conflicts": conflicts, "gateHits": hits,
		} {
			if len(got) != 0 {
				t.Errorf("%s = %v, want none", name, labelsOf(got))
			}
		}
	})
}

// #68's Go-side safety net. jd_extraction.tmpl asks for the same
// deduplication, but a prompt is a request; this makes extraction noise
// harmless rather than merely unlikely.
func TestCollapseSubsumed(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			// The ZoomInfo shape: the must-haves offer "Java and/or Python",
			// and a later "Core Technical Stack" section restates both bare.
			name: "a group beats bare restatements of its members",
			in:   []string{"Java | Python", "Python", "Java", "Apache Kafka"},
			want: []string{"Java | Python", "Apache Kafka"},
		},
		{
			name: "unrelated entries survive untouched",
			in:   []string{"Go", "Kubernetes", "PostgreSQL"},
			want: []string{"Go", "Kubernetes", "PostgreSQL"},
		},
		{
			// Strict subset only. Equal groups subsume each other, and dropping
			// both would lose the entry entirely.
			name: "identical groups are both kept",
			in:   []string{"Java | Python", "Python | Java"},
			want: []string{"Java | Python", "Python | Java"},
		},
		{
			name: "a smaller group inside a larger one is dropped",
			in:   []string{"Java | Python | Go", "Java | Python"},
			want: []string{"Java | Python | Go"},
		},
		{
			// A blank entry yields no alternatives, and an empty set is a
			// subset of everything — it must be dropped rather than allowed to
			// match nothing loudly.
			name: "blank entries are dropped",
			in:   []string{"Go", "   "},
			want: []string{"Go"},
		},
		{
			name: "order is preserved",
			in:   []string{"Kafka", "Java | Python", "Java"},
			want: []string{"Kafka", "Java | Python"},
		},
		{name: "empty input", in: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := collapseSubsumed(tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("collapseSubsumed(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// #53. work_type and domain preferences were routed at single-value enums that
// their labels could not appear in, so roughly 50 of 139 possible weight was
// not merely unlikely to be earned but unearnable on any JD, ever.
func TestPreferenceRoutingReachesProseSignals(t *testing.T) {
	positive := func(label, prefType string, w int16) db.Preference {
		return db.Preference{
			ID: uuid.New(), Label: label, PreferenceType: prefType,
			Sentiment: "positive", Weight: w,
		}
	}

	tests := []struct {
		name    string
		pref    db.Preference
		signals JDSignals
		wantGap bool
	}{
		{
			// signals.WorkType is remote|hybrid|onsite|unknown. No work_type
			// label can ever appear in it; the role shape is in culture_signals.
			name:    "a work_type preference reads culture signals",
			pref:    positive("small team, high ownership", "work_type", 9),
			signals: JDSignals{WorkType: "remote", CultureSignals: []string{"small team", "high ownership"}},
		},
		{
			name: "a work_type preference reads core competencies",
			pref: positive("greenfield development", "work_type", 6),
			signals: JDSignals{
				WorkType:         "remote",
				CoreCompetencies: []string{"greenfield development of a new ingest service"},
			},
		},
		{
			// The enum has no logistics value, so extraction correctly says
			// "saas" and the real industry survives only in screening_summary.
			name: "a domain preference reads the screening summary industry",
			pref: positive("logistics", "domain", 9),
			signals: JDSignals{
				Domain: "saas",
				ScreeningSummary: generation.ScreeningSummary{
					Industry: "freight logistics visibility platform",
				},
			},
		},
		{
			// "observability" was reported as an unmet gap on a posting that
			// required observability work outright.
			name: "a domain preference reads core competencies",
			pref: positive("observability", "domain", 7),
			signals: JDSignals{
				Domain:           "healthcare",
				CoreCompetencies: []string{"observability and incident response for production services"},
			},
		},
		{
			name: "a culture preference reads core competencies",
			pref: positive("mentoring", "culture", 7),
			signals: JDSignals{
				WorkType:         "remote",
				CoreCompetencies: []string{"mentoring senior engineers"},
			},
		},
		{
			// The widening must not make everything match. A JD that genuinely
			// says nothing about a preference still reports it as a gap.
			name:    "a silent JD still reports the preference as a gap",
			pref:    positive("logistics", "domain", 9),
			signals: JDSignals{Domain: "healthcare", CoreCompetencies: []string{"claims adjudication"}},
			wantGap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gaps, _, _ := ScorePreferenceFit([]db.Preference{tt.pref}, tt.signals)
			gotGap := slices.Contains(labelsOf(gaps), tt.pref.Label)
			if gotGap != tt.wantGap {
				t.Errorf("reported as gap = %v, want %v (gaps: %v)",
					gotGap, tt.wantGap, labelsOf(gaps))
			}
		})
	}
}

// The oppositional pair that widening the work_type routing exposed. The two
// labels seeded before migration 015 tokenized to the same bag of words, so a
// JD mentioning internal tooling earned the positive and scored the conflict in
// the same pass. matchesSignal compares token runs and cannot tell "X over Y"
// from "Y over X" — the fix is disjoint labels, not a smarter matcher.
func TestOppositeWorkTypePreferencesDoNotBothMatch(t *testing.T) {
	prefs := []db.Preference{
		{ID: uuid.New(), Label: "product engineering", PreferenceType: "work_type",
			Sentiment: "positive", Weight: 8},
		{ID: uuid.New(), Label: "internal platform / developer tooling", PreferenceType: "work_type",
			Sentiment: "negative", Weight: 5},
	}
	signals := JDSignals{
		WorkType:       "remote",
		CultureSignals: []string{"internal platform / developer tooling"},
	}

	_, gaps, conflicts, _ := ScorePreferenceFit(prefs, signals)

	if !slices.Contains(labelsOf(conflicts), "internal platform / developer tooling") {
		t.Errorf("conflicts = %v, want the internal-tooling negative", labelsOf(conflicts))
	}
	if !slices.Contains(labelsOf(gaps), "product engineering") {
		t.Errorf("gaps = %v: an internal-tooling JD must not also earn the "+
			"product-engineering positive", labelsOf(gaps))
	}
}

func TestContainsPhrase(t *testing.T) {
	tests := []struct {
		needle, haystack string
		want             bool
	}{
		{"defense", "defense / aerospace", true},
		{"Python", "Python as a primary language", true},
		{"TypeScript", "TypeScript / Node.js as primary language", true},
		{"node.js", "TypeScript / Node.js as primary language", true},
		{"remote", "remote-first", true},
		{"defense / aerospace", "defense / aerospace", true},

		// Character overlap that is not a word match.
		{"go", "mongodb", false},
		{"ai", "maintain", false},
		{"java", "javascript", false},

		// Needle longer than haystack, and non-contiguous word runs.
		{"defense / aerospace", "defense", false},
		{"primary python", "Python as a primary language", false},

		// Empty needle matches nothing rather than everything.
		{"", "anything", false},
		{"   /  ", "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.needle+"|"+tt.haystack, func(t *testing.T) {
			if got := containsPhrase(tt.needle, tt.haystack); got != tt.want {
				t.Errorf("containsPhrase(%q, %q) = %v, want %v", tt.needle, tt.haystack, got, tt.want)
			}
		})
	}
}

// Known limitation, asserted so it is a documented decision rather than an
// untested assumption. Tokenization is exact: no stemming, no plural folding.
// "microservice" does not match "microservices". Fixing that means a stemmer,
// which is a bigger change than this pass warrants and risks its own class of
// false positives.
func TestContainsPhraseDoesNotStem(t *testing.T) {
	if containsPhrase("microservice", "microservices architecture") {
		t.Error("stemming appears to have been added; update this test and the doc comment")
	}
}

// The Citi Principal Java Engineer JD, which is what surfaced the
// double-counting. Two of its requirements each offer four interchangeable
// technologies; holding one of each satisfies both.
func TestScoreTechnicalFitOrGroups(t *testing.T) {
	citiRequired := []string{
		"Java",
		"Spring Boot | Quarkus | Micronaut | Vert.x",
		"Tekton | Harness | CircleCI | Jenkins",
	}

	tests := []struct {
		name       string
		skillNames []string
		signals    JDSignals
		wantScore  float64
		wantGaps   []string
	}{
		{
			// One alternative from each group is enough. Flattened into six
			// entries this scored 50% with three bogus gaps.
			name:       "one alternative from each group satisfies it fully",
			skillNames: []string{"Java", "Spring Boot", "Jenkins"},
			signals:    JDSignals{RequiredSkills: citiRequired},
			wantScore:  100.0,
			wantGaps:   nil,
		},
		{
			// An unmet group is one gap holding the whole entry, not one gap
			// per alternative.
			name:       "an unmet group reports a single gap",
			skillNames: []string{"Java"},
			signals: JDSignals{
				RequiredSkills: []string{"Java", "Quarkus | Micronaut | Vert.x"},
			},
			wantScore: 50.0,
			wantGaps:  []string{"Quarkus | Micronaut | Vert.x"},
		},
		{
			name:       "a group counts once toward the total, not once per alternative",
			skillNames: []string{"Jenkins"},
			signals: JDSignals{
				RequiredSkills: []string{"Tekton | Harness | CircleCI | Jenkins"},
			},
			wantScore: 100.0,
			wantGaps:  nil,
		},
		{
			// Preferred groups score the same way at half the weight, and
			// never produce gaps.
			name:       "preferred groups match on any alternative",
			skillNames: []string{"Java", "Redis"},
			signals: JDSignals{
				RequiredSkills:  []string{"Java"},
				PreferredSkills: []string{"Redis | Memcached | Hazelcast"},
			},
			wantScore: 100.0,
			wantGaps:  nil,
		},
		{
			name:       "an entry without the delimiter is unchanged",
			skillNames: []string{"Go"},
			signals:    JDSignals{RequiredSkills: []string{"Go", "Rust"}},
			wantScore:  50.0,
			wantGaps:   []string{"Rust"},
		},
		{
			// A slash is part of the skill name, not a separator.
			name:       "a slash in a skill name is not treated as an alternative",
			skillNames: []string{"CI/CD"},
			signals:    JDSignals{RequiredSkills: []string{"CI/CD"}},
			wantScore:  100.0,
			wantGaps:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fit := ScoreTechnicalFit(nameOnlySkills(tt.skillNames), tt.signals)
			score := fit.Score
			gaps := fit.Gaps
			if score != tt.wantScore {
				t.Errorf("score = %v, want %v", score, tt.wantScore)
			}
			if !slices.Equal(gaps, tt.wantGaps) {
				t.Errorf("gaps = %q, want %q", gaps, tt.wantGaps)
			}
		})
	}
}

// nameOnlySkills lifts bare tag names into SkillTerms carrying no aliases and
// no category. It keeps the name-matching tables in this file testing exactly
// what they always did — the matcher's string behavior, with neither of the
// two data-driven layers able to interfere.
func nameOnlySkills(names []string) []SkillTerm {
	out := make([]SkillTerm, 0, len(names))
	for _, n := range names {
		out = append(out, SkillTerm{Name: n})
	}
	return out
}

// citiSkills is a subset of the real stored skill tag names, kept verbatim
// because the whole failure was about their exact shape: the tag is "REST",
// not "REST APIs", and the JD phrase never contained it.
var citiSkills = []string{
	"Java", "REST", "GraphQL", "Spring Boot", "Jenkins", "Docker",
	"PostgreSQL", "Go", "Kafka", "React", "Splunk", "Harness",
}

// The Citi Principal Java Engineer JD again, this time for skill-name shape.
// Its extraction emitted "RESTful APIs" as a required skill while the stored
// tag is "REST", and the one-directional substring check reported a 10-year
// expert strength as a technical gap.
func TestScoreTechnicalFitSkillNameShapes(t *testing.T) {
	tests := []struct {
		name       string
		skillNames []string
		required   []string
		wantGaps   []string
	}{
		{
			// The fix. The canonical tag sits as a whole word inside the JD's
			// longer phrase, which the reverse direction now matches.
			name:       "a stored skill matches a longer JD phrase containing it",
			skillNames: citiSkills,
			required:   []string{"REST APIs"},
			wantGaps:   nil,
		},
		{
			// The forward direction still has to work: a JD naming the general
			// technology is answered by a specific implementation of it.
			name:       "a JD term inside a longer skill name still matches",
			skillNames: citiSkills,
			required:   []string{"SQL"},
			wantGaps:   nil,
		},
		{
			// GraphQL is genuinely held, so it is genuinely not a gap. Asserted
			// because the session that produced this test began from the
			// opposite assumption.
			name:       "graphql is held and does not gap",
			skillNames: citiSkills,
			required:   []string{"GraphQL"},
			wantGaps:   nil,
		},
		{
			// The reason the reverse direction is word-boundary matched rather
			// than a raw substring. Both of these would match unguarded.
			name:       "a short skill name does not match an unrelated JD term containing its letters",
			skillNames: []string{"Go"},
			required:   []string{"Google Cloud", "MongoDB", "Django"},
			wantGaps:   []string{"Google Cloud", "MongoDB", "Django"},
		},
		{
			// Whole post-fix Citi requirement set. Terraform | CloudFormation
			// is the only genuine miss, and it is preferred, so no gaps at all.
			name:       "the citi requirements are fully covered",
			skillNames: citiSkills,
			required: []string{
				"Java",
				"Docker | Kubernetes | OpenShift",
				"REST",
				"GraphQL",
				"Spring Boot | Quarkus | Micronaut | Vert.x",
				"Tekton | Harness | CircleCI | Jenkins",
			},
			wantGaps: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fit := ScoreTechnicalFit(nameOnlySkills(tt.skillNames), JDSignals{RequiredSkills: tt.required})
			gaps := fit.Gaps
			if !slices.Equal(gaps, tt.wantGaps) {
				t.Errorf("gaps = %q, want %q", gaps, tt.wantGaps)
			}
		})
	}
}

// Asserted so it stays a decision rather than a surprise: string matching
// alone still does not bridge a morphological difference. "REST" and
// "RESTful APIs" share no whole word and neither contains the other, so a
// skill carrying no aliases cannot answer the adjectival phrase.
//
// This is no longer a dead end, only the floor — see
// TestScoreTechnicalFitBridgesAdjectivalFormsViaAliases for the data that
// closes it. Canonicalization in jd_extraction.tmpl remains the first line of
// defence; aliases are the recovery path when it does not fire.
func TestScoreTechnicalFitDoesNotBridgeAdjectivalFormsOnNameAlone(t *testing.T) {
	fit := ScoreTechnicalFit(nameOnlySkills(citiSkills), JDSignals{RequiredSkills: []string{"RESTful APIs"}})
	gaps := fit.Gaps
	if len(gaps) == 0 {
		t.Error("matchesAny appears to bridge RESTful->REST on the name alone now; " +
			"update this test, the matchesAny doc comment, and the canonicalization " +
			"rule in jd_extraction.tmpl")
	}
}

// The recovery path the test above leaves open. The stored REST tag really
// does carry 'restful' in tags.aliases (database/seed/013_skills.sql), and
// reading that column is what turns a ten-year skill from a reported gap into
// a match.
func TestScoreTechnicalFitBridgesAdjectivalFormsViaAliases(t *testing.T) {
	skills := []SkillTerm{{Name: "REST", Aliases: []string{"rest api", "restful"}}}

	fit := ScoreTechnicalFit(skills, JDSignals{RequiredSkills: []string{"RESTful APIs"}})
	score := fit.Score
	gaps := fit.Gaps
	matches := fit.Matches
	if len(gaps) != 0 {
		t.Fatalf("gaps = %q, want none — the 'restful' alias should answer this", gaps)
	}
	if score != 100.0 {
		t.Errorf("score = %v, want 100", score)
	}
	if len(matches) != 1 || matches[0].Kind != MatchAlias {
		t.Fatalf("matches = %+v, want one alias match", matches)
	}
	if !slices.Equal(matches[0].Evidence, []string{"REST"}) {
		t.Errorf("evidence = %q, want [REST]", matches[0].Evidence)
	}
}

// competencySkills models the shape the production query now returns for the
// real profile: technology-named tags, each carrying the competency
// vocabulary of the category it sits in. Trimmed to the categories the
// regression below exercises.
var competencySkills = []SkillTerm{
	{Name: "Jenkins", Category: "Tools & CI/CD", CategoryAliases: []string{"ci/cd", "continuous integration"}},
	{Name: "GitHub Actions", Category: "Tools & CI/CD", CategoryAliases: []string{"ci/cd", "continuous integration"}},
	{Name: "Splunk", Category: "Observability", CategoryAliases: []string{"observability", "monitoring"}},
	{Name: "Dynatrace", Category: "Observability", CategoryAliases: []string{"observability", "monitoring"}},
	{Name: "JUnit", Category: "Testing", CategoryAliases: []string{"automated testing", "test automation"}},
	{Name: "Kafka", Category: "Protocols & Messaging", CategoryAliases: []string{"event-driven systems", "api design"}},
	{Name: "REST", Aliases: []string{"rest api", "restful"}, Category: "Protocols & Messaging", CategoryAliases: []string{"event-driven systems", "api design"}},
	{Name: "PostgreSQL", Aliases: []string{"postgres"}, Category: "Databases", CategoryAliases: []string{"data modeling", "database design"}},
}

// The defect this whole change exists for. A staff-level JD that names no
// concrete technology at all scored 0/100 with every requirement reported as
// a gap, and the narrative told the user they had no overlap with the role.
//
// The assertion that matters most is not that the score rose. It is that
// "auth/authz frameworks" and "access control" are STILL gaps: the user holds
// no auth tags, and a fix that painted those green would have replaced a
// false negative with a false positive.
func TestScoreTechnicalFitCompetencyWordedJD(t *testing.T) {
	signals := JDSignals{
		RequiredSkills: []string{
			"auth/authz frameworks", "API design", "access control",
			"automated testing", "system design", "observability",
			"event-driven systems", "CI/CD", "data modeling",
		},
		PreferredSkills: []string{"IaC", "multi-agent", "RAG"},
	}

	fit := ScoreTechnicalFit(competencySkills, signals)
	score := fit.Score
	gaps := fit.Gaps
	matches := fit.Matches

	wantGaps := []string{"auth/authz frameworks", "access control", "system design"}
	if !slices.Equal(gaps, wantGaps) {
		t.Errorf("gaps = %q, want %q", gaps, wantGaps)
	}
	if score == 0 {
		t.Fatal("score = 0; the competency-worded JD regression is back")
	}
	if score >= 100 {
		t.Fatalf("score = %v; genuine gaps should keep this well under 100", score)
	}

	// Every match must carry evidence, or the narrative has nothing to cite.
	byReq := map[string]SkillMatch{}
	for _, m := range matches {
		if len(m.Evidence) == 0 {
			t.Errorf("match %q carries no evidence", m.Requirement)
		}
		byReq[m.Requirement] = m
	}

	ci, ok := byReq["CI/CD"]
	if !ok {
		t.Fatal(`"CI/CD" was not matched`)
	}
	if ci.Kind != MatchCategory || ci.Category != "Tools & CI/CD" {
		t.Errorf("CI/CD matched as %v/%q, want category/Tools & CI/CD", ci.Kind, ci.Category)
	}
	if !slices.Equal(ci.Evidence, []string{"Jenkins", "GitHub Actions"}) {
		t.Errorf("CI/CD evidence = %q, want [Jenkins GitHub Actions]", ci.Evidence)
	}

	// A preferred skill naming a specific practice must not be answered by the
	// broad category it loosely relates to.
	if m, ok := byReq["IaC"]; ok {
		t.Errorf("IaC should be unmatched, got %+v", m)
	}
}

// Category credit is only reachable through a skill the user actually holds.
// A JD asking about a capability the user has no tags for must not find the
// category by vocabulary alone.
func TestScoreTechnicalFitCategoryRequiresAnActiveSkill(t *testing.T) {
	skills := []SkillTerm{
		{Name: "Jenkins", Category: "Tools & CI/CD", CategoryAliases: []string{"ci/cd"}},
	}

	fit := ScoreTechnicalFit(skills, JDSignals{
		RequiredSkills: []string{"observability", "CI/CD"},
	})
	gaps, matches := fit.Gaps, fit.Matches

	if !slices.Equal(gaps, []string{"observability"}) {
		t.Errorf("gaps = %q, want [observability] — no skill sits in an observability category", gaps)
	}
	if len(matches) != 1 || matches[0].Requirement != "CI/CD" {
		t.Fatalf("matches = %+v, want only CI/CD", matches)
	}
}

// Provenance ranks strongest-first. A requirement answerable both by a stored
// skill name and by the category around it should report the skill, since
// that is the stronger claim and the more useful thing to show a reader.
func TestScoreTechnicalFitPrefersDirectMatchOverCategory(t *testing.T) {
	skills := []SkillTerm{
		{Name: "Kafka", Category: "Protocols & Messaging", CategoryAliases: []string{"kafka", "messaging"}},
		{Name: "REST", Category: "Protocols & Messaging", CategoryAliases: []string{"kafka", "messaging"}},
	}

	fit := ScoreTechnicalFit(skills, JDSignals{RequiredSkills: []string{"Kafka"}})
	matches := fit.Matches
	if len(matches) != 1 {
		t.Fatalf("matches = %+v, want one", matches)
	}
	if matches[0].Kind != MatchDirect {
		t.Errorf("kind = %v, want direct", matches[0].Kind)
	}
	if !slices.Equal(matches[0].Evidence, []string{"Kafka"}) {
		t.Errorf("evidence = %q, want [Kafka] alone, not the whole category", matches[0].Evidence)
	}
}

// Found against live data, not imagined. A JD requiring "RAG" matched the
// Testing category and offered TDD, JUnit, and Mockito as evidence, because
// the raw-substring direction of matchesAny found "rag" inside the alias
// "test coverage". Category and alias vocabulary is whole-word matched for
// this reason; only a skill's own name keeps the substring direction.
func TestScoreTechnicalFitCategoryDoesNotMatchOnSubstrings(t *testing.T) {
	skills := []SkillTerm{
		{Name: "JUnit", Category: "Testing", CategoryAliases: []string{"automated testing", "test coverage"}},
	}

	fit := ScoreTechnicalFit(skills, JDSignals{RequiredSkills: []string{"RAG"}})
	gaps := fit.Gaps
	matches := fit.Matches
	if len(matches) != 0 {
		t.Errorf("matches = %+v, want none — 'rag' inside 'test coverage' is not a match", matches)
	}
	if !slices.Equal(gaps, []string{"RAG"}) {
		t.Errorf("gaps = %q, want [RAG]", gaps)
	}
}

func TestSplitAlternatives(t *testing.T) {
	tests := []struct {
		entry string
		want  []string
	}{
		{"Spring Boot | Quarkus", []string{"Spring Boot", "Quarkus"}},
		{"Go", []string{"Go"}},
		{"CI/CD", []string{"CI/CD"}},
		{"  Go  ", []string{"Go"}},
		{"Go |  | Rust", []string{"Go", "Rust"}},
		// A blank entry yields no alternatives, so it cannot match. An empty
		// needle would otherwise be a substring of every skill name.
		{"", nil},
		{"   ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			if got := splitAlternatives(tt.entry); !slices.Equal(got, tt.want) {
				t.Errorf("splitAlternatives(%q) = %q, want %q", tt.entry, got, tt.want)
			}
		})
	}
}

// A JD from which Stage 1 extracted no technical requirements used to score a
// perfect 100 with zero matches and zero evidence, and the narrative wrote
// confident coverage prose around it: the stored AirBnB report asserted
// "complete coverage across backend engineering, distributed systems, payment
// platforms" against an empty requirement list.
//
// "Answers none of the requirements" and "there were no requirements" are
// opposite findings. Scored is what keeps them apart.
func TestScoreTechnicalFitUnscoredWhenJDStatesNoRequirements(t *testing.T) {
	skills := nameOnlySkills([]string{"Java", "Spring Boot", "PostgreSQL"})

	fit := ScoreTechnicalFit(skills, JDSignals{})

	if fit.Scored {
		t.Fatal("Scored = true, want false — the JD stated nothing to score")
	}
	if fit.Score == 100 {
		t.Error("the unscored case is reporting a perfect score again")
	}
	if len(fit.Gaps) != 0 || len(fit.Matches) != 0 {
		t.Errorf("gaps = %v, matches = %v, want both empty", fit.Gaps, fit.Matches)
	}
}

// A profile that answers nothing must stay distinguishable from a JD that
// asked nothing: same zero score, opposite Scored.
func TestScoreTechnicalFitScoredWhenNothingMatches(t *testing.T) {
	fit := ScoreTechnicalFit(nameOnlySkills([]string{"Java"}), JDSignals{
		RequiredSkills: []string{"Erlang", "Elixir"},
	})

	if !fit.Scored {
		t.Fatal("Scored = false, want true — the JD stated requirements, they just went unanswered")
	}
	if fit.Score != 0 {
		t.Errorf("score = %v, want 0", fit.Score)
	}
	if len(fit.Gaps) != 2 {
		t.Errorf("gaps = %v, want both requirements reported", fit.Gaps)
	}
}
