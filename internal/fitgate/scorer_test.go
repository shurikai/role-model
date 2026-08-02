package fitgate

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// Real label text from the seeded preference rows. The exact wording is what
// produces the collisions these tests exist to catch, so approximations would
// defeat the point.
const (
	consultingLabel = "IT consulting / staff augmentation model"
	defenseLabel    = "defense / aerospace"
	pythonLabel     = "expert Python as primary requirement"
	typescriptLabel = "TypeScript / Node.js as primary language"
	cultureLabel    = "Big Four consulting culture"
	angularLabel    = "Angular as co-equal frontend requirement"
)

func hardExclude(label, prefType string) db.Preference {
	return db.Preference{
		ID:             uuid.New(),
		Label:          label,
		PreferenceType: prefType,
		Sentiment:      "hard_exclude",
	}
}

func TestRunAntiPatternGate(t *testing.T) {
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
			pref:       hardExclude(consultingLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", WorkType: "remote", Seniority: "staff"},
			wantPassed: true,
		},
		{
			name:       "genuine consulting culture signal still trips it",
			pref:       hardExclude(consultingLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", CultureSignals: []string{"IT consulting / staff augmentation model"}},
			wantPassed: false,
		},
		{
			name:       "defense domain still trips the domain exclude",
			pref:       hardExclude(defenseLabel, "domain"),
			signals:    JDSignals{Domain: "defense", Seniority: "principal"},
			wantPassed: false,
		},
		{
			// A domain exclude must not reach into the skills arrays.
			name:       "domain exclude ignores skills",
			pref:       hardExclude(defenseLabel, "domain"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"defense"}},
			wantPassed: true,
		},
		{
			// Task 2: skills-shaped excludes could never fire before, because
			// nothing in the gate read the skills arrays at all.
			name:       "python required skill trips the python exclude",
			pref:       hardExclude(pythonLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"Python", "Django"}},
			wantPassed: false,
		},
		{
			// Inverted deliberately. A nice-to-have mention is not a
			// requirement, so it must not trip a hard exclude — see
			// gateFieldsFor.
			name:       "python preferred skill does not trip the python exclude",
			pref:       hardExclude(pythonLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", PreferredSkills: []string{"Python"}},
			wantPassed: true,
		},
		{
			// The Citi case. Angular appeared once, in a "nice to have"
			// bullet, and that was enough to disqualify the role with a
			// narrative asserting a requirement the JD never stated.
			name:       "angular as a preferred skill only does not trip the angular exclude",
			pref:       hardExclude(angularLabel, "anti_pattern"),
			signals: JDSignals{
				Domain:          "fintech",
				RequiredSkills:  []string{"Java", "Spring Boot"},
				PreferredSkills: []string{"React", "Angular"},
			},
			wantPassed: true,
		},
		{
			// The other half: narrowing the gate to required skills must not
			// disable it. A real Angular requirement still trips it.
			name:       "angular as a required skill still trips the angular exclude",
			pref:       hardExclude(angularLabel, "anti_pattern"),
			signals: JDSignals{
				Domain:         "fintech",
				RequiredSkills: []string{"Java", "Angular"},
			},
			wantPassed: false,
		},
		{
			// An exclude must still reach an alternative inside an OR-group.
			name:       "an exclude matches an alternative inside an or-group",
			pref:       hardExclude(pythonLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"Java | Python | Go"}},
			wantPassed: false,
		},
		{
			name:       "typescript required skill trips the typescript exclude",
			pref:       hardExclude(typescriptLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"TypeScript"}},
			wantPassed: false,
		},
		{
			name:       "unrelated stack does not trip a skills exclude",
			pref:       hardExclude(pythonLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", RequiredSkills: []string{"Go", "Kubernetes"}},
			wantPassed: true,
		},
		{
			name:       "culture exclude matches a culture signal",
			pref:       hardExclude(cultureLabel, "culture"),
			signals:    JDSignals{Domain: "saas", CultureSignals: []string{"Big Four consulting culture"}},
			wantPassed: false,
		},
		{
			name:       "non-hard-exclude preferences are ignored by the gate",
			pref:       db.Preference{ID: uuid.New(), Label: defenseLabel, PreferenceType: "domain", Sentiment: "negative"},
			signals:    JDSignals{Domain: "defense"},
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passed, hits := RunAntiPatternGate([]db.Preference{tt.pref}, tt.signals)
			if passed != tt.wantPassed {
				t.Errorf("passed = %v, want %v (hits: %d)", passed, tt.wantPassed, len(hits))
			}
			if !passed && len(hits) != 1 {
				t.Errorf("a failing gate should report exactly one hit, got %d", len(hits))
			}
		})
	}
}

func TestContainsPhrase(t *testing.T) {
	tests := []struct {
		needle, haystack string
		want             bool
	}{
		{"defense", "defense / aerospace", true},
		{"Python", "expert Python as primary requirement", true},
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
		{"primary python", "expert Python as primary requirement", false},

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
			score, gaps := ScoreTechnicalFit(tt.skillNames, tt.signals)
			if score != tt.wantScore {
				t.Errorf("score = %v, want %v", score, tt.wantScore)
			}
			if !slices.Equal(gaps, tt.wantGaps) {
				t.Errorf("gaps = %q, want %q", gaps, tt.wantGaps)
			}
		})
	}
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
			_, gaps := ScoreTechnicalFit(tt.skillNames, JDSignals{RequiredSkills: tt.required})
			if !slices.Equal(gaps, tt.wantGaps) {
				t.Errorf("gaps = %q, want %q", gaps, tt.wantGaps)
			}
		})
	}
}

// Known limitation, asserted so it stays a decision rather than a surprise.
// "REST" and "RESTful APIs" share no whole word and neither contains the
// other, so no amount of matching in this file connects them. Canonicalizing
// the adjectival form is jd_extraction.tmpl's job; if that regresses, this is
// the shape the gap comes back in.
func TestScoreTechnicalFitDoesNotBridgeAdjectivalForms(t *testing.T) {
	_, gaps := ScoreTechnicalFit(citiSkills, JDSignals{RequiredSkills: []string{"RESTful APIs"}})
	if len(gaps) == 0 {
		t.Error("matchesAny appears to bridge RESTful->REST now; update this test, " +
			"the matchesAny doc comment, and the canonicalization rule in jd_extraction.tmpl")
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
