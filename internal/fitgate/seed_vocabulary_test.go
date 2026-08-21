package fitgate

import (
	"os"
	"path/filepath"
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
	// One representative capability phrase per category the fit gate depends
	// on, chosen because each is a phrasing real postings use in place of
	// naming a technology. "apis" is listed explicitly: it is the one that
	// surfaced #74.
	want := map[string][]string{
		"Databases":             {"data modeling", "schema design", "sql"},
		"Protocols & Messaging": {"apis", "api design", "event-driven systems", "messaging"},
		"Testing":               {"automated testing", "test automation"},
		"Observability":         {"observability", "monitoring", "telemetry"},
		"Methodologies":         {"system design", "architecture", "scalability"},
		"Tools & CI/CD":         {"ci/cd", "continuous integration", "devops"},
	}

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

// A category alias must name a capability, not a technology: aliasing one tool
// onto a category grants the whole category for it. Migration 012 calls this
// out and declines to add 'kafka' to Protocols & Messaging for that reason.
// The seed is now where the vocabulary lives, so the rule is checked here.
func TestSampleSeedCategoryVocabularyNamesNoTechnology(t *testing.T) {
	block := tagCategoryInsert(t)

	// Product names that would each hand over a whole category. Not
	// exhaustive — it is a tripwire for the tempting cases, and the tempting
	// case is always "this one tool is what the category means to me".
	banned := []string{
		"kafka", "jenkins", "splunk", "junit", "postgres", "postgresql",
		"docker", "kubernetes", "terraform", "react", "spring boot",
		"github actions", "dynatrace", "redis", "mysql", "cassandra",
	}
	for _, tech := range banned {
		if strings.Contains(strings.ToLower(block), "'"+tech+"'") {
			t.Errorf("%q appears as a category alias in the sample seed.\n"+
				"A category alias grants every skill in the category as evidence, so a\n"+
				"technology name there means one tool answers a requirement the other\n"+
				"members cannot. Put it in tags.aliases on that tag's own row instead.", tech)
		}
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

	// term -> the tags that must all carry it. The sample has no Microservices
	// tag, so its backend set is the three the dataset does have.
	want := map[string][]string{
		"backend systems":     {"Java", "Distributed Systems", "REST"},
		"backend engineering": {"Java", "Distributed Systems", "REST"},
		"apis":                {"REST"},
	}

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
