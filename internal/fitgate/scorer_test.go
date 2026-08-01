package fitgate

import (
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
			name:       "python preferred skill trips the python exclude",
			pref:       hardExclude(pythonLabel, "anti_pattern"),
			signals:    JDSignals{Domain: "saas", PreferredSkills: []string{"Python"}},
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
