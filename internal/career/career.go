// Package career assembles the read views of a user's career: the whole
// employer → position tree in one response, and a position's contributions
// with their tags. Both are several queries stitched together in Go, which is
// assembly rather than HTTP, so it lives here and not in a handler.
package career

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// Tree is every employer the user has, each carrying its positions.
type Tree struct {
	Employers []Employer `json:"employers"`
}

// Employer is the full employer row, so an edit form can round-trip it.
type Employer struct {
	db.Employer
	Positions []Position `json:"positions"`
}

// Position is the full position row plus how many contributions sit under it.
// Contributions themselves are not inlined: a long career is hundreds of
// them, and they load per position.
type Position struct {
	db.Position
	ContributionCounts ContributionCounts `json:"contribution_counts"`
}

// ContributionCounts splits on is_active. Hidden contributions are kept but
// never reach a generated resume, which is why the view counts them apart.
type ContributionCounts struct {
	Active int32 `json:"active"`
	Hidden int32 `json:"hidden"`
}

// Contribution is a contribution row with the tags linked to it.
type Contribution struct {
	db.Contribution
	Tags []Tag `json:"tags"`
}

type Tag struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Category string    `json:"category"`
}

type Reader struct {
	queries db.Querier
}

func NewReader(queries db.Querier) *Reader {
	return &Reader{queries: queries}
}

// Tree reads the user's career in three queries. Employers arrive in resume
// order (current first) and positions in sort_order then newest start; both
// orders are kept as-is, since grouping positions by employer preserves the
// order they were read in.
func (r *Reader) Tree(ctx context.Context, userID uuid.UUID) (Tree, error) {
	employers, err := r.queries.ListEmployersByRecency(ctx, userID)
	if err != nil {
		return Tree{}, fmt.Errorf("career tree: employers: %w", err)
	}
	positions, err := r.queries.ListPositionsByUser(ctx, userID)
	if err != nil {
		return Tree{}, fmt.Errorf("career tree: positions: %w", err)
	}
	counts, err := r.queries.CountContributionsByPositionForUser(ctx, userID)
	if err != nil {
		return Tree{}, fmt.Errorf("career tree: contribution counts: %w", err)
	}

	countsByPosition := make(map[uuid.UUID]ContributionCounts, len(counts))
	for _, c := range counts {
		countsByPosition[c.PositionID] = ContributionCounts{Active: c.Active, Hidden: c.Hidden}
	}

	positionsByEmployer := make(map[uuid.UUID][]Position, len(employers))
	for _, p := range positions {
		positionsByEmployer[p.EmployerID] = append(positionsByEmployer[p.EmployerID], Position{
			Position:           p,
			ContributionCounts: countsByPosition[p.ID],
		})
	}

	tree := Tree{Employers: make([]Employer, 0, len(employers))}
	for _, e := range employers {
		ps := positionsByEmployer[e.ID]
		if ps == nil {
			ps = []Position{}
		}
		tree.Employers = append(tree.Employers, Employer{Employer: e, Positions: ps})
	}
	return tree, nil
}

// PositionContributions reads one position's contributions and every tag on
// them in two queries, rather than one tag query per contribution.
func (r *Reader) PositionContributions(ctx context.Context, userID, positionID uuid.UUID) ([]Contribution, error) {
	contributions, err := r.queries.GetContributionsByPosition(ctx, db.GetContributionsByPositionParams{
		PositionID: &positionID,
		UserID:     userID,
	})
	if err != nil {
		return nil, fmt.Errorf("position contributions: %w", err)
	}
	tags, err := r.queries.ListContributionTagsByPosition(ctx, db.ListContributionTagsByPositionParams{
		PositionID: &positionID,
		UserID:     userID,
	})
	if err != nil {
		return nil, fmt.Errorf("position contributions: tags: %w", err)
	}

	tagsByContribution := make(map[uuid.UUID][]Tag)
	for _, t := range tags {
		tagsByContribution[t.ContributionID] = append(tagsByContribution[t.ContributionID], Tag{
			ID: t.ID, Name: t.Name, Category: t.Category,
		})
	}

	out := make([]Contribution, 0, len(contributions))
	for _, c := range contributions {
		ts := tagsByContribution[c.ID]
		if ts == nil {
			ts = []Tag{}
		}
		out = append(out, Contribution{Contribution: c, Tags: ts})
	}
	return out, nil
}
