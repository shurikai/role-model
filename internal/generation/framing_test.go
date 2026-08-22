package generation

import (
	"strings"
	"testing"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/vocabulary"
)

// A software-shaped ladder, built inline. The point of career_levels is that
// this vocabulary is data, so the tests own their ladder rather than asserting
// whatever a seed or the shipped defaults happen to carry.
//
// senior is the fallback and rank 3 of 6 — deliberately NOT the median rank,
// so a fallback derived from rank instead of read from the flag would pick
// staff and fail these cases rather than passing them by coincidence.
func testLadder() []db.CareerLevel {
	return []db.CareerLevel{
		{Value: "junior", Rank: 1, Aliases: []string{"entry level"}, LengthBudget: "short", FramingGuidance: "work"},
		{Value: "mid", Rank: 2, Aliases: []string{"intermediate"}, LengthBudget: "short", FramingGuidance: "work"},
		{Value: "senior", Rank: 3, LengthBudget: "medium", FramingGuidance: "work", IsFallback: true},
		{Value: "lead", Rank: 4, Aliases: []string{"tech lead"}, LengthBudget: "long", FramingGuidance: "ownership"},
		{Value: "staff", Rank: 5, LengthBudget: "long", FramingGuidance: "ownership"},
		{Value: "principal", Rank: 6, LengthBudget: "long", FramingGuidance: "ownership"},
	}
}

// Length and framing are the two levers seniority drives. They are tested
// together, and they now come from one row, so a future third lever is added
// as a third column rather than bolted on somewhere else.
func TestSeniorityLevers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		seniority   string
		wantValue   string
		wantBudget  string
		wantFraming string
	}{
		{name: "junior", seniority: "junior", wantValue: "junior", wantBudget: "short", wantFraming: "work"},
		{name: "mid", seniority: "mid", wantValue: "mid", wantBudget: "short", wantFraming: "work"},
		{name: "senior", seniority: "senior", wantValue: "senior", wantBudget: "medium", wantFraming: "work"},
		{name: "staff", seniority: "staff", wantValue: "staff", wantBudget: "long", wantFraming: "ownership"},
		{name: "principal", seniority: "principal", wantValue: "principal", wantBudget: "long", wantFraming: "ownership"},
		{name: "lead", seniority: "lead", wantValue: "lead", wantBudget: "long", wantFraming: "ownership"},

		// Case and surrounding whitespace are extraction noise, not a
		// different level.
		{name: "mixed case", seniority: "Staff", wantValue: "staff", wantBudget: "long", wantFraming: "ownership"},
		{name: "padded", seniority: "  lead  ", wantValue: "lead", wantBudget: "long", wantFraming: "ownership"},

		// An alias reaches the rung it names.
		{name: "alias", seniority: "tech lead", wantValue: "lead", wantBudget: "long", wantFraming: "ownership"},

		// The value extraction emits when it cannot tell, and anything else
		// unrecognised, land on the flagged fallback. Neither is evidence of a
		// senior role, which is what a median-rank fallback would have made
		// them: rank 3.5 of this ladder is lead, on the ownership framing.
		{name: "unknown", seniority: "unknown", wantValue: "senior", wantBudget: "medium", wantFraming: "work"},
		{name: "garbage", seniority: "wizard", wantValue: "senior", wantBudget: "medium", wantFraming: "work"},
		{name: "empty", seniority: "", wantValue: "senior", wantBudget: "medium", wantFraming: "work"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := pickCareerLevel(testLadder(), tc.seniority)
			if got == nil {
				t.Fatalf("pickCareerLevel(%q) = nil", tc.seniority)
			}
			if got.Value != tc.wantValue {
				t.Errorf("value = %q, want %q", got.Value, tc.wantValue)
			}
			if got.LengthBudget != tc.wantBudget {
				t.Errorf("length budget = %q, want %q", got.LengthBudget, tc.wantBudget)
			}
			if got.FramingGuidance != tc.wantFraming {
				t.Errorf("framing = %q, want %q", got.FramingGuidance, tc.wantFraming)
			}
		})
	}
}

