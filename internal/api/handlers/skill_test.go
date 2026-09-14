package handlers

import "testing"

// No DB dependency here, like onboarding_test.go -- parseYears is a pure
// function, not a fixture for the database.

func TestParseYears(t *testing.T) {
	cases := []struct {
		name    string
		in      *float64
		wantErr bool
	}{
		{"nil stays NULL", nil, false},
		{"zero is valid", floatPtr(0), false},
		{"an ordinary value is valid", floatPtr(5.5), false},
		{"negative is rejected", floatPtr(-1), true},
		{"exactly the ceiling is valid", floatPtr(maxYearsExperience), false},
		{"just over the ceiling is rejected", floatPtr(maxYearsExperience + 0.1), true},
		{
			// Found by cmd/onboardingeval's first real run: the onboarding
			// agent misread "I got my ACLS in 2012" as 2012 years of
			// experience, which is well past what NUMERIC(4,1) can even
			// store. Postgres used to be the one to reject it, as a raw
			// "numeric field overflow" surfaced to the caller as an opaque
			// 500 rather than the validation error it actually is.
			"a calendar year is rejected",
			floatPtr(2012),
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseYears(tc.in)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseYears(%v) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
		})
	}
}

func floatPtr(f float64) *float64 { return &f }
