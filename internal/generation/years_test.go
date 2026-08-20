package generation

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildYearsExperience(t *testing.T) {
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name       string
		experience string
		want       string
	}{
		{
			// The real shape: earliest position starts 1999-06, so the
			// answer is 27 whether the model feels like rounding or not.
			name: "earliest position wins regardless of order",
			experience: `[
				{"positions":[{"started_on":"2025-09"}]},
				{"positions":[{"started_on":"2003-01"},{"started_on":"1999-06"}]},
				{"positions":[{"started_on":"2016-02"}]}
			]`,
			want: "27 years",
		},
		{
			name:       "single position",
			experience: `[{"positions":[{"started_on":"2020-08"}]}]`,
			want:       "6 years",
		},
		{
			// Partial years round down. "6 years of experience" from
			// 6 years 11 months is the honest direction to err.
			name:       "rounds down",
			experience: `[{"positions":[{"started_on":"2019-09"}]}]`,
			want:       "6 years",
		},
		{
			name:       "under a year yields nothing to quote",
			experience: `[{"positions":[{"started_on":"2026-03"}]}]`,
			want:       "",
		},
		{
			// 2a's output has not been schema-validated at the point this
			// runs, so one bad date must not cost the whole figure.
			name:       "unparseable date is skipped, not fatal",
			experience: `[{"positions":[{"started_on":"whenever"},{"started_on":"2010-01"}]}]`,
			want:       "16 years",
		},
		{name: "no usable dates", experience: `[{"positions":[{"started_on":""}]}]`, want: ""},
		{name: "no positions", experience: `[{"positions":[]}]`, want: ""},
		{name: "empty experience", experience: `[]`, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildYearsExperience(json.RawMessage(tc.experience), now)
			if err != nil {
				t.Fatalf("buildYearsExperience: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildYearsExperienceAbsentBlock(t *testing.T) {
	got, err := buildYearsExperience(nil, time.Now())
	if err != nil {
		t.Fatalf("buildYearsExperience: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty — the prompt reads empty as 'do not mention years'", got)
	}
}
