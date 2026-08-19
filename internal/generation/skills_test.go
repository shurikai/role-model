package generation

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// years_experience is numeric(4,1) and the prompt prints these verbatim
// ("Java (25 yrs)"), so a scaling or precision error here lands as a false
// claim on a rendered resume rather than as a visible failure.
func TestSkillYears(t *testing.T) {
	for _, tc := range []struct {
		name  string
		raw   string // empty means SQL NULL
		want  float64
		isNil bool
	}{
		{name: "whole years", raw: "25.0", want: 25},
		{name: "half year", raw: "3.5", want: 3.5},
		{name: "sub-year", raw: "0.3", want: 0.3},
		{name: "max scale", raw: "999.9", want: 999.9},
		{name: "unrecorded", raw: "", isNil: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var n pgtype.Numeric
			if tc.raw != "" {
				if err := n.Scan(tc.raw); err != nil {
					t.Fatalf("scan %q: %v", tc.raw, err)
				}
			}

			got, err := skillYears(n)
			if err != nil {
				t.Fatalf("skillYears: %v", err)
			}

			if tc.isNil {
				if got != nil {
					t.Fatalf("got %v, want nil — an unrecorded duration must stay absent, not become 0", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want a value")
			}
			if *got != tc.want {
				t.Errorf("got %v, want %v", *got, tc.want)
			}
		})
	}
}
