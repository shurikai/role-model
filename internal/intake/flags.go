package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

	// Every new category the batch proposes, set only when that set is large
	// or visibly redundant enough to be worth reviewing as a group before any
	// one of them is approved. #88: one 640-word career extracted to thirteen
	// categories, several of them restatements of each other. Every tag sits
	// in exactly one category, so a capability spread across three is the
	// evidence for it split three ways.
	CrowdedCategories []string `json:"crowded_categories,omitempty"`
}

// crowdedCategoryLimit is the count above which a batch's new categories are
// flagged for review as a group. The extraction prompt asks for four to eight
// per career; more than eight new ones in a single import is the shape #88
// describes, where thirteen were proposed and a human merged them to eight.
const crowdedCategoryLimit = 8

// FlagDraft computes review flags for one drafted entity and stores them.
//
// This runs at DRAFT time rather than at approval time on purpose. A collision
// found at approval is found after the reviewer has already decided; a
// collision found at draft time is on the card they are reading.
//
// batch is every draft staged in the same import, so a flag can be about the
// set rather than the row — a preference checked against the ones drafted
// alongside it, a new category weighed against the other new categories.
func FlagDraft(ctx context.Context, q *db.Queries, userID uuid.UUID, d db.EntityDraft, batch []db.EntityDraft) (DraftFlags, error) {
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

			crowded, err := crowdedBatchCategories(ctx, q, userID, batch)
			if err != nil {
				return flags, err
			}
			flags.CrowdedCategories = crowded
		}
	}

	if flags.PreferenceCollisions == nil && flags.NewCategories == nil && flags.CrowdedCategories == nil {
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

// crowdedBatchCategories returns every new category the batch proposes when
// that set is large (more than crowdedCategoryLimit) or visibly redundant (two
// names sharing a word), and nil otherwise.
//
// #88: one 640-word career extracted to thirteen categories, several of them
// restatements of each other — "Quality Improvement" / "Quality and Safety" /
// "Process Improvement". Because a tag sits in exactly one category, a
// capability spread across three is the evidence for it split three ways, and
// the fit gate's category layer then answers a posting with whichever fragment
// its wording happens to reach. A human merged that set to eight in review;
// this puts the whole set on the cards so that judgment is prompted rather
// than left to be noticed.
func crowdedBatchCategories(ctx context.Context, q *db.Queries, userID uuid.UUID, batch []db.EntityDraft) ([]string, error) {
	existing, err := q.ListTagCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("flag draft: list categories: %w", err)
	}
	have := make(map[string]bool, len(existing))
	for _, c := range existing {
		have[strings.ToLower(strings.TrimSpace(c.Name))] = true
	}

	var proposed []string
	seen := map[string]bool{}
	for _, row := range batch {
		if row.Kind != KindSkill {
			continue
		}
		var p skillPayload
		if err := payloadOf(row, &p); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(p.Category)
		key := strings.ToLower(name)
		if name == "" || have[key] || seen[key] {
			continue
		}
		seen[key] = true
		proposed = append(proposed, name)
	}

	if !isCrowded(proposed) {
		return nil, nil
	}
	sort.Strings(proposed)
	return proposed, nil
}

// isCrowded reports whether a set of newly proposed categories is large
// (more than crowdedCategoryLimit) or visibly redundant (two names sharing a
// word) enough to review as a group.
func isCrowded(proposed []string) bool {
	return len(proposed) > crowdedCategoryLimit || hasSharedToken(proposed)
}

// hasSharedToken reports whether any two of the names share a word of four or
// more letters. "Quality Improvement" and "Process Improvement" share
// "improvement"; that overlap is the signal that two categories are one idea
// written twice. It reuses tokenBag, so the split matches the fit gate's.
func hasSharedToken(names []string) bool {
	seen := map[string]bool{}
	for _, n := range names {
		local := map[string]bool{}
		for _, tok := range tokenBag(n) {
			if len(tok) < 4 || local[tok] {
				continue
			}
			local[tok] = true
			if seen[tok] {
				return true
			}
			seen[tok] = true
		}
	}
	return false
}
