package stage0

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

func tag(name, category string) db.Tag {
	return db.Tag{ID: uuid.New(), Name: name, Category: category}
}

func TestResolveSuggestedTags(t *testing.T) {
	golang := tag("Go", "Languages")
	kafka := tag("Kafka", "Protocols & Messaging")
	postgres := tag("PostgreSQL", "Databases")
	available := []db.Tag{golang, kafka, postgres}

	tests := []struct {
		name  string
		names []string
		want  []string // expected tag names, in order
	}{
		{"exact match", []string{"Go", "Kafka"}, []string{"Go", "Kafka"}},
		{"case-insensitive", []string{"go", "KAFKA"}, []string{"Go", "Kafka"}},
		{"whitespace trimmed", []string{"  Go  ", "\tKafka\n"}, []string{"Go", "Kafka"}},
		{"unknown dropped", []string{"Go", "Rust", "Kafka"}, []string{"Go", "Kafka"}},
		{"duplicate names deduped", []string{"Go", "Go"}, []string{"Go"}},
		{"same tag via different casing deduped", []string{"Go", "go", "GO"}, []string{"Go"}},
		{"order follows the model", []string{"PostgreSQL", "Go"}, []string{"PostgreSQL", "Go"}},
		{"empty names", nil, []string{}},
		{"all unknown", []string{"Rust", "Elixir"}, []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSuggestedTags(available, tc.names)
			if got == nil {
				t.Fatal("result is nil; want a non-nil slice so it marshals to []")
			}
			gotNames := make([]string, len(got))
			for i, s := range got {
				gotNames[i] = s.Name
			}
			if strings.Join(gotNames, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", gotNames, tc.want)
			}
			for _, s := range got {
				if s.TagID == uuid.Nil {
					t.Errorf("resolved %q carries a nil tag_id", s.Name)
				}
				if s.Category == "" {
					t.Errorf("resolved %q carries an empty category", s.Name)
				}
			}
		})
	}
}

func TestResolveSuggestedTagsEmptyVocabulary(t *testing.T) {
	got := resolveSuggestedTags(nil, []string{"Go", "Kafka"})
	if got == nil {
		t.Fatal("result is nil; want a non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("got %d suggestions from an empty vocabulary, want 0", len(got))
	}
}

func TestResolveSuggestedTagsMarshalsToArray(t *testing.T) {
	b, err := json.Marshal(resolveSuggestedTags(nil, nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Fatalf("got %s, want []", b)
	}
}

func TestEnrichmentInputMarshalsAvailableTags(t *testing.T) {
	summary := "did the thing"
	in := enrichmentInput{
		EmployerName:  "Acme",
		PositionTitle: "Engineer",
		Summary:       &summary,
		AvailableTags: []enrichmentTagOption{
			{Name: "Go", Category: "Languages"},
			{Name: "Kafka", Category: "Protocols & Messaging"},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var round struct {
		AvailableTags []struct {
			Name     string `json:"name"`
			Category string `json:"category"`
		} `json:"available_tags"`
	}
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(round.AvailableTags) != 2 || round.AvailableTags[0].Name != "Go" ||
		round.AvailableTags[1].Category != "Protocols & Messaging" {
		t.Fatalf("available_tags did not round-trip: %s", b)
	}
}

func TestEnrichmentResultParsesSuggestedTags(t *testing.T) {
	raw := `{"flags":[{"type":"gap","field":"outcomes","message":"thin"}],"suggested_tags":["Go","Kafka"]}`
	var got enrichmentResult
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Flags) != 1 {
		t.Fatalf("got %d flags, want 1", len(got.Flags))
	}
	if strings.Join(got.SuggestedTags, ",") != "Go,Kafka" {
		t.Fatalf("got suggested_tags %v", got.SuggestedTags)
	}
}

func TestEnrichmentResultToleratesMissingSuggestedTags(t *testing.T) {
	// A response from before this field existed, or a model that omitted it.
	var got enrichmentResult
	if err := json.Unmarshal([]byte(`{"flags":[]}`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SuggestedTags != nil {
		t.Fatalf("want nil SuggestedTags, got %v", got.SuggestedTags)
	}
	// resolveSuggestedTags must still produce a marshalable [].
	if got := resolveSuggestedTags(nil, got.SuggestedTags); got == nil {
		t.Fatal("resolveSuggestedTags(nil, nil) returned nil")
	}
}
