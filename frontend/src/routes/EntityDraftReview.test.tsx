import { describe, it, expect, vi, afterEach } from "vitest";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { EntityDraftReview } from "./EntityDraftReview";
import type { EntityDraft, ImportBatch } from "../lib/types";

const BATCH_ID = "batch-1";
const EMPLOYER_ID = "draft-employer";
const POSITION_ID = "draft-position";

function batch(overrides: Partial<ImportBatch> = {}): ImportBatch {
  return {
    id: BATCH_ID,
    user_id: "u1",
    raw_text: "pasted career text",
    status: "review",
    error_text: null,
    created_at: "2026-08-25T00:00:00Z",
    updated_at: "2026-08-25T00:00:00Z",
    draft_counts: { total: 6, pending: 6, approved: 0, rejected: 0 },
    ...overrides,
  };
}

function draft(
  overrides: Partial<EntityDraft> & Pick<EntityDraft, "id" | "kind">,
): EntityDraft {
  return {
    batch_id: BATCH_ID,
    payload: {},
    depends_on: [],
    resolved_id: null,
    flags: null,
    status: "pending",
    ...overrides,
  } as EntityDraft;
}

/**
 * Deliberately in the wrong order — contributions first, employer last. The
 * tree has to be built from depends_on, and a fixture in tidy parent-first
 * order would pass even if the screen just rendered the list as it arrived.
 */
function careerDrafts(): EntityDraft[] {
  return [
    draft({
      id: "draft-contribution-1",
      kind: "contribution",
      depends_on: [POSITION_ID],
      payload: {
        position_draft: POSITION_ID,
        summary: "Ran the floor",
        full_description: "Charge duties on a 24-bed unit.",
      },
    }),
    draft({
      id: "draft-contribution-2",
      kind: "contribution",
      depends_on: [POSITION_ID],
      payload: {
        position_draft: POSITION_ID,
        summary: "Rebuilt the triage protocol",
        full_description: "Cut average triage time.",
      },
    }),
    draft({
      id: "draft-skill",
      kind: "skill",
      flags: { new_categories: ["Clinical"] },
      payload: { category: "Clinical", tag: "ACLS", proficiency: "expert" },
    }),
    draft({
      id: "draft-preference",
      kind: "preference",
      flags: {
        preference_collisions: ['"night shifts" overlaps "night work"'],
      },
      payload: {
        preference_type: "role_shape",
        label: "night shifts",
        sentiment: "negative",
        weight: 8,
        is_hard_gate: false,
      },
    }),
    draft({
      id: POSITION_ID,
      kind: "position",
      depends_on: [EMPLOYER_ID],
      payload: {
        employer_draft: EMPLOYER_ID,
        title: "Staff Nurse",
        started_on: "2019-04",
      },
    }),
    draft({
      id: EMPLOYER_ID,
      kind: "employer",
      payload: { name: "Nimbus Health", industry: "healthcare", notes: null },
    }),
  ];
}

interface Recorded {
  method: string;
  path: string;
  body: unknown;
}

function stubFetch(
  drafts: EntityDraft[],
  options: { approveError?: { status: number; body: unknown } } = {},
) {
  const calls: Recorded[] = [];
  const fetchMock = vi.fn(async (url: unknown, init?: RequestInit) => {
    const path = String(url);
    const method = init?.method ?? "GET";
    calls.push({
      method,
      path,
      body: init?.body ? JSON.parse(String(init.body)) : null,
    });

    if (path.endsWith("/approve") && options.approveError) {
      return {
        ok: false,
        status: options.approveError.status,
        json: vi.fn().mockResolvedValue(options.approveError.body),
      } as unknown as Response;
    }

    let body: unknown = null;
    if (method === "GET" && path.endsWith("/entities")) {
      body = drafts;
    } else if (method === "GET") {
      body = batch();
    } else {
      body = { id: "x", status: "rejected" };
    }
    return {
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue(body),
    } as unknown as Response;
  });
  return { fetchMock, calls };
}

