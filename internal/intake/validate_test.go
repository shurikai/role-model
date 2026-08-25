package intake

import (
	"encoding/json"
	"strings"
	"testing"
)

// ValidatePayload is what stands between an edited draft and a resolve failure
// nobody can act on. These cases are the ones a person actually produces with
// an editor open: a field emptied, a date retyped by hand, a key renamed.
func TestValidatePayload(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		payload string
		wantErr string // substring; empty means the payload must be accepted
	}{
		{
			name:    "employer round-trips",
			kind:    KindEmployer,
			payload: `{"name":"Nimbus Health","industry":"healthcare","notes":null}`,
		},
		{
			name:    "employer without a name is refused",
			kind:    KindEmployer,
			payload: `{"name":"   ","industry":null,"notes":null}`,
			wantErr: "employer name is required",
		},
		{
			// The failure this exists for: a field the payload does not have
			// is a typo, and dropping it silently means an edit the person
			// watched save is not there.
			name:    "an unknown field is refused rather than dropped",
			kind:    KindEmployer,
			payload: `{"name":"Nimbus Health","industy":"healthcare"}`,
			wantErr: "unknown field",
		},
		{
			name:    "position accepts a month-only start, as extraction writes it",
			kind:    KindPosition,
			payload: `{"employer_draft":null,"title":"Staff Nurse","started_on":"2019-04","ended_on":null,"industry_level":null,"industry_role":null,"location":null,"context_narrative":null}`,
		},
		{
			name:    "position rejects a start date that is not a date",
			kind:    KindPosition,
			payload: `{"title":"Staff Nurse","started_on":"April 2019"}`,
			wantErr: "started_on",
		},
		{
			name:    "position rejects an end date that is not a date",
			kind:    KindPosition,
			payload: `{"title":"Staff Nurse","started_on":"2019-04","ended_on":"still there"}`,
			wantErr: "ended_on",
		},
		{
			name:    "contribution needs both a summary and a description",
			kind:    KindContribution,
			payload: `{"summary":"Ran the floor","full_description":""}`,
			wantErr: "summary and full_description",
		},
		{
			name:    "skill needs a proficiency",
			kind:    KindSkill,
			payload: `{"category":"Clinical","tag":"ACLS","proficiency":"","years_experience":null}`,
			wantErr: "proficiency is required",
		},
		{
			name:    "skill round-trips with a years figure",
			kind:    KindSkill,
			payload: `{"category":"Clinical","tag":"ACLS","proficiency":"expert","years_experience":12.5}`,
		},
		{
			name:    "preference needs a label",
			kind:    KindPreference,
			payload: `{"preference_type":"culture","label":"","sentiment":"positive","weight":5,"is_hard_gate":false,"notes":null,"aliases":[]}`,
			wantErr: "label is required",
		},
		{
			name:    "preference round-trips",
			kind:    KindPreference,
			payload: `{"preference_type":"culture","label":"small team","sentiment":"positive","weight":7,"is_hard_gate":false,"notes":null,"aliases":["lean team"]}`,
		},
		{
			// kind is deliberately not a database CHECK, so an unresolvable
			// kind is a thing that reaches here rather than an impossibility.
			name:    "a kind with no resolver is refused",
			kind:    "publication",
			payload: `{"title":"A paper"}`,
			wantErr: `no resolver for kind "publication"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePayload(tc.kind, json.RawMessage(tc.payload))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected the payload to be accepted, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// The validator and the resolver must not be able to disagree about what a
// usable payload is: a validator that accepts what resolution then refuses is
// worse than no validator, because the refusal arrives after the reviewer has
// finished and moved on. They share one function per kind, and this pins that
// they still do.
func TestResolversUseTheSameValidation(t *testing.T) {
	// Each of these is refused by validate(), and each resolver calls it
	// before touching the database — so these never reach a query.
	if err := (employerPayload{Name: " "}).validate(); err == nil {
		t.Error("employer: expected a blank name to be refused")
	}
	if err := (contributionPayload{Summary: "x"}).validate(); err == nil {
		t.Error("contribution: expected a missing full_description to be refused")
	}
	if err := (skillPayload{Category: "c", Tag: "t"}).validate(); err == nil {
		t.Error("skill: expected a missing proficiency to be refused")
	}
	if err := (preferencePayload{}).validate(); err == nil {
		t.Error("preference: expected a missing label to be refused")
	}
	if err := (positionPayload{StartedOn: "nonsense"}).validate(); err == nil {
		t.Error("position: expected an unparseable started_on to be refused")
	}
}
