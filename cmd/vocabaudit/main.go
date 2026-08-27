// Command vocabaudit diffs the canonical career document against the skills
// actually seeded for an account, and prints what each side is missing.
//
// It exists because the only mechanism that has ever surfaced a missing skill
// is coincidence: JFrog Artifactory was flagged as a technical gap by three
// consecutive fit reports before anyone noticed there was no tags/skills row
// behind it, and it was noticed only because a job description happened to name
// it. Every other unseeded skill sits in that same blind spot until some JD
// probes it. That is tolerable while one person can cross-reference from
// memory and stops being tolerable the moment anyone else uses this.
//
// It is deliberately not wired into the server, and is not a scheduled job or a
// CI check. It reads two stores and prints a report for a human to act on; it
// writes nothing.
//
// # What it cannot do
//
// The "documented" half is extracted from free-form prose that was never
// designed to be machine-read, so extraction here is deliberately shallow: it
// takes the bold lead-in terms from the Canonical Facts section, which is a
// real and stable convention in that document, and nothing else. Terms named
// mid-sentence are invisible to it — "real-time Kafka integration via C++
// plugins" names C++ in a bullet whose lead-in is Kafka, and the chi/pgx/sqlc
// stack is listed inside the Role Model bullet. Those need a human reading the
// source doc alongside this output.
//
// Extraction also stops at judgment: a line like "AI tooling: Claude Code,
// GitHub Copilot, Cursor" does not say whether those are skills rows or
// narrative colour, and this tool does not guess. Building a parser good enough
// to settle those cases would be building a worse version of the human pass,
// so the report labels what it found and leaves the rest.
//
// Two sections of the document are read but never treated as claims. Hard
// excludes name technologies to AVOID and recurring honest gaps name
// technologies explicitly LACKED; extracting either as a "documented skill"
// would report Ruby/Rails and Terraform as urgent missing rows. They are
// reported separately, as context.
//
// Usage:
//
//	go run ./cmd/vocabaudit --file path/to/career.md --email you@example.com
//
// --file has no default. The document it reads is a personal career record
// and is not kept in this repository; defaulting to a path that cannot exist
// reports a missing file as though the tool were misconfigured.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
)

func main() {
	var (
		file  = flag.String("file", "", "path to the canonical career document (required)")
		email = flag.String("email", "", "email of the account to audit")
		user  = flag.String("user", "", "user UUID, if you would rather not look it up by email")
	)
	flag.Parse()

	if *file == "" {
		log.Fatal("-file is required: the canonical career document lives outside this repo")
	}
	if *email == "" && *user == "" {
		log.Fatal("one of -email or -user is required")
	}

	// Fail loudly on a missing source file. Reading it as empty would print a
	// clean report with an empty documented list and no gaps at all, which is
	// the most dangerous output this tool could produce: it looks like a pass.
	text, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read canonical document %s: %v\n\nThis file is the audit's only source for what SHOULD exist.\nWithout it there is nothing to diff against, and an empty\n\"documented\" list would read as a clean bill of health.", *file, err)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)

	uid, err := resolveUser(ctx, q, *user, *email)
	if err != nil {
		log.Fatalf("resolve account: %v", err)
	}

	seeded, err := q.ListActiveSkillMatchTermsByUser(ctx, uid)
	if err != nil {
		log.Fatalf("list active skills: %v", err)
	}
	if len(seeded) == 0 {
		log.Fatalf("account %s has no active skills; refusing to report every documented term as a gap", uid)
	}

	doc := parseDocument(string(text))
	report(os.Stdout, *file, uid, doc, seeded)
}

func resolveUser(ctx context.Context, q *db.Queries, userID, email string) (uuid.UUID, error) {
	if userID != "" {
		return uuid.Parse(userID)
	}
	u, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, fmt.Errorf("no account for %s: %w", email, err)
	}
	return u.ID, nil
}

// document is what a single pass over the canonical file yields.
type document struct {
	// claims are the bold lead-in terms of the Canonical Facts bullets.
	claims []string
	// avoided are named under hard excludes or recurring honest gaps. They are
	// named in the document but are the opposite of a claim, and reporting them
	// as missing skills would be actively wrong.
	avoided []string
	// body is the whole file, lowercased, for the reverse direction.
	body string
}

