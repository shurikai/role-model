package vocabulary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The level vocabularies are user-owned rows now, which means they have three
// separate creators and no single one of them covers every account: migration
// 020 backfills accounts that already existed, Install seeds accounts created
// through signup, and the sample seed creates its own user directly in SQL and
// is reached by neither.
//
// That third case is the one that goes wrong quietly. A sample user with no
// career_levels rows still generates a resume — the lookup falls through to the
// shipped neutral ladder — so nothing errors, and the seeded ladder's absence
// shows up only as a resume written at the wrong altitude. It is the same
// shape as #74, where migration 012 populated a column for rows the seed had
// not created yet and nine of ten categories sat on NULL with every unit test
// green.
//
// So this reads the seed file, the way internal/fitgate's vocabulary tests do.
// database/sample is the dataset tracked in this repo; database/seed is a
// separate private checkout and cannot be asserted on here.
func sampleFoundation(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "database", "sample", "001_foundation.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestSampleSeedCarriesLevelVocabularies(t *testing.T) {
	seed := sampleFoundation(t)

	for _, table := range []string{"career_levels", "proficiency_levels"} {
		if !strings.Contains(seed, "INSERT INTO "+table) {
			t.Errorf("the sample seed creates a user but no %s rows.\n"+
				"Whoever creates a row populates its columns: neither migration 020's\n"+
				"backfill nor signup reaches a user this file invents, so the ladder has\n"+
				"to be seeded here. An account with none still generates — it just\n"+
				"generates at the wrong altitude, silently.", table)
		}
	}
}

// Exactly one rung carries is_fallback. Zero means an unreadable posting finds
// no rung at all and falls through to the shipped neutral ladder, quietly
// abandoning the seeded one; more than one is rejected by the partial unique
// index, but only once someone runs the seed.
func TestSampleSeedLadderHasExactlyOneFallback(t *testing.T) {
	block := careerLevelsBlock(t, sampleFoundation(t))

	if got := strings.Count(block, "TRUE"); got != 1 {
		t.Errorf("sample ladder flags %d rungs as the fallback, want exactly 1", got)
	}
}

// Both levers a rung drives must be real text. An empty length_budget or
// framing_guidance is not a neutral default: two of 2a's rules refer to those
// sections by name, so an empty one leaves the rules pointing at nothing.
func TestSampleSeedLadderCarriesBothLevers(t *testing.T) {
	block := careerLevelsBlock(t, sampleFoundation(t))

	if !strings.Contains(block, "Target 1 page") ||
		!strings.Contains(block, "Target 2 pages") {
		t.Error("sample ladder does not carry distinct length budgets across its rungs")
	}

	// The ownership framing exists to add scope ON TOP of the evidence.
	// Losing either half turns it into the failure mode it was written to
	// prevent, and the seed carries its own copy of the text.
	for _, want := range []string{
		"NEVER trade the evidence for the framing",
		"NEVER manufacture scope",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("sample ladder's top-rung framing no longer contains %q", want)
		}
	}
}

// careerLevelsBlock returns the seed text from the career_levels statement up
// to the proficiency_levels one. The framing paragraphs sit in a CTE above the
// INSERT, so the block starts at the comment banner rather than the keyword.
func careerLevelsBlock(t *testing.T, seed string) string {
	t.Helper()

	start := strings.Index(seed, "-- Level vocabularies")
	if start == -1 {
		t.Fatal("no level-vocabulary section in the sample seed")
	}
	end := strings.Index(seed, "INSERT INTO proficiency_levels")
	if end == -1 || end < start {
		t.Fatal("no proficiency_levels insert after the career_levels one")
	}
	return seed[start:end]
}
