package intake

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

func draft(id uuid.UUID, kind string, deps ...uuid.UUID) db.EntityDraft {
	payload := json.RawMessage(`{}`)
	return db.EntityDraft{
		ID: id, Kind: kind, Status: "pending",
		DependsOn: deps, Payload: &payload,
	}
}

// The ordering problem this substrate exists for. A position has no employer_id
// until its employer draft is approved, and a contribution has no position_id
// until the position is — which is why contribution_drafts stored both parents
// as plain text and then required the caller to have created them by hand.
func TestTopoOrderResolvesParentsFirst(t *testing.T) {
	employer := uuid.New()
	position := uuid.New()
	contribution := uuid.New()

	// Deliberately supplied child-first: the input order is whatever the
	// extractor emitted, and depending on it is the bug.
	drafts := []db.EntityDraft{
		draft(contribution, KindContribution, position),
		draft(position, KindPosition, employer),
		draft(employer, KindEmployer),
	}

	order, stuck := topoOrder(drafts, map[uuid.UUID]uuid.UUID{})

	if len(stuck) != 0 {
		t.Fatalf("stuck drafts on a well-formed batch: %v", stuck)
	}
	got := []uuid.UUID{order[0].ID, order[1].ID, order[2].ID}
	want := []uuid.UUID{employer, position, contribution}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// A draft whose parent already exists as a row resolves on its own. This is the
// common case after the first import: three new contributions against employers
// that have been there for years.
func TestTopoOrderAcceptsAlreadyResolvedParents(t *testing.T) {
	employer := uuid.New()
	position := uuid.New()

	drafts := []db.EntityDraft{draft(position, KindPosition, employer)}
	already := map[uuid.UUID]uuid.UUID{employer: uuid.New()}

	order, stuck := topoOrder(drafts, already)
	if len(order) != 1 || len(stuck) != 0 {
		t.Fatalf("order = %v, stuck = %v; want the position resolvable", order, stuck)
	}
}

// A draft depending on something that was rejected must be REPORTED, not
// silently skipped. A draft that vanishes from a review queue without becoming
// a row is exactly the failure this substrate exists to avoid.
func TestTopoOrderReportsUnreachableDrafts(t *testing.T) {
	rejected := uuid.New()
	orphan := uuid.New()

	drafts := []db.EntityDraft{
		{ID: rejected, Kind: KindEmployer, Status: "rejected"},
		draft(orphan, KindPosition, rejected),
	}

	order, stuck := topoOrder(drafts, map[uuid.UUID]uuid.UUID{})
	if len(order) != 0 {
		t.Errorf("resolved %d drafts; the orphan's parent was rejected", len(order))
	}
	why, ok := stuck[orphan]
	if !ok {
		t.Fatal("the orphaned draft was neither resolved nor reported")
	}
	if want := rejected.String(); !contains(why, want) {
		t.Errorf("reason %q does not name the unmet dependency %s", why, want)
	}
}

// A cycle is reported as a cycle rather than looping forever or dropping both
// drafts silently.
func TestTopoOrderReportsCycles(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	drafts := []db.EntityDraft{
		draft(a, KindPosition, b),
		draft(b, KindPosition, a),
	}

	order, stuck := topoOrder(drafts, map[uuid.UUID]uuid.UUID{})
	if len(order) != 0 {
		t.Errorf("resolved %d drafts out of a cycle", len(order))
	}
	if len(stuck) != 2 {
		t.Fatalf("stuck = %v, want both drafts reported", stuck)
	}
	for id, why := range stuck {
		if !contains(why, "cycle") {
			t.Errorf("draft %s reported as %q, want it to name the cycle", id, why)
		}
	}
}

// The order must be stable: a batch that resolves has to resolve the same way
// twice, or a retry after a partial failure writes rows in a different order
// and a bug becomes unreproducible.
func TestTopoOrderIsStable(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	drafts := []db.EntityDraft{
		draft(ids[0], KindEmployer),
		draft(ids[1], KindEmployer),
		draft(ids[2], KindPosition, ids[0]),
		draft(ids[3], KindPosition, ids[1]),
	}

	first, _ := topoOrder(drafts, map[uuid.UUID]uuid.UUID{})
	for i := 0; i < 20; i++ {
		again, _ := topoOrder(drafts, map[uuid.UUID]uuid.UUID{})
		for j := range first {
			if first[j].ID != again[j].ID {
				t.Fatalf("order is not stable: position %d was %s, now %s", j, first[j].ID, again[j].ID)
			}
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
