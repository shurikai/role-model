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
				ID:               pos.ID,
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

func (s *Service) assembleProjects(ctx context.Context, userID uuid.UUID) ([]ProjectView, error) {
	projects, err := s.q.GetProjects(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("assemble projects: get projects: %w", err)
	}

	result := make([]ProjectView, 0, len(projects))

	for _, p := range projects {
		contributions, err := s.q.GetContributionsByProject(ctx, db.GetContributionsByProjectParams{
			ProjectID: p.ID,
			UserID:    userID,
		})
		if err != nil {
			return nil, fmt.Errorf("assemble projects: contributions for project %q: %w", p.Name, err)
		}

		cvs := make([]ContributionView, 0, len(contributions))
		for _, c := range contributions {
			tags, err := s.q.GetTagsByContribution(ctx, db.GetTagsByContributionParams{
				ContributionID: c.ID,
				UserID:         userID,
			})
			if err != nil {
				return nil, fmt.Errorf("assemble projects: tags for contribution %s: %w", c.ID, err)
			}

			tagViews := make([]TagView, len(tags))
			for i, t := range tags {
				tagViews[i] = TagView{Name: t.Name, Category: t.Category}
			}

			cvs = append(cvs, ContributionView{
				ID:              c.ID,
				Summary:         c.Summary,
				FullDescription: c.FullDescription,
				Outcomes:        c.Outcomes,
				ScaleContext:    c.ScaleContext,
				Tags:            tagViews,
			})
		}

		projectTags, err := s.q.GetTagsByProject(ctx, db.GetTagsByProjectParams{
			ProjectID: p.ID,
			UserID:    userID,
		})
		if err != nil {
			return nil, fmt.Errorf("assemble projects: tags for project %q: %w", p.Name, err)
		}

		tagViews := make([]TagView, len(projectTags))
		for i, t := range projectTags {
			tagViews[i] = TagView{Name: t.Name, Category: t.Category}
		}

		pv := ProjectView{
			ID:            p.ID,
			Name:          p.Name,
			Tagline:       p.Tagline,
			Role:          p.Role,
			Status:        p.Status,
			RepoURL:       p.RepoUrl,
			LiveURL:       p.LiveUrl,
			WriteupURL:    p.WriteupUrl,
			ForceInclude:  p.ForceInclude,
			ForceExclude:  p.ForceExclude,
			Contributions: cvs,
			Tags:          tagViews,
		}
		if p.StartedOn.Valid {
			d := p.StartedOn.Time.Format("2006-01")
			pv.StartedOn = &d
		}
		if p.EndedOn.Valid {
			d := p.EndedOn.Time.Format("2006-01")
			pv.EndedOn = &d
		}

		result = append(result, pv)
	}

	return result, nil
}

func (s *Service) assembleEducation(ctx context.Context, userID uuid.UUID) ([]EducationView, error) {
	rows, err := s.q.GetEducation(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("assemble education: %w", err)
	}

	result := make([]EducationView, 0, len(rows))
	for _, e := range rows {
		ev := EducationView{
			Institution:  e.Institution,
			Degree:       e.Degree,
			FieldOfStudy: e.FieldOfStudy,
			Notes:        e.Notes,
		}
		if e.EndedOn.Valid {
			d := e.EndedOn.Time.Format("2006")
			ev.Graduated = &d
		}
		result = append(result, ev)
	}

	return result, nil
}

func (s *Service) assembleCredentials(ctx context.Context, userID uuid.UUID) ([]CredentialView, error) {
	rows, err := s.q.GetCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("assemble credentials: %w", err)
	}

	result := make([]CredentialView, 0, len(rows))
	for _, c := range rows {
		cv := CredentialView{
			Name:          c.Name,
			Issuer:        c.Issuer,
			CredentialURL: c.CredentialUrl,
		}
		if c.IssuedOn.Valid {
			d := c.IssuedOn.Time.Format("2006-01")
			cv.IssuedOn = &d
		}
		if c.ExpiresOn.Valid {
			d := c.ExpiresOn.Time.Format("2006-01")
			cv.ExpiresOn = &d
		}
		result = append(result, cv)
	}

	return result, nil
}