// A rung's own name outranks another rung's alias, the same precedence the
// fit-gate matcher keeps between direct and alias matches. Without it, whoever
// wrote the alias silently decides which rung a level resolves to.
func TestCareerLevelValueBeatsAnotherRungsAlias(t *testing.T) {
	ladder := []db.CareerLevel{
		{Value: "principal", Rank: 6, Aliases: []string{"staff"}, IsFallback: true},
		{Value: "staff", Rank: 5},
	}
	got := pickCareerLevel(ladder, "staff")
	if got == nil || got.Value != "staff" {
		t.Fatalf("pickCareerLevel = %v, want the rung named staff, not the one aliasing it", got)
	}
}

// A ladder with no fallback row must report the miss rather than inventing a
// rung, so the caller can fall back to the shipped set instead of handing 2a
// an empty <length_budget>.
func TestCareerLevelMissWithoutFallback(t *testing.T) {
	ladder := []db.CareerLevel{{Value: "staff", Rank: 5}}
	if got := pickCareerLevel(ladder, "wizard"); got != nil {
		t.Errorf("pickCareerLevel = %v, want nil when nothing matches and no row is flagged", got)
	}
	if got := pickCareerLevel(nil, "staff"); got != nil {
		t.Errorf("pickCareerLevel over an empty ladder = %v, want nil", got)
	}
}

// The safety net behind seniorityLevers. Both prompt sections it fills are
// referenced by name from 2a's rules, so an empty one leaves two rules
// pointing at nothing.
func TestDefaultCareerLevelRowsAlwaysAnswer(t *testing.T) {
	rows := defaultCareerLevelRows()
	if len(rows) == 0 {
		t.Fatal("shipped ladder is empty")
	}
	for _, seniority := range []string{"", "unknown", "wizard", "staff", "entry"} {
		got := pickCareerLevel(rows, seniority)
		if got == nil {
			t.Fatalf("shipped ladder returned nil for %q", seniority)
		}
		if strings.TrimSpace(got.LengthBudget) == "" {
			t.Errorf("shipped rung %q has an empty length budget", got.Value)
		}
		if strings.TrimSpace(got.FramingGuidance) == "" {
			t.Errorf("shipped rung %q has an empty framing guidance", got.Value)
		}
	}
}

// The shipped ladder is what a brand-new account starts on, so its shape is
// the friction guard: exactly one fallback, distinct ranks, and no software
// vocabulary as a rung's own name.
func TestShippedLadderShape(t *testing.T) {
	levels := vocabulary.DefaultCareerLevels()

	fallbacks := 0
	ranks := map[int32]bool{}
	for _, l := range levels {
		if l.IsFallback {
			fallbacks++
		}
		if ranks[l.Rank] {
			t.Errorf("rank %d appears twice", l.Rank)
		}
		ranks[l.Rank] = true
	}
	if fallbacks != 1 {
		t.Errorf("shipped ladder has %d fallback rungs, want exactly 1", fallbacks)
	}

	// A default set is harder to notice and harder to remove than a Go switch
	// was, because it lands in the user's own data. These are the rungs that
	// would tell a chef their ladder had been chosen for them.
	for _, l := range levels {
		for _, banned := range []string{"staff", "principal", "ic"} {
			if strings.EqualFold(l.Value, banned) {
				t.Errorf("shipped ladder names a rung %q; the default set must stay field-neutral", l.Value)
			}
		}
	}
}

// The ownership framing exists to add scope ON TOP of the evidence. Losing
// either half of that instruction turns it into the failure mode it was
// written to prevent: broad claims with nothing behind them.
func TestOwnershipFramingKeepsBothHalves(t *testing.T) {
	var ownership string
	for _, l := range vocabulary.DefaultCareerLevels() {
		if l.Rank == 3 {
			ownership = l.FramingGuidance
		}
	}
	if ownership == "" {
		t.Fatal("no top rung on the shipped ladder")
	}
	for _, want := range []string{
		"NEVER trade the evidence for the framing",
		"NEVER manufacture scope",
	} {
		if !strings.Contains(ownership, want) {
			t.Errorf("top-rung framing no longer contains %q", want)
		}
	}
}
