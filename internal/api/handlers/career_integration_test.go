//go:build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/testenv"
)

type careerFixture struct {
	t   *testing.T
	ctx context.Context
	q   *db.Queries
}

func newCareerFixture(t *testing.T) *careerFixture {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testenv.DatabaseURL(t))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return &careerFixture{t: t, ctx: ctx, q: db.New(pool)}
}

func (f *careerFixture) user() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if _, err := f.q.CreateUser(f.ctx, db.CreateUserParams{
		ID: id, Email: "career-" + id.String() + "@test.local",
	}); err != nil {
		f.t.Fatalf("create user: %v", err)
	}
	return id
}

func (f *careerFixture) employer(userID uuid.UUID, name string) uuid.UUID {
	f.t.Helper()
	e, err := f.q.CreateEmployer(f.ctx, db.CreateEmployerParams{ID: uuid.New(), UserID: userID, Name: name})
	if err != nil {
		f.t.Fatalf("create employer %q: %v", name, err)
	}
	return e.ID
}

func day(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return pgtype.Date{Time: d, Valid: true}
}

func (f *careerFixture) position(userID, employerID uuid.UUID, title, started, ended string, sortOrder int32) uuid.UUID {
	f.t.Helper()
	p, err := f.q.CreatePosition(f.ctx, db.CreatePositionParams{
		ID: uuid.New(), UserID: userID, EmployerID: employerID, Title: title,
		StartedOn: day(started), EndedOn: day(ended), SortOrder: sortOrder,
	})
	if err != nil {
		f.t.Fatalf("create position %q: %v", title, err)
	}
	return p.ID
}

func (f *careerFixture) contribution(userID, positionID uuid.UUID, summary string, active bool) uuid.UUID {
	f.t.Helper()
	c, err := f.q.CreateContribution(f.ctx, db.CreateContributionParams{
		ID: uuid.New(), UserID: userID, PositionID: &positionID,
		Summary: summary, FullDescription: summary + " in full.", IsActive: active,
	})
	if err != nil {
		f.t.Fatalf("create contribution: %v", err)
	}
	return c.ID
}

func (f *careerFixture) tag(userID uuid.UUID, category, name string, sortOrder int32) uuid.UUID {
	f.t.Helper()
	// A category is unique per user; a second tag in one category reuses it.
	_, _ = f.q.CreateTagCategory(f.ctx, db.CreateTagCategoryParams{
		ID: uuid.New(), UserID: userID, Name: category,
	})
	t, err := f.q.CreateTag(f.ctx, db.CreateTagParams{
		ID: uuid.New(), UserID: userID, Name: name, Category: category, SortOrder: sortOrder,
	})
	if err != nil {
		f.t.Fatalf("create tag %q: %v", name, err)
	}
	return t.ID
}

func (f *careerFixture) link(contributionID, tagID uuid.UUID) {
	f.t.Helper()
	if err := f.q.AssignTagToContribution(f.ctx, db.AssignTagToContributionParams{
		ContributionID: contributionID, TagID: tagID,
	}); err != nil {
		f.t.Fatalf("assign tag: %v", err)
	}
}

type careerResponse struct {
	Employers []struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		Positions []struct {
			ID                 uuid.UUID `json:"id"`
			Title              string    `json:"title"`
			StartedOn          *string   `json:"started_on"`
			EndedOn            *string   `json:"ended_on"`
			ContributionCounts struct {
				Active int `json:"active"`
				Hidden int `json:"hidden"`
			} `json:"contribution_counts"`
		} `json:"positions"`
	} `json:"employers"`
}

