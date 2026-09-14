package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shurikai/role-model/internal/db"
)

// clinicalSeedUserID is database/sample-clinical's known account -- see its
// README. Loaded via `make seed-clinical`; the ground-truth comparison below
// degrades gracefully (raw counts only, no diff) when it isn't present in
// this database, since requiring it would make every eval run depend on a
// manual step most people won't remember.
var clinicalSeedUserID = uuid.MustParse("5b000000-0000-0000-0000-000000000001")

// capturedCareer is one account's career data, read directly from Postgres.
// This is the one place this tool touches the database for anything other
// than account setup/teardown -- reading back what the onboarding agent
// wrote, through the same tables cmd/clearhistory and cmd/intakerun already
// read and write directly. The "never bypasses the API" rule in #117 is
// about how the AGENT writes; how this eval tool inspects the outcome
// afterward is a different question entirely.
type capturedCareer struct {
	employers               []db.Employer
	positionsByEmployer     map[uuid.UUID][]db.Position
	contributionsByPosition map[uuid.UUID][]db.Contribution
	tags                    []db.Tag
	skills                  []db.ListSkillsWithTagsByUserRow
}

func loadCareer(ctx context.Context, q *db.Queries, userID uuid.UUID) (*capturedCareer, error) {
	employers, err := q.GetEmployers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list employers: %w", err)
	}

	career := &capturedCareer{
		employers:               employers,
		positionsByEmployer:     map[uuid.UUID][]db.Position{},
		contributionsByPosition: map[uuid.UUID][]db.Contribution{},
	}

	for _, e := range employers {
		positions, err := q.GetPositionsByEmployer(ctx, db.GetPositionsByEmployerParams{EmployerID: e.ID, UserID: userID})
		if err != nil {
			return nil, fmt.Errorf("list positions for %s: %w", e.Name, err)
		}
		career.positionsByEmployer[e.ID] = positions

		for _, p := range positions {
			positionID := p.ID
			contributions, err := q.GetContributionsByPosition(ctx, db.GetContributionsByPositionParams{PositionID: &positionID, UserID: userID})
			if err != nil {
				return nil, fmt.Errorf("list contributions for %s: %w", p.Title, err)
			}
			career.contributionsByPosition[p.ID] = contributions
		}
	}

	tags, err := q.ListTags(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	career.tags = tags

	skills, err := q.ListSkillsWithTagsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	career.skills = skills

	return career, nil
}

// employersByName is a case-insensitive lookup, used both for printing and
// for diffing against the ground-truth account.
func (c *capturedCareer) employersByName() map[string]db.Employer {
	out := make(map[string]db.Employer, len(c.employers))
	for _, e := range c.employers {
		out[strings.ToLower(e.Name)] = e
	}
	return out
}

func (c *capturedCareer) tagNames() map[string]bool {
	out := make(map[string]bool, len(c.tags))
	for _, t := range c.tags {
		out[strings.ToLower(t.Name)] = true
	}
	return out
}

func printReport(ctx context.Context, q *db.Queries, userID uuid.UUID, transcript []qaTurn) error {
	mine, err := loadCareer(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("load captured career: %w", err)
	}

	// The one automated assertion: #117's own acceptance criteria says every
	// skill claim must link to a supporting contribution. Everything else
	// below is for a person to read and judge; this is a correctness check.
	fmt.Println("=== Evidence-link check ===")
	unlinked := 0
	for _, s := range mine.skills {
		contribIDs, err := q.ListContributionsBySkill(ctx, s.ID)
		if err != nil {
			return fmt.Errorf("check evidence for skill %s: %w", s.Name, err)
		}
		if len(contribIDs) == 0 {
			unlinked++
			fmt.Printf("  FAIL  %q has no supporting contribution\n", s.Name)
		}
	}
	switch {
	case len(mine.skills) == 0:
		fmt.Println("  (no skills captured)")
	case unlinked == 0:
		fmt.Printf("  PASS  all %d captured skills trace to at least one contribution\n", len(mine.skills))
	default:
		fmt.Printf("  %d of %d skills have NO supporting contribution -- this violates #117's own acceptance criteria\n", unlinked, len(mine.skills))
	}

	fmt.Println("\n=== Transcript ===")
	if len(transcript) == 0 {
		fmt.Println("  (empty)")
	}
	for i, t := range transcript {
		fmt.Printf("\n%d. Q: %s\n   A: %s\n", i+1, t.Question, t.Answer)
	}

	fmt.Println("\n=== Captured ===")
	printCaptured(mine)

	known, err := loadCareer(ctx, q, clinicalSeedUserID)
	if err != nil {
		return fmt.Errorf("load ground-truth career: %w", err)
	}
	if len(known.employers) == 0 {
		fmt.Println("\n(no ground-truth comparison -- database/sample-clinical isn't loaded in this database; run `make seed-clinical` first for a diff)")
		return nil
	}
	fmt.Println("\n=== Compared against database/sample-clinical's known-correct data ===")
	diffCareer(mine, known)
	return nil
}

