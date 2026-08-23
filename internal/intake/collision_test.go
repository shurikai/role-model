package intake

import (
	"strings"
	"testing"
)

// The collision migration 015 records, and the reason this check exists at all.
//
// "product over platform / internal tooling" (positive) and
// "platform / internal tooling over product" (negative) tokenize to the same
// bag of words. matchesSignal compares token runs and cannot tell "X over Y"
// from "Y over X", so both matched the same posting and the profile earned and
// conflicted on one signal at once.
//
// A person writes preference labels a handful at a time and notices this. An
// import proposing twenty in one pass will not, and nothing downstream
// complains — the rows simply both fire, quietly.
func TestCheckPreferenceLabelCatchesTheSeededCollision(t *testing.T) {
	existing := []string{"product over platform / internal tooling"}
	got := CheckPreferenceLabel("platform / internal tooling over product", existing)

	if len(got) == 0 {
		t.Fatal("the migration-015 collision was not reported")
	}
	if !strings.Contains(got[0].Reason, "different order") {
		t.Errorf("reason = %q, want the reordering explanation", got[0].Reason)
	}
}

func TestCheckPreferenceLabel(t *testing.T) {
	existing := []string{
		"remote-first",
		"defense / aerospace",
		"small team, high ownership",
	}

	for _, tt := range []struct {
		name       string
		label      string
		wantHit    bool
		wantReason string
	}{
		{
			name: "an unrelated label is clean",
			// The check must not flag everything, or a review queue full of
			// flags is the same as no flags.
			label: "healthcare", wantHit: false,
		},
		{
			name:  "an exact duplicate is reported as one",
			label: "remote-first", wantHit: true, wantReason: "duplicate",
		},
		{
			name:  "case and punctuation do not hide a duplicate",
			label: "Remote First", wantHit: true, wantReason: "duplicate",
		},
		{
			name: "a reordering is reported",
			// The seeded failure, in the shape the checker generalises it to.
			label: "aerospace / defense", wantHit: true, wantReason: "different order",
		},
		{
			name: "a contained label is reported",
			// "small team" fires wherever "small team, high ownership" does.
			// Not always wrong, always worth a look.
			label: "small team", wantHit: true, wantReason: "contains",
		},
		{
			name: "sharing one word is not a collision",
			// "team" alone appears in an existing label, but as one word of
			// three — it does not make both rows fire on the same posting.
			label: "distributed team culture", wantHit: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPreferenceLabel(tt.label, existing)
			if tt.wantHit && len(got) == 0 {
				t.Fatalf("no collision reported for %q", tt.label)
			}
			if !tt.wantHit && len(got) > 0 {
				t.Fatalf("false positive for %q: %v", tt.label, got)
			}
			if tt.wantHit && !strings.Contains(got[0].Reason, tt.wantReason) {
				t.Errorf("reason = %q, want it to mention %q", got[0].Reason, tt.wantReason)
			}
		})
	}
}

// An empty or punctuation-only label has nothing to compare, and must not
// report a collision with everything.
func TestCheckPreferenceLabelIgnoresEmptyLabels(t *testing.T) {
	for _, label := range []string{"", "   ", "///", "--"} {
		if got := CheckPreferenceLabel(label, []string{"remote-first"}); len(got) > 0 {
			t.Errorf("empty label %q reported collisions: %v", label, got)
		}
	}
}
