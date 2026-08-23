package intake

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// ResolveOrCreateTag returns the tag named name in category, creating the
// category, the tag, or both if they are missing.
//
// This exists because `skills -> tags -> tag_categories` is a three-link chain
// held together by a composite foreign key — tags carries (user_id, category)
// REFERENCES tag_categories (user_id, name) — and an import hearing "fluent in
// Spanish" needs all three writes in the right order. A careful human writing
// seed SQL does that by hand and gets it right; an extractor proposing forty
// skills does not, and the failure is an FK violation that aborts the whole
// batch rather than a flag on one draft.
//
// Matching is case-insensitive on the name, because an extractor writes
// "postgresql" where the user wrote "PostgreSQL" and creating a second tag for
// it would split the evidence for every requirement either one answers. The
// EXISTING spelling wins: it is the one already attached to contributions, and
// renaming it out from under them to match an extractor's capitalisation is
// not an improvement.
//
// A category created here carries no aliases. That is correct rather than
// incomplete — the competency vocabulary is what lets a capability-worded
// posting reach this category, and guessing it from a tag name would produce
// exactly the over-broad aliases the seed tests exist to prevent. It belongs in
// review, which is why the caller flags it.
func ResolveOrCreateTag(ctx context.Context, q *db.Queries, userID uuid.UUID, category, name string) (db.Tag, error) {
	category = strings.TrimSpace(category)
	name = strings.TrimSpace(name)
	if category == "" || name == "" {
		return db.Tag{}, fmt.Errorf("resolve tag: category and name are both required (got %q / %q)", category, name)
	}

	tags, err := q.ListTags(ctx, userID)
	if err != nil {
		return db.Tag{}, fmt.Errorf("resolve tag: list tags: %w", err)
	}
	for _, t := range tags {
		if strings.EqualFold(t.Name, name) {
			return t, nil
		}
	}

	categories, err := q.ListTagCategories(ctx, userID)
	if err != nil {
		return db.Tag{}, fmt.Errorf("resolve tag: list categories: %w", err)
	}
	categoryName := ""
	maxSort := int32(0)
	for _, c := range categories {
		if c.SortOrder > maxSort {
			maxSort = c.SortOrder
		}
		if strings.EqualFold(c.Name, category) {
			categoryName = c.Name
		}
	}

	if categoryName == "" {
		created, err := q.CreateTagCategory(ctx, db.CreateTagCategoryParams{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      category,
			SortOrder: maxSort + 1,
		})
		if err != nil {
			return db.Tag{}, fmt.Errorf("resolve tag: create category %q: %w", category, err)
		}
		categoryName = created.Name
	}

	tag, err := q.CreateTag(ctx, db.CreateTagParams{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     name,
		Aliases:  nil,
		Category: categoryName,
	})
	if err != nil {
		return db.Tag{}, fmt.Errorf("resolve tag: create tag %q in %q: %w", name, categoryName, err)
	}
	return tag, nil
}
