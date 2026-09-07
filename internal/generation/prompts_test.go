package generation

import (
	"os/exec"
	"strings"
	"testing"
)

// The whole provenance scheme rests on promptFingerprint producing exactly the
// hash git produces for the same bytes. If that ever drifts, every blob
// recorded in generation_params becomes unresolvable against the repository,
// silently -- so assert it against the real git binary rather than a
// hand-copied constant.
func TestPromptFingerprintMatchesGitHashObject(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	for _, name := range []string{
		jdExtractionPrompt,
		careerExtractPrompt,
		resumeBodyPrompt,
		resumeSummaryPrompt,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := promptFingerprint(name)
			if err != nil {
				t.Fatalf("promptFingerprint(%q): %v", name, err)
			}

			out, err := exec.Command("git", "hash-object", "prompts/"+name).Output()
			if err != nil {
				t.Fatalf("git hash-object %q: %v", name, err)
			}
			want := strings.TrimSpace(string(out))

			if got != want {
				t.Errorf("fingerprint mismatch for %s:\n  computed: %s\n  git:      %s",
					name, got, want)
			}
		})
	}
}

// Templates carry {{/* */}} rationale headers. Those must never reach the
// model, and must not leave a leading blank line -- both would silently change
// the prompt. The `-}}` trim marker is what prevents the newline; this test is
// what catches its removal.
func TestPromptCommentsDoNotLeak(t *testing.T) {
	cases := []struct {
		name string
		data any
	}{
		{jdExtractionPrompt, extractPromptData{JobDescription: "x"}},
		{resumeSummaryPrompt, resumeSummaryPromptData{
			CompanyName: "ACME", RoleTitle: "Staff Eng",
			JDSignals: "{}", HeaderTitle: "Staff Backend Engineer", Body: "{}",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := renderPrompt(tc.name, tc.data)
			if err != nil {
				t.Fatalf("render %q: %v", tc.name, err)
			}
			if strings.Contains(out, "unversioned by design") {
				t.Errorf("%s: template comment leaked into rendered prompt", tc.name)
			}
			if strings.HasPrefix(out, "\n") {
				t.Errorf("%s: rendered prompt starts with a newline "+
					"(missing `-}}` trim marker on the comment header)", tc.name)
			}
		})
	}
}

// #87: the career extraction prompt asked for industry_level but never told
// the model what the rungs were, so it invented free text ("Charge/supervisory
// level") that matched no career_levels row and dropped every position to the
// ladder's fallback rung. Both vocabulary lists must reach the rendered prompt.
func TestCareerExtractionPromptRendersVocabularies(t *testing.T) {
	out, err := RenderCareerExtractionPrompt(CareerExtractionPromptData{
		CareerText:        "twenty years on the wards",
		ProficiencyValues: `"novice", "proficient", "expert"`,
		CareerLevels:      `"staff nurse", "charge nurse", "nurse manager"`,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		`"charge nurse"`,  // a ladder rung the model must choose from
		`"nurse manager"`, // ...not invent around
		`"proficient"`,    // the depth scale, still rendered
		"twenty years on the wards",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered career extraction prompt is missing %q", want)
		}
	}
	if strings.Contains(out, "{{") {
		t.Errorf("rendered prompt still contains an unexpanded template action:\n%s", out)
	}
}

// Every prompt the pipeline names must actually be embedded. Catches a typo'd
// or renamed template at test time instead of at the first generation call.
func TestNamedPromptsExist(t *testing.T) {
	for _, name := range []string{
		jdExtractionPrompt,
		careerExtractPrompt,
		resumeBodyPrompt,
		resumeSummaryPrompt,
	} {
		if templates.Lookup(name) == nil {
			t.Errorf("no embedded template named %q", name)
		}
		if _, err := promptFingerprint(name); err != nil {
			t.Errorf("cannot fingerprint %q: %v", name, err)
		}
	}
}
