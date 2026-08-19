package generation

import (
	"encoding/json"
	"strings"
	"testing"
)

// Length and framing are the two levers seniority drives. They are tested
// together so a future third lever is added to a table rather than bolted on.
func TestSeniorityLevers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		seniority   string
		wantBudget  string
		wantFraming string
	}{
		{name: "junior", seniority: "junior", wantBudget: "Target 1 page", wantFraming: framingDefault},
		{name: "mid", seniority: "mid", wantBudget: "Target 1 page", wantFraming: framingDefault},
		{name: "senior", seniority: "senior", wantBudget: "Target 1-2 pages", wantFraming: framingDefault},
		{name: "staff", seniority: "staff", wantBudget: "Target 2 pages", wantFraming: framingStaff},
		{name: "principal", seniority: "principal", wantBudget: "Target 2 pages", wantFraming: framingStaff},
		{name: "lead", seniority: "lead", wantBudget: "Target 2 pages", wantFraming: framingStaff},
		// A seniority the extractor never emits must not fall into the
		// staff branch by accident — an unrecognized value is not evidence
		// of a senior role.
		{name: "unknown", seniority: "unknown", wantBudget: "Target 1-2 pages", wantFraming: framingDefault},
		{name: "garbage", seniority: "wizard", wantBudget: "Target 1-2 pages", wantFraming: framingDefault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(`{"seniority":"` + tc.seniority + `"}`)

			budget, err := buildLengthBudget(&raw)
			if err != nil {
				t.Fatalf("buildLengthBudget: %v", err)
			}
			if !strings.HasPrefix(budget, tc.wantBudget) {
				t.Errorf("budget = %q, want prefix %q", budget, tc.wantBudget)
			}

			framing, err := buildFramingGuidance(&raw)
			if err != nil {
				t.Fatalf("buildFramingGuidance: %v", err)
			}
			if framing != tc.wantFraming {
				t.Errorf("framing for %q took the wrong branch", tc.seniority)
			}
		})
	}
}

func TestFramingGuidanceNilSignals(t *testing.T) {
	got, err := buildFramingGuidance(nil)
	if err != nil {
		t.Fatalf("buildFramingGuidance: %v", err)
	}
	if got != framingDefault {
		t.Error("absent signals must take the default branch, not the staff one")
	}
}

// The staff guidance exists to add ownership framing on top of the evidence.
// Losing either half of that instruction turns it into the failure mode it
// was written to prevent: broad claims with nothing behind them.
func TestStaffFramingKeepsBothHalves(t *testing.T) {
	for _, want := range []string{
		"NEVER trade the evidence for the framing",
		"NEVER manufacture scope",
	} {
		if !strings.Contains(framingStaff, want) {
			t.Errorf("staff framing guidance no longer contains %q", want)
		}
	}
}