var (
	sectionRE = regexp.MustCompile(`(?m)^##\s+(.*)$`)
	// A Canonical Facts bullet leads with its subject in bold: "- **Kafka**: ...".
	// Some carry no bold lead-in at all; those are simply not extracted, which
	// is the shallowness the package comment describes.
	leadInRE = regexp.MustCompile(`(?m)^\s*[-*]\s+\*\*([^*]+)\*\*`)
	// Hard excludes and honest gaps are written as flat comma/slash lists.
	splitRE = regexp.MustCompile(`\s*(?:,|/| and )\s*`)
)

// biographical lead-ins are Canonical Facts bullets that carry contact and
// history detail rather than a capability. They are structurally incapable of
// being skills, so matching them against the skills table reports every one as
// an urgent gap forever. Six false entries in a thirteen-line urgent list is
// how a report stops being read, which costs more than the handful of terms
// this drops.
//
// It is a stop-list rather than a classifier because the distinction is not
// inferable from the text -- "Location" and "Kafka" are the same shape -- and
// guessing would trade a visible false positive for an invisible false
// negative.
var biographical = map[string]bool{
	"employment dates": true,
	"education":        true,
	"location":         true,
	"github":           true,
	"blog":             true,
	"linkedin":         true,
}

func parseDocument(text string) document {
	d := document{body: strings.ToLower(text)}

	for name, body := range sections(text) {
		lower := strings.ToLower(name)
		switch {
		case strings.Contains(lower, "canonical facts"):
			for _, m := range leadInRE.FindAllStringSubmatch(body, -1) {
				lead := strings.TrimSpace(m[1])
				if biographical[strings.ToLower(lead)] {
					continue
				}
				// A lead-in can name several things at once -- "C# / .NET",
				// "Docker/Jenkins", "DIS/dead-reckoning". Checking the whole
				// string reports all three as gaps while every component is
				// seeded, so each part is checked on its own.
				for _, part := range splitRE.Split(lead, -1) {
					if part = strings.TrimSpace(part); part != "" {
						d.claims = append(d.claims, part)
					}
				}
			}
		case strings.Contains(lower, "job search targeting"):
			d.avoided = append(d.avoided, avoidedTerms(body)...)
		}
	}

	sort.Strings(d.claims)
	sort.Strings(d.avoided)
	return d
}

// sections splits the file on "## " headings.
func sections(text string) map[string]string {
	out := map[string]string{}
	idx := sectionRE.FindAllStringSubmatchIndex(text, -1)
	for i, m := range idx {
		name := text[m[2]:m[3]]
		end := len(text)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out[name] = text[m[1]:end]
	}
	return out
}

// avoidedTerms pulls the hard-exclude bullets and the honest-gaps line. These
// are reported as context, never as gaps.
func avoidedTerms(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "- ") {
			out = append(out, strings.TrimPrefix(t, "- "))
			continue
		}
		if i := strings.Index(strings.ToLower(t), "honest gaps"); i >= 0 {
			if j := strings.Index(t, ":"); j >= 0 {
				for _, term := range splitRE.Split(t[j+1:], -1) {
					if term = strings.Trim(term, " .*"); term != "" {
						out = append(out, term)
					}
				}
			}
		}
	}
	return out
}

// containsWord reports whether term appears in hay on whole-word boundaries.
//
// Substring matching is wrong here for exactly the reason it is wrong in
// internal/fitgate: it finds "REST" inside "interest" and "Git" inside
// "digital", which during this audit produced false "documented" hits for both.
// The boundary test is "not alphanumeric" rather than a regexp \b, because real
// skill names carry punctuation that \b breaks on -- C++, C#, .NET, CI/CD.
func containsWord(term, hay string) bool {
	term, hay = strings.ToLower(strings.TrimSpace(term)), strings.ToLower(hay)
	if term == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(hay[i:], term)
		if j < 0 {
			return false
		}
		j += i
		before := j == 0 || !isAlnum(rune(hay[j-1]))
		end := j + len(term)
		after := end == len(hay) || !isAlnum(rune(hay[end]))
		if before && after {
			return true
		}
		i = j + 1
	}
}

