package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// DraftFlags is what a reviewer sees before approving. Same shape and purpose
// as contribution_drafts.flags: things a human should look at, never a
// rejection. The import proposes; the person decides.
type DraftFlags struct {
	// Collisions on a drafted preference label. Advisory: two labels sharing
	// vocabulary can be legitimate, and the person is better placed to say.
	PreferenceCollisions []string `json:"preference_collisions,omitempty"`

	// A category this draft would create. Worth surfacing because a new
	// category starts with no competency vocabulary, which means a
	// capability-worded posting cannot reach it until someone writes some.
	NewCategories []string `json:"new_categories,omitempty"`
}

// FlagDraft computes review flags for one drafted entity and stores them.
//
// This runs at DRAFT time rather than at approval time on purpose. A collision
// found at approval is found after the reviewer has already decided; a
// collision found at draft time is on the card they are reading.
func FlagDraft(ctx context.Context, q *db.Queries, userID uuid.UUID, d db.EntityDraft) (DraftFlags, error) {
	var flags DraftFlags

	switch d.Kind {
	case KindPreference:
		var p preferencePayload
		if err := payloadOf(d, &p); err != nil {
			return flags, err
		}
		existing, err := q.ListPreferencesByUser(ctx, userID)
		if err != nil {
			return flags, fmt.Errorf("flag draft: list preferences: %w", err)
		}
		labels := make([]string, 0, len(existing))
		for _, e := range existing {
			labels = append(labels, e.Label)
		}
		for _, c := range CheckPreferenceLabel(p.Label, labels) {
			flags.PreferenceCollisions = append(flags.PreferenceCollisions, c.String())
		}

	case KindSkill:
		var p skillPayload
		if err := payloadOf(d, &p); err != nil {
			return flags, err
		}
		isNew, err := categoryIsNew(ctx, q, userID, p.Category)
		if err != nil {
			return flags, err
		}
		if isNew {
			flags.NewCategories = append(flags.NewCategories, p.Category)
		}
	}

	if flags.PreferenceCollisions == nil && flags.NewCategories == nil {
		return flags, nil
	}

	raw, err := json.Marshal(flags)
	if err != nil {
		return flags, fmt.Errorf("flag draft: marshal flags: %w", err)
	}
	msg := json.RawMessage(raw)
	if _, err := q.SetEntityDraftFlags(ctx, db.SetEntityDraftFlagsParams{
		ID: d.ID, UserID: userID, Flags: &msg,
	}); err != nil {
		return flags, fmt.Errorf("flag draft: store flags: %w", err)
	}
	return flags, nil
}

func categoryIsNew(ctx context.Context, q *db.Queries, userID uuid.UUID, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	categories, err := q.ListTagCategories(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("flag draft: list categories: %w", err)
	}
	for _, c := range categories {
		if strings.EqualFold(c.Name, name) {
			return false, nil
		}
	}
	return true, nil
}