func getCareer(t *testing.T, q *db.Queries, userID uuid.UUID) (*httptest.ResponseRecorder, careerResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/career", nil)
	req = req.WithContext(httputil.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	NewCareerHandler(q).Get(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var got careerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	return rec, got
}

func TestCareerTree(t *testing.T) {
	f := newCareerFixture(t)
	userID := f.user()

	// Created in an order that is neither alphabetical nor resume order, so
	// passing proves the query sorts rather than echoing insertion order.
	old := f.employer(userID, "Aardvark Old Co")
	empty := f.employer(userID, "Bare Employer")
	current := f.employer(userID, "Zephyr Current")
	recent := f.employer(userID, "Middle Recent")

	f.position(userID, old, "Junior", "2010-01-01", "2014-06-30", 0)
	f.position(userID, recent, "Mid", "2014-07-01", "2020-12-31", 0)
	// Two at the current employer: the earlier one ended, the later one is
	// ongoing. Within an employer, newest start comes first.
	ended := f.position(userID, current, "Senior", "2021-01-01", "2023-03-31", 0)
	ongoing := f.position(userID, current, "Lead", "2023-04-01", "", 0)

	f.contribution(userID, ended, "Shipped a thing", true)
	f.contribution(userID, ended, "Shipped another", true)
	f.contribution(userID, ended, "Retired work", false)

	// Another user's career must not appear.
	otherID := f.user()
	f.position(otherID, f.employer(otherID, "Elsewhere"), "Other", "2022-01-01", "", 0)

	_, got := getCareer(t, f.q, userID)

	wantOrder := []uuid.UUID{current, recent, old, empty}
	if len(got.Employers) != len(wantOrder) {
		t.Fatalf("want %d employers, got %d", len(wantOrder), len(got.Employers))
	}
	for i, id := range wantOrder {
		if got.Employers[i].ID != id {
			t.Errorf("employer %d = %s, want %v", i, got.Employers[i].Name, id)
		}
	}

	cur := got.Employers[0]
	if len(cur.Positions) != 2 || cur.Positions[0].ID != ongoing || cur.Positions[1].ID != ended {
		t.Fatalf("current employer positions out of order: %+v", cur.Positions)
	}
	if cur.Positions[0].EndedOn != nil {
		t.Errorf("ongoing position ended_on = %v, want null", *cur.Positions[0].EndedOn)
	}
	if c := cur.Positions[1].ContributionCounts; c.Active != 2 || c.Hidden != 1 {
		t.Errorf("counts = %+v, want {2 1}", c)
	}
	if c := cur.Positions[0].ContributionCounts; c.Active != 0 || c.Hidden != 0 {
		t.Errorf("position with no contributions counts = %+v, want zeros", c)
	}

	if ps := got.Employers[3].Positions; ps == nil || len(ps) != 0 {
		t.Errorf("employer with no positions: want [], got %v", ps)
	}
}

func TestCareerTreeEmpty(t *testing.T) {
	f := newCareerFixture(t)
	rec, _ := getCareer(t, f.q, f.user())
	if body := bytes.TrimSpace(rec.Body.Bytes()); string(body) != `{"employers":[]}` {
		t.Fatalf(`want {"employers":[]}, got %s`, body)
	}
}

// sort_order outranks dates: it is the user's own ordering when they have set
// one.
func TestCareerTreeSortOrderOutranksDates(t *testing.T) {
	f := newCareerFixture(t)
	userID := f.user()
	e := f.employer(userID, "Acme")
	newer := f.position(userID, e, "Newer", "2022-01-01", "", 1)
	older := f.position(userID, e, "Older", "2015-01-01", "2021-12-31", 0)

	_, got := getCareer(t, f.q, userID)
	ps := got.Employers[0].Positions
	if len(ps) != 2 || ps[0].ID != older || ps[1].ID != newer {
		t.Fatalf("want [Older Newer] by sort_order, got %+v", ps)
	}
}

func TestListContributionsCarriesTags(t *testing.T) {
	f := newCareerFixture(t)
	userID := f.user()
	pos := f.position(userID, f.employer(userID, "Acme"), "Engineer", "2020-01-01", "", 0)

	tagged := f.contribution(userID, pos, "Tagged", true)
	untagged := f.contribution(userID, pos, "Untagged", false)
	goTag := f.tag(userID, "Languages", "Go", 1)
	pgTag := f.tag(userID, "Datastores", "PostgreSQL", 0)
	f.link(tagged, goTag)
	f.link(tagged, pgTag)

	// A contribution on a different position, tagged, must not leak in.
	otherPos := f.position(userID, f.employer(userID, "Other"), "Other", "2010-01-01", "2012-01-01", 0)
	f.link(f.contribution(userID, otherPos, "Elsewhere", true), goTag)

	r := chi.NewRouter()
	r.Get("/positions/{positionID}/contributions", NewContributionHandler(f.q, nil).ListByPosition)
	get := func(user, position uuid.UUID) []byte {
		req := httptest.NewRequest(http.MethodGet, "/positions/"+position.String()+"/contributions", nil)
		req = req.WithContext(httputil.WithUserID(req.Context(), user))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		return rec.Body.Bytes()
	}

	var got []struct {
		ID       uuid.UUID `json:"id"`
		Summary  string    `json:"summary"`
		IsActive bool      `json:"is_active"`
		Tags     []struct {
			ID       uuid.UUID `json:"id"`
			Name     string    `json:"name"`
			Category string    `json:"category"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(get(userID, pos), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 || got[0].ID != tagged || got[1].ID != untagged {
		t.Fatalf("want [tagged untagged], got %+v", got)
	}
	// Tags in sort_order: PostgreSQL (0) before Go (1).
	if ts := got[0].Tags; len(ts) != 2 || ts[0].Name != "PostgreSQL" || ts[1].Name != "Go" || ts[1].Category != "Languages" {
		t.Errorf("tags = %+v, want [PostgreSQL Go]", ts)
	}
	if got[1].Tags == nil || len(got[1].Tags) != 0 {
		t.Errorf("untagged contribution: want tags [], got %v", got[1].Tags)
	}
	if got[1].IsActive {
		t.Errorf("hidden contribution reads as active")
	}

	// Another user asking for this position sees nothing, not someone else's
	// contributions; and an empty list is [], not null.
	if body := bytes.TrimSpace(get(f.user(), pos)); string(body) != "[]" {
		t.Errorf("other user: want [], got %s", body)
	}
}
