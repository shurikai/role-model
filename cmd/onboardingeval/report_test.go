package main

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shurikai/role-model/internal/db"
)

func pd(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return pgtype.Date{Time: t, Valid: true}
}

func TestDiffPositionDates(t *testing.T) {
	cases := []struct {
		name                 string
		knownStart, knownEnd string
		mineStart, mineEnd   string
		want                 int
	}{
		// The interview stores "2019-03" as the 1st; that is a match.
		{"same month, different day", "2019-03-15", "2022-06-30", "2019-03-01", "2022-06-01", 0},
		{"both current", "2019-03-15", "", "2019-03-01", "", 0},
		{"wrong start", "2019-03-15", "", "2018-03-01", "", 1},
		// #161: an end stored as null against a known end is a finding, not a
		// match -- null claims the job is current.
		{"null end against a known end", "2019-03-15", "2022-06-30", "2019-03-01", "", 1},
		{"end against a known current job", "2019-03-15", "", "2019-03-01", "2022-06-01", 1},
		{"both wrong", "2019-03-15", "2022-06-30", "1900-01-01", "", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			known := db.Position{Title: "RN", StartedOn: pd(c.knownStart), EndedOn: pd(c.knownEnd)}
			mine := db.Position{Title: "RN", StartedOn: pd(c.mineStart), EndedOn: pd(c.mineEnd)}
			if got := diffPositionDates(known, mine); len(got) != c.want {
				t.Errorf("got %d findings %q, want %d", len(got), got, c.want)
			}
		})
	}
}