func isAlnum(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

func report(w *os.File, path string, uid uuid.UUID, doc document, seeded []db.ListActiveSkillMatchTermsByUserRow) {
	fmt.Fprintf(w, "Canonical vocabulary audit\n")
	fmt.Fprintf(w, "  source doc:    %s\n", path)
	fmt.Fprintf(w, "  account:       %s\n", uid)
	fmt.Fprintf(w, "  active skills: %d\n", len(seeded))
	fmt.Fprintf(w, "  extracted:     %d claim(s) from Canonical Facts\n\n", len(doc.claims))

	// Direction 1: documented, not seeded. A claim counts as seeded if it
	// matches a tag name OR any of that tag's aliases -- checking the name
	// alone would report "Golang" as a gap against a stored "Go".
	fmt.Fprintf(w, "== DOCUMENTED BUT NOT SEEDED (urgent -- but review each) ==\n")
	fmt.Fprintf(w, "   Lead-ins also cover project names and capabilities, which are\n")
	fmt.Fprintf(w, "   not necessarily skills rows. Confirm before seeding any of these.\n\n")
	var gaps []string
	for _, c := range doc.claims {
		found := false
		for _, s := range seeded {
			if containsWord(c, s.Name) || matchesAny(c, s.Aliases) {
				found = true
				break
			}
		}
		if !found {
			gaps = append(gaps, c)
		}
	}
	if len(gaps) == 0 {
		fmt.Fprintf(w, "  (none)\n")
	}
	for _, g := range gaps {
		fmt.Fprintf(w, "  %s\n      Canonical Facts -- check the source bullet for an employer\n", g)
	}

	// Direction 2: seeded, not documented. Deliberately reported second and
	// labelled: a summary document is not meant to enumerate every skill, so a
	// long list here is the expected shape rather than a finding.
	fmt.Fprintf(w, "\n== SEEDED BUT NOT DOCUMENTED (informational, lower priority) ==\n")
	fmt.Fprintf(w, "   Often legitimate: the canonical doc is a per-thread summary,\n")
	fmt.Fprintf(w, "   not an inventory. Worth skimming for stale or test rows only.\n\n")

	byCategory := map[string][]string{}
	for _, s := range seeded {
		if containsWord(s.Name, doc.body) || matchesAnyIn(s.Aliases, doc.body) {
			continue
		}
		byCategory[s.Category] = append(byCategory[s.Category], s.Name)
	}
	if len(byCategory) == 0 {
		fmt.Fprintf(w, "  (none)\n")
	}
	for _, cat := range sortedKeys(byCategory) {
		names := byCategory[cat]
		sort.Strings(names)
		fmt.Fprintf(w, "  %-24s %s\n", cat, strings.Join(names, ", "))
	}

	// Context, never gaps.
	fmt.Fprintf(w, "\n== NAMED IN THE DOC AS AVOIDED OR LACKING (not gaps) ==\n")
	fmt.Fprintf(w, "   Hard excludes and honest gaps. Listed so they are not mistaken\n")
	fmt.Fprintf(w, "   for missing skills. A seeded hit here is worth a look: it may be\n")
	fmt.Fprintf(w, "   a genuine low-proficiency row, which agrees with the doc.\n\n")
	for _, a := range doc.avoided {
		note := ""
		for _, s := range seeded {
			if containsWord(s.Name, a) {
				note = fmt.Sprintf("   <- seeded: %s (%s)", s.Name, s.Proficiency)
				break
			}
		}
		fmt.Fprintf(w, "  %s%s\n", a, note)
	}

	fmt.Fprintf(w, "\nExtraction is shallow by design -- terms named mid-sentence are not\n")
	fmt.Fprintf(w, "picked up. Read %s alongside this output.\n", path)
}

func matchesAny(term string, aliases []string) bool {
	for _, a := range aliases {
		if containsWord(term, a) {
			return true
		}
	}
	return false
}

func matchesAnyIn(aliases []string, hay string) bool {
	for _, a := range aliases {
		// Short aliases are skipped: a two-character alias matches far too
		// much ordinary prose to be evidence that a skill was documented.
		if len(strings.TrimSpace(a)) >= 3 && containsWord(a, hay) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