function renderReview() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/import/career/${BATCH_ID}`]}>
        <Routes>
          <Route
            path="/import/career/:batchID"
            element={<EntityDraftReview />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("EntityDraftReview", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("nests positions under their employer using depends_on, not list order", async () => {
    const { fetchMock } = stubFetch(careerDrafts());
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const employerCard = (await screen.findByText("Nimbus Health")).closest(
      "div.mb-2",
    );
    const employerSection = employerCard?.parentElement;
    expect(employerSection).toContainElement(screen.getByText("Staff Nurse"));
  });

  it("collapses contributions to a count rather than nesting a third level", async () => {
    const user = userEvent.setup();
    const { fetchMock } = stubFetch(careerDrafts());
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const toggle = await screen.findByRole("button", {
      name: /2 contributions drafted/,
    });
    expect(screen.queryByText("Ran the floor")).not.toBeInTheDocument();

    await user.click(toggle);
    expect(screen.getByText("Ran the floor")).toBeInTheDocument();
    expect(screen.getByText("Rebuilt the triage protocol")).toBeInTheDocument();
  });

  it("renders each flag's reason as text, not just a badge", async () => {
    const { fetchMock } = stubFetch(careerDrafts());
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    // The advisory content itself has to be readable — a warning icon with no
    // explanation tells the reviewer nothing they can act on.
    expect(
      await screen.findByText(/night shifts.*overlaps.*night work/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Would create a new category "Clinical"/),
    ).toBeInTheDocument();
  });

  it("names what would be orphaned before rejecting a draft with dependents", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(careerDrafts());
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const employerCard = (await screen.findByText("Nimbus Health")).closest(
      "div.mb-2",
    ) as HTMLElement;
    await user.click(
      within(employerCard).getByRole("button", { name: /Reject/ }),
    );

    // Transitive: the position depends on the employer, and both
    // contributions depend on the position.
    expect(
      screen.getByText(/1 position and 2 contributions depend on this/),
    ).toBeInTheDocument();
    // And nothing was sent while the question is still on screen.
    expect(calls.filter((c) => c.method === "POST")).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "Reject anyway" }));
    await waitFor(() => {
      expect(calls.some((c) => c.path.endsWith(`/${EMPLOYER_ID}/reject`))).toBe(
        true,
      );
    });
  });

  it("rejects a draft with no dependents without asking", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(careerDrafts());
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const skillCard = (await screen.findByText("ACLS")).closest(
      "div.mb-2",
    ) as HTMLElement;
    await user.click(within(skillCard).getByRole("button", { name: /Reject/ }));

    await waitFor(() => {
      expect(calls.some((c) => c.path.endsWith("/draft-skill/reject"))).toBe(
        true,
      );
    });
    expect(screen.queryByText(/depend on this/)).not.toBeInTheDocument();
  });

  it("shows which dependency is missing when an approve is refused", async () => {
    const user = userEvent.setup();
    const { fetchMock } = stubFetch(careerDrafts(), {
      approveError: {
        status: 409,
        body: {
          error:
            "draft depends on a draft that has not been resolved: employer draft draft-employer is pending",
          code: "dependency_not_resolved",
        },
      },
    });
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const positionCard = (await screen.findByText("Staff Nurse")).closest(
      "div.mb-2",
    ) as HTMLElement;
    await user.click(
      within(positionCard).getByRole("button", { name: /Approve/ }),
    );

    // On the card, naming the parent. A generic toast would leave the
    // reviewer to guess which of six cards to approve first.
    expect(
      await within(positionCard).findByText(
        /employer draft draft-employer is pending/,
      ),
    ).toBeInTheDocument();
  });

  // #89: the extractor now proposes education and credentials, and before this
  // screen knew the kinds they landed under "Not attached to anything" —
  // technically reviewable, but reading as a bug to the person reviewing it. A
  // licensed professional's credentials are the point of the section, not
  // leftovers.
  it("groups education and credentials rather than treating them as unplaced", async () => {
    const { fetchMock } = stubFetch([
      ...careerDrafts(),
      draft({
        id: "draft-education",
        kind: "education",
        payload: {
          institution: "Rush University",
          degree: "BSN",
          field_of_study: "Nursing",
        },
      }),
      draft({
        id: "draft-credential",
        kind: "credential",
        payload: { name: "ACLS", issuer: "American Heart Association" },
      }),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    // By heading role, not by text: KIND_LABEL puts "Education" on the draft
    // card's own badge too, and the payload editor repeats each value, so a
    // bare text query matches several nodes. Scope to the section instead.
    const educationHeading = await screen.findByRole("heading", {
      name: "Education",
    });
    const educationSection = educationHeading.closest("section")!;
    expect(
      within(educationSection).getAllByText("Rush University").length,
    ).toBeGreaterThan(0);

    const credentialSection = screen
      .getByRole("heading", { name: "Credentials" })
      .closest("section")!;
    expect(
      within(credentialSection).getAllByText("ACLS").length,
    ).toBeGreaterThan(0);

    // The bucket exists for kinds this screen genuinely has no place for.
    // Neither of these is one any more.
    expect(
      screen.queryByText("Not attached to anything"),
    ).not.toBeInTheDocument();
  });

  it("shows drafts it has no place for rather than dropping them", async () => {
    const { fetchMock } = stubFetch([
      ...careerDrafts(),
      draft({
        id: "draft-publication",
        kind: "publication" as EntityDraft["kind"],
        payload: { title: "A paper nobody can resolve yet" },
      }),
      draft({
        id: "draft-orphan-position",
        kind: "position",
        depends_on: ["an-employer-not-in-this-batch"],
        payload: { title: "Orphaned Role", started_on: "2015-01" },
      }),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    // kind is deliberately not a database CHECK, so an unknown kind reaches
    // this screen — and a draft that vanishes unreviewed is the failure the
    // whole draft substrate exists to prevent.
    expect(
      await screen.findByText("Not attached to anything"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("A paper nobody can resolve yet"),
    ).toBeInTheDocument();
    expect(screen.getByText("Orphaned Role")).toBeInTheDocument();
  });
});
