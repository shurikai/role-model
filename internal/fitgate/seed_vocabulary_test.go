package fitgate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Guards the *existence* of the category competency vocabulary, which is a
// different question from whether the matcher uses it correctly.
//
// scorer_test.go covers the category layer thoroughly — TestScoreTechnicalFit-
// CompetencyWordedJD, ...PrefersDirectMatchOverCategory, ...DoesNotMatchOn-
// Substrings — and every one of those tests builds its SkillTerms in Go with
// CategoryAliases spelled out inline. That is the right way to test matching
// mechanics, and it is exactly why none of them could catch #74: for nine of
// ten categories the production column was NULL, the third matching layer was
// dead in the live database, and the entire unit suite stayed green because it
// supplied its own vocabulary and never asked where real vocabulary comes from.
//
// Migration 012 introduced tag_categories.aliases and seeded it with
// UPDATE ... WHERE name = '...'. On a fresh database migrations run before
// `make seed`, so those UPDATEs matched zero rows, and the seed that then
// created the categories named only (id, user_id, name, sort_order). Neither
// half was wrong on its own; they simply each assumed the other owned the
// column. Nothing failed, and a Senior Staff posting requiring "APIs" scored it
// as a gap against a profile holding REST at expert.
//
// So this test reads the seed. database/sample is the dataset tracked in this
// repo (database/seed is a separate private checkout and cannot be asserted on
// here), and CLAUDE.md commits it to using deliberately the same nine category
// names as the private set — which makes it the one place a drift like this is
// visible to `make test`.
//
// It asserts the vocabulary is present, not that it is complete. Adding a
// phrase should never fail this; deleting the column's population should.
func TestSampleSeedCarriesCategoryVocabulary(t *testing.T) {
	// The expectations are the DATASET's, read from
	// database/sample/vocabulary.json, not written here. That split is the
	// point: this file used to name nine software categories and their
	// phrases inline, so swapping in a non-software sample dataset failed
	// `make test` on a test that was checking content rather than mechanism.
	// A second career ships its own vocabulary.json and its own expectations
	// instead of breaking this one.
	want := sampleVocabulary(t).CategoryAliases

	block := tagCategoryInsert(t)

	for category, phrases := range want {
		row, ok := seedRowFor(block, category)
		if !ok {
			t.Errorf("category %q has no row in the sample seed's tag_categories insert", category)
			continue
		}
		for _, phrase := range phrases {
			if !strings.Contains(row, "'"+phrase+"'") {
				t.Errorf("category %q is missing alias %q.\n"+
					"The fit gate's category layer is how a capability-worded JD reaches a\n"+
					"technology-worded profile. Without this vocabulary the layer is inert and\n"+
					"every competency phrase scores as a gap — silently, because the matcher\n"+
					"itself still works. See #74.\n\nrow was:\n%s", category, phrase, row)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Structural invariants. Career-neutral, content-free, and true of ANY seed
// dataset — these are what the fit gate's matching mechanism needs to hold,
// as opposed to what this particular career happens to contain.
//
// They deliberately do NOT assert that every category carries vocabulary.
// Languages, Frameworks & Libraries and Cloud & Infrastructure are on NULL on
// purpose: bare "languages", "frameworks" and "cloud" would each grant a whole
// category to any posting that used the word, which is the failure the "an
// alias names a capability, not a tool" rule exists to prevent, seen from the
// other end. A taxonomy built through conversational intake will also
// legitimately start with none, and that belongs in review flags rather than
// in a failing build.
// ---------------------------------------------------------------------------

// An alias on two categories makes the category layer ambiguous: the JD term
// reaches whichever category the skill loop happens to see first, and the
// evidence set changes with row order.
func TestSeedCategoryAliasesAreUnambiguous(t *testing.T) {
	owner := map[string]string{}
	for category, aliases := range parseCategoryAliases(t, tagCategoryInsert(t)) {
		seen := map[string]bool{}
		for _, a := range aliases {
			key := strings.ToLower(strings.TrimSpace(a))
			if seen[key] {
				t.Errorf("category %q lists alias %q twice", category, a)
			}
			seen[key] = true

			if other, ok := owner[key]; ok && other != category {
				t.Errorf("alias %q is on both %q and %q; a category term must name one category",
					a, other, category)
			}
			owner[key] = category
		}
	}
}

// A tag whose alias is another tag's NAME answers job descriptions asking for
// the other one. That is a shadow, not a synonym, and it is invisible from
// either row on its own — which is exactly why it needs a test rather than a
// convention.
func TestSeedTagAliasesDoNotShadowOtherTags(t *testing.T) {
	aliasesByTag := parseTagAliases(t, tagInsert(t))

	names := map[string]string{}
	for tag := range aliasesByTag {
		names[strings.ToLower(tag)] = tag
	}

	for tag, aliases := range aliasesByTag {
		for _, a := range aliases {
			key := strings.ToLower(strings.TrimSpace(a))
			if other, ok := names[key]; ok && other != tag {
				t.Errorf("tag %q carries alias %q, which is the name of tag %q.\n"+
					"A job description asking for %q would be answered by %q. If they really are\n"+
					"the same thing, they should be one tag.", tag, a, other, other, tag)
			}
		}
	}
}

// The whole category layer being empty is the #74 shape: nothing fails, the
// third matching layer is simply inert, and every capability-worded
// requirement scores as a gap. Asserting "at least one" rather than "every
// one" is the difference between a mechanism check and a content check.
func TestSeedCarriesSomeCategoryVocabulary(t *testing.T) {
	total := 0
	for _, aliases := range parseCategoryAliases(t, tagCategoryInsert(t)) {
		total += len(aliases)
	}
	if total == 0 {
		t.Error("no category in the sample seed carries any alias; the fit gate's third " +
			"matching layer is inert and every competency-worded requirement will score as a gap")
	}
}

// Guards the other one-to-many mechanism: the same alias string on several
// tags, which the alias layer accumulates into one match carrying all of them
// as evidence.
//
// This is the only way to express a capability whose evidence spans categories.
// tags.category is a single NOT NULL column, so categories strictly partition
// tags — a category alias cannot reach Java (Languages) and REST (Protocols &
// Messaging) at once, and "backend systems" needs both.
//
// It is guarded because it is invisible from any one row. Nothing about Java's
// alias list says it participates in a shared term, so deleting the term from
// one tag silently shrinks the evidence set instead of breaking anything. The
// same property is why a competency_terms table is the better long-term model:
// this test is standing in for the constraint the schema does not express.
func TestSampleSeedCarriesCrossCategoryTerms(t *testing.T) {
	block := tagInsert(t)

	// term -> the tags that must all carry it, again owned by the dataset.
	want := sampleVocabulary(t).CrossCategoryTerms

	for term, tags := range want {
		for _, tag := range tags {
			row, ok := seedTagRow(block, tag)
			if !ok {
				t.Errorf("tag %q has no row in the sample seed", tag)
				continue
			}
			if !strings.Contains(strings.ToLower(row), "'"+strings.ToLower(term)+"'") {
				t.Errorf("tag %q is missing the shared term %q.\n"+
					"A term spanning categories is expressed by putting the same alias on\n"+
					"every tag that answers it; the alias layer accumulates them into one\n"+
					"match. Dropping it from one tag does not fail anything — it just\n"+
					"quietly removes that skill from the evidence. See #74.\n\nrow was:\n%s",
					tag, term, row)
			}
		}
	}
}

// tagCategoryInsert returns the text of the sample seed's tag_categories
// INSERT, from the statement to its terminating ON CONFLICT.
func tagCategoryInsert(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "database", "sample", "001_foundation.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sql := string(b)

	start := strings.Index(sql, "INSERT INTO tag_categories")
	if start < 0 {
		t.Fatalf("%s has no tag_categories insert", path)
	}
	rest := sql[start:]
	end := strings.Index(rest, "ON CONFLICT")
	if end < 0 {
		t.Fatalf("%s: tag_categories insert has no ON CONFLICT terminator", path)
	}
	return rest[:end]
}

// seedRowFor returns the text of the one VALUES tuple naming category. Rows are
// delimited by the seed's stable UUID prefix, which is what makes this a split
// rather than a parse: every row begins with one and no alias contains one.
func seedRowFor(block, category string) (string, bool) {
	const rowPrefix = "('5c000000-"

	for _, row := range strings.Split(block, rowPrefix)[1:] {
		if strings.Contains(row, "'"+category+"'") {
			return row, true
		}
	}
	return "", false
}

// tagInsert returns the text of the sample seed's tags INSERT.
func tagInsert(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "database", "sample", "001_foundation.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sql := string(b)

	start := strings.Index(sql, "INSERT INTO tags")
	if start < 0 {
		t.Fatalf("%s has no tags insert", path)
	}
	rest := sql[start:]
	end := strings.Index(rest, "ON CONFLICT")
	if end < 0 {
		t.Fatalf("%s: tags insert has no ON CONFLICT terminator", path)
	}
	return rest[:end]
}

// seedTagRow returns the text of the one VALUES tuple naming tag. Rows are
// delimited by the seed's stable tag UUID prefix; the name is matched with its
// trailing comma so 'REST' does not also match a row for 'RESTful'.
func seedTagRow(block, tag string) (string, bool) {
	const rowPrefix = "('57000000-"

	for _, row := range strings.Split(block, rowPrefix)[1:] {
		if strings.Contains(row, "'"+tag+"',") {
			return row, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Dataset-owned expectations, and the parsing the structural tests need.
// ---------------------------------------------------------------------------

// sampleVocabulary is the content contract database/sample states about itself.
// It lives with the dataset so a second career ships its own rather than
// breaking this one — the split this file's structural half exists to make.
type sampleVocabularyFile struct {
	CategoryAliases    map[string][]string `json:"category_aliases"`
	CrossCategoryTerms map[string][]string `json:"cross_category_terms"`
}

func sampleVocabulary(t *testing.T) sampleVocabularyFile {
	t.Helper()
	path := filepath.Join("..", "..", "database", "sample", "vocabulary.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var v sampleVocabularyFile
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(v.CategoryAliases) == 0 {
		t.Fatalf("%s states no category expectations", path)
	}
	return v
}

// seedRow matches one VALUES tuple of the shape both the tag_categories and
// tags inserts use — (id, user_id, name, aliases, ...) — capturing the name and
// the aliases literal.
//
// Splitting on the UUID prefix, which the content helpers above do, cannot be
// reused here: it cuts each row in the middle of a quoted literal, so every
// subsequent quote pairs with the wrong partner and the "names" come out as
// the separators between fields. That produced a parser which returned one
// entry keyed ", " and three tests that passed against a seed with a
// deliberately duplicated alias in it. Anchoring on the whole tuple is what
// makes the quote pairing unambiguous.
var seedRow = regexp.MustCompile(`\('[0-9a-fA-F-]{36}',\s*'[0-9a-fA-F-]{36}',\s*'([^']*)',\s*(ARRAY\[[^\]]*\]|NULL)`)

var quotedValue = regexp.MustCompile(`'([^']*)'`)

// parseSeedAliases maps each seeded row's name to its alias array. A NULL
// column yields nil — a row that deliberately carries no vocabulary, not one
// missing it.
func parseSeedAliases(t *testing.T, block, what string) map[string][]string {
	t.Helper()

	out := map[string][]string{}
	for _, m := range seedRow.FindAllStringSubmatch(block, -1) {
		name, aliases := m[1], m[2]
		if aliases == "NULL" {
			out[name] = nil
			continue
		}
		var list []string
		for _, q := range quotedValue.FindAllStringSubmatch(aliases, -1) {
			list = append(list, q[1])
		}
		out[name] = list
	}
	if len(out) == 0 {
		t.Fatalf("parsed no %s out of the sample seed", what)
	}
	return out
}

func parseCategoryAliases(t *testing.T, block string) map[string][]string {
	t.Helper()
	return parseSeedAliases(t, block, "categories")
}

func parseTagAliases(t *testing.T, block string) map[string][]string {
	t.Helper()
	return parseSeedAliases(t, block, "tags")
}
