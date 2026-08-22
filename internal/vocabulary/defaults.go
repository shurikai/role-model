// Package vocabulary holds the level vocabularies a brand-new account starts
// with, and installs them when the account is created.
//
// It exists because of the friction guard on the career-neutrality work: a
// user must never be asked to author a seniority ladder before they can do
// anything. Nothing in the pipeline reads this package at runtime -- the
// length budget, the framing guidance, the extraction enum, and the depth
// scale are all read back out of the database. These are starting rows, not a
// lookup table, which is the whole difference between this and the six
// hardcoded copies it replaced.
//
// The default set is deliberately NEUTRAL rather than the software ladder.
// Seeding every new account with junior/mid/staff/principal would put one
// industry's vocabulary into a chef's data, where it is harder to notice and
// harder to remove than it was in a Go switch. Three bands cover any working
// life; import proposes the real ladder later (source 'inferred') and the user
// confirms it.
package vocabulary

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// Length budgets. Soft guidance for the 2a prompt, which allocates bullets by
// relevance against a target rather than emitting everything it is given.
const (
	budgetShort  = "Target 1 page (~8-10 bullets total across ALL positions and projects combined)."
	budgetMedium = "Target 1-2 pages (~12-15 bullets total across ALL positions and projects combined)."
	budgetLong   = "Target 2 pages (~15-18 bullets total across ALL positions and projects combined)."
)

// Framing guidance: the altitude bullets are written at, and the second of the
// two levers a level drives. Length was the first and for a long time the only
// one, which meant a senior posting got more bullets of the same altitude
// rather than bullets pitched at the level it was hiring for.
//
// The rule these strings enforce is the one that is easy to get backwards:
// ownership framing is added ON TOP of the evidence, never in place of it. The
// metric is what makes the ownership claim believable, and a broad claim with
// nothing behind it is the shape a skeptical reader discounts.
const (
	framingOwnership = `This role is pitched at the top of the ladder. A reader at this level is
scanning for scope and accountability, not implementation detail alone.

On the 2-3 most JD-relevant positions, open each bullet with what was owned,
decided, or changed at the system or team level, then land the supporting
evidence — the metric, the scale, the team size — in the same sentence.

  Weaker: "Rebuilt the referral intake process, cutting average wait from
    three weeks to four days."
  Stronger: "Owned referral intake across a six-site region, cutting average
    wait from three weeks to four days by rebuilding how referrals were
    routed and triaged."

Both sentences carry the same fact. The second also says what the candidate
was responsible for. This holds in every field; nothing about it is specific
to any one kind of work.

Two hard limits on this:
  - NEVER trade the evidence for the framing. A bullet that claims ownership
    and drops the number is weaker than one that only reports the number.
    Both together, or the number alone — never the claim alone.
  - NEVER manufacture scope the source material does not support. "Owned",
    "led", "set direction for" are factual claims and need backing in the
    contribution data like any other. If the data shows the work but not the
    ownership, write the work.`

	framingWork = `This role is pitched below the top of the ladder. Lead with the concrete work
and the outcome it produced.

Where the source material genuinely supports ownership or leadership scope,
say so — but do not reach for it. A bullet claiming ownership or scope the
contribution data does not support reads as padding, and costs more
credibility than the framing gains.`
)

// CareerLevel is one rung of a starting ladder, in the shape career_levels
// stores.
type CareerLevel struct {
	Value           string
	Label           string
	Rank            int32
	Aliases         []string
	LengthBudget    string
	FramingGuidance string
	IsFallback      bool
}

// ProficiencyLevel is one band of a starting depth scale.
type ProficiencyLevel struct {
	Value   string
	Label   string
	Rank    int32
	Aliases []string
}

// DefaultCareerLevels returns the neutral three-band ladder.
//
// "Experienced" is the fallback rather than the median rung, and the
// distinction matters more on a longer ladder: falling back to whatever sits
// in the middle reads as the safe choice and is not, because on a ten-rung
// ladder the middle is staff. A posting whose seniority could not be read is
// not evidence of a senior role.
func DefaultCareerLevels() []CareerLevel {
	return []CareerLevel{
		{
			Value:           "entry",
			Label:           "Entry",
			Rank:            1,
			Aliases:         []string{"junior", "entry level", "entry-level", "associate", "assistant", "apprentice", "trainee"},
			LengthBudget:    budgetShort,
			FramingGuidance: framingWork,
		},
		{
			Value:           "experienced",
			Label:           "Experienced",
			Rank:            2,
			Aliases:         []string{"mid", "mid level", "mid-level", "intermediate", "journeyman"},
			LengthBudget:    budgetMedium,
			FramingGuidance: framingWork,
			IsFallback:      true,
		},
		{
			Value:           "senior",
			Label:           "Senior",
			Rank:            3,
			Aliases:         []string{"lead", "principal", "staff", "head", "chief", "supervising"},
			LengthBudget:    budgetLong,
			FramingGuidance: framingOwnership,
		},
	}
}

// DefaultProficiencyLevels returns the neutral three-band depth scale.
//
// The aliases carry the phrasings a posting uses for depth, because that is
// the side this scale is compared against: extraction reads "5+ years of Go"
// or "exposure to Kafka" off a JD and records a level for the requirement.
func DefaultProficiencyLevels() []ProficiencyLevel {
	return []ProficiencyLevel{
		{Value: "novice", Label: "Novice", Rank: 1, Aliases: []string{"beginner", "familiarity", "exposure", "awareness"}},
		{Value: "proficient", Label: "Proficient", Rank: 2, Aliases: []string{"working knowledge", "solid", "competent"}},
		{Value: "expert", Label: "Expert", Rank: 3, Aliases: []string{"deep expertise", "advanced", "mastery", "authority"}},
	}
}

// Install writes the default vocabularies for a newly created user.
//
// Called on the signup path. An account with no rows is not broken -- the
// level lookup degrades to unranked guidance and the depth scale ranks
// everything at zero -- but it produces a resume written at no particular
// altitude, which is a worse failure for being quiet.
func Install(ctx context.Context, q *db.Queries, userID uuid.UUID) error {
	for _, l := range DefaultCareerLevels() {
		_, err := q.CreateCareerLevel(ctx, db.CreateCareerLevelParams{
			ID:              uuid.New(),
			UserID:          userID,
			Value:           l.Value,
			Label:           l.Label,
			Rank:            l.Rank,
			Aliases:         l.Aliases,
			LengthBudget:    l.LengthBudget,
			FramingGuidance: l.FramingGuidance,
			IsFallback:      l.IsFallback,
			Source:          "default",
			SortOrder:       l.Rank,
		})
		if err != nil {
			return fmt.Errorf("install career level %q: %w", l.Value, err)
		}
	}

	for _, p := range DefaultProficiencyLevels() {
		_, err := q.CreateProficiencyLevel(ctx, db.CreateProficiencyLevelParams{
			ID:        uuid.New(),
			UserID:    userID,
			Value:     p.Value,
			Label:     p.Label,
			Rank:      p.Rank,
			Aliases:   p.Aliases,
			Source:    "default",
			SortOrder: p.Rank,
		})
		if err != nil {
			return fmt.Errorf("install proficiency level %q: %w", p.Value, err)
		}
	}

	return nil
}
