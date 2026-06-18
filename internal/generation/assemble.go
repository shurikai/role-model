package generation

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shurikai/role-model/internal/db"
)

// AssembleContext gathers all active career data for userID into a ResumeContext
// ready for the generation prompt. No filtering or ranking is applied.
func (s *Service) AssembleContext(ctx context.Context, userID uuid.UUID) (*ResumeContext, error) {
	employers, err := s.q.GetEmployers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("assemble context: get employers: %w", err)
	}

	result := &ResumeContext{
		Employers: make([]EmployerView, 0, len(employers)),
	}

	for _, emp := range employers {
		positions, err := s.q.GetPositionsByEmployer(ctx, db.GetPositionsByEmployerParams{
			EmployerID: emp.ID,
			UserID:     userID,
		})
		if err != nil {
			return nil, fmt.Errorf("assemble context: get positions for employer %q: %w", emp.Name, err)
		}

		ev := EmployerView{
			Name:      emp.Name,
			Positions: make([]PositionView, 0, len(positions)),
		}

		for _, pos := range positions {
			contributions, err := s.q.GetContributionsByPosition(ctx, db.GetContributionsByPositionParams{
				PositionID: pos.ID,
				UserID:     userID,
			})
			if err != nil {
				return nil, fmt.Errorf("assemble context: get contributions for position %q: %w", pos.Title, err)
			}

			pv := PositionView{
				Title:            pos.Title,
				IndustryLevel:    pos.IndustryLevel,
				IndustryRole:     pos.IndustryRole,
				ContextNarrative: pos.ContextNarrative,
				Location:         pos.Location,
				StartedOn:        pos.StartedOn.Time.Format("2006-01"),
				Contributions:    make([]ContributionView, 0),
			}
			if pos.EndedOn.Valid {
				s := pos.EndedOn.Time.Format("2006-01")
				pv.EndedOn = &s
			}

			for _, c := range contributions {
				if !c.IsActive {
					continue
				}

				tags, err := s.q.GetTagsByContribution(ctx, db.GetTagsByContributionParams{
					ContributionID: c.ID,
					UserID:         userID,
				})
				if err != nil {
					return nil, fmt.Errorf("assemble context: get tags for contribution %s: %w", c.ID, err)
				}

				tagViews := make([]TagView, len(tags))
				for i, t := range tags {
					tagViews[i] = TagView{Name: t.Name, Category: t.Category}
				}

				pv.Contributions = append(pv.Contributions, ContributionView{
					ID:              c.ID,
					Summary:         c.Summary,
					FullDescription: c.FullDescription,
					Outcomes:        c.Outcomes,
					ScaleContext:    c.ScaleContext,
					Tags:            tagViews,
				})
			}

			ev.Positions = append(ev.Positions, pv)
		}

		result.Employers = append(result.Employers, ev)
	}

	return result, nil
}