func printCaptured(c *capturedCareer) {
	if len(c.employers) == 0 {
		fmt.Println("  (nothing captured)")
		return
	}
	for _, e := range c.employers {
		fmt.Printf("  %s\n", e.Name)
		for _, p := range c.positionsByEmployer[e.ID] {
			fmt.Printf("    - %s (%s)\n", p.Title, formatDate(p.StartedOn))
			for _, contrib := range c.contributionsByPosition[p.ID] {
				fmt.Printf("        * %s\n", contrib.Summary)
			}
		}
	}
	if len(c.tags) > 0 {
		names := make([]string, len(c.tags))
		for i, t := range c.tags {
			names[i] = t.Name
		}
		fmt.Printf("  tags: %s\n", strings.Join(names, ", "))
	}
	for _, s := range c.skills {
		years := "years unstated"
		if s.YearsExperience.Valid {
			if f, err := s.YearsExperience.Float64Value(); err == nil && f.Valid {
				years = fmt.Sprintf("%.1f years", f.Float64)
			}
		}
		fmt.Printf("  skill: %s (%s, %s)\n", s.Name, s.Proficiency, years)
	}
}

// diffCareer compares by name only -- an employer or a tag either matches
// the ground truth's or it doesn't, and case/whitespace shouldn't decide
// that. Contribution CONTENT is deliberately never diffed: free text won't
// match verbatim between an extraction pass and a conversational one, and
// that comparison belongs to the person reading the transcript above, not
// to string equality here.
func diffCareer(mine, known *capturedCareer) {
	mineByName := mine.employersByName()
	knownByName := known.employersByName()

	var matched, missed, extra []string
	for name, e := range knownByName {
		if _, ok := mineByName[name]; ok {
			matched = append(matched, e.Name)
		} else {
			missed = append(missed, e.Name)
		}
	}
	for name, e := range mineByName {
		if _, ok := knownByName[name]; !ok {
			extra = append(extra, e.Name)
		}
	}

	fmt.Printf("  employers: %d/%d known employers captured\n", len(matched), len(knownByName))
	for _, name := range missed {
		fmt.Printf("    missed:  %s\n", name)
	}
	for _, name := range extra {
		fmt.Printf("    EXTRA:   %s (not in the known account -- check this wasn't invented)\n", name)
	}

	for name, knownEmployer := range knownByName {
		mineEmployer, ok := mineByName[name]
		if !ok {
			continue
		}
		knownTitles := positionTitles(known.positionsByEmployer[knownEmployer.ID])
		mineTitles := positionTitles(mine.positionsByEmployer[mineEmployer.ID])
		matchedTitles := 0
		for title := range knownTitles {
			if mineTitles[title] {
				matchedTitles++
			}
		}
		fmt.Printf("  %s: %d/%d known positions captured\n", knownEmployer.Name, matchedTitles, len(knownTitles))
	}

	mineTags := mine.tagNames()
	knownTags := known.tagNames()
	tagMatches := 0
	for name := range knownTags {
		if mineTags[name] {
			tagMatches++
		}
	}
	fmt.Printf("  tags: %d/%d known tags captured\n", tagMatches, len(knownTags))
}

func positionTitles(positions []db.Position) map[string]bool {
	out := make(map[string]bool, len(positions))
	for _, p := range positions {
		out[strings.ToLower(p.Title)] = true
	}
	return out
}

func formatDate(d pgtype.Date) string {
	if !d.Valid {
		return "date unstated"
	}
	return d.Time.Format("2006-01-02")
}
