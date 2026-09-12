import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ImportReview } from "./ImportReview";
import type {
  ContributionDraft,
  Employer,
  ImportBatch,
  Position,
} from "../lib/types";

const BATCH_ID = "batch-1";

function makeBatch(overrides: Partial<ImportBatch> = {}): ImportBatch {
  return {
    id: BATCH_ID,
    user_id: "u1",
    raw_text: "pasted resume text",
    status: "review",
    error_text: null,
    created_at: "2026-08-25T00:00:00Z",
    updated_at: "2026-08-25T00:00:00Z",
    draft_counts: { total: 1, pending: 1, approved: 0, rejected: 0 },
    ...overrides,
  };
}

function makeDraft(
  overrides: Partial<ContributionDraft> = {},
): ContributionDraft {
  return {
    id: "draft-1",
    user_id: "u1",
    batch_id: BATCH_ID,
    employer_name: "Continental Freightways",
    position_title: "Senior Backend Engineer",
    summary: "Decomposed a monolith into services.",
    full_description: "Longer description of the same work.",
    outcomes: null,
    scale_context: null,
    flags: null,
    suggested_tags: null,
    status: "pending",
    created_at: "2026-08-25T00:00:00Z",
    updated_at: "2026-08-25T00:00:00Z",
    ...overrides,
  };
}

function makeEmployer(overrides: Partial<Employer> = {}): Employer {
  return {
    id: "employer-1",
    user_id: "u1",
    name: "Continental Freightways",
    industry: null,
    notes: null,
    created_at: "2026-08-25T00:00:00Z",
    updated_at: "2026-08-25T00:00:00Z",
    ...overrides,
  };
}

function makePosition(overrides: Partial<Position> = {}): Position {
  return {
    id: "position-1",
    user_id: "u1",
    employer_id: "employer-1",
    title: "Senior Backend Engineer",
    industry_level: null,
    industry_role: null,
    level_rationale: null,
    started_on: "2020-01",
    ended_on: null,
    context_narrative: null,
    location: null,
    sort_order: 0,
    created_at: "2026-08-25T00:00:00Z",
    updated_at: "2026-08-25T00:00:00Z",
    ...overrides,
  };
}

interface Recorded {
  method: string;
  path: string;
  body: unknown;
}

/**
 * Routes every GET the screen and its position picker can issue, and records
 * every write, so a test can assert what was actually sent — the PUT body in
 * particular, since the handler 400s on any key outside the four editable
 * fields, and the approve body, since that is what carries tag_ids.
 *
 * employers/positionsByEmployer default to empty, which is what every test
 * before the approve flow needed: the picker never gets past "no employers
 * yet" and no position is ever resolved.
 */
function stubFetch(
  batch: ImportBatch,
  drafts: ContributionDraft[],
  employers: Employer[] = [],
  positionsByEmployer: Record<string, Position[]> = {},
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

    let body: unknown = null;
    if (method === "GET" && path.endsWith("/drafts")) {
      body = drafts;
    } else if (method === "GET" && path.includes("/positions")) {
      const employerID = path.match(/\/employers\/([^/]+)\/positions/)?.[1];
      body = (employerID && positionsByEmployer[employerID]) || [];
    } else if (method === "GET" && path.endsWith("/employers")) {
      body = employers;
    } else if (method === "GET") {
      body = batch;
    } else if (path.endsWith("/reject")) {
      body = { id: drafts[0].id, status: "rejected" };
    } else {
      body = drafts[0];
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
      <MemoryRouter initialEntries={[`/import/${BATCH_ID}`]}>
        <Routes>
          <Route path="/import/:batchID" element={<ImportReview />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

/**
 * Drives the approve flow through PositionPicker to a resolved position: open
 * the picker, pick the (only) employer, pick the (only) position. Every
 * approve test uses exactly one of each, so there is nothing to disambiguate.
 */
async function approveThroughPicker(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: /Approve/ }));
  await user.click(
    await screen.findByRole("button", { name: "Continental Freightways" }),
  );
  await user.click(
    await screen.findByRole("button", { name: /Senior Backend Engineer/ }),
  );
}

describe("ImportReview", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("renders the four editable fields seeded from the draft", async () => {
    const { fetchMock } = stubFetch(makeBatch(), [makeDraft()]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    expect(await screen.findByLabelText("Summary")).toHaveValue(
      "Decomposed a monolith into services.",
    );
    expect(screen.getByLabelText("Full description")).toHaveValue(
      "Longer description of the same work.",
    );
    // Null fields are still editable, just empty — a gap is a thing to fill in.
    expect(screen.getByLabelText("Outcomes")).toHaveValue("");
    expect(screen.getByLabelText("Scale context")).toHaveValue("");
  });

  it("sends only the field that changed, on blur", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(makeBatch(), [makeDraft()]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const summary = await screen.findByLabelText("Summary");
    await user.clear(summary);
    await user.type(summary, "Rewrote it by hand.");
    await user.tab();

    await waitFor(() => {
      expect(calls.some((c) => c.method === "PUT")).toBe(true);
    });
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.path).toContain("/import/drafts/draft-1");
    // The whole point: one key, and not one of the two flaggable-but-not-
    // editable fields, which would fail the request outright.
    expect(put.body).toEqual({ summary: "Rewrote it by hand." });
  });

  it("does not write when a field is blurred untouched", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(makeBatch(), [makeDraft()]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const summary = await screen.findByLabelText("Summary");
    await user.click(summary);
    await user.tab();

    expect(calls.filter((c) => c.method === "PUT")).toHaveLength(0);
  });

  it("groups flags under the field each one names", async () => {
    const { fetchMock } = stubFetch(makeBatch(), [
      makeDraft({
        flags: [
          {
            type: "inference",
            field: "summary",
            message: "The 40% figure is not in the source.",
          },
          {
            type: "gap",
            field: "scale_context",
            message: "No team size stated.",
          },
        ],
      }),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const summaryFlag = await screen.findByText(
      "The 40% figure is not in the source.",
    );
    const scaleFlag = screen.getByText("No team size stated.");

    // Each flag renders next to its own textarea rather than in one list at
    // the bottom — there is no span-level location data, but there is a field.
    expect(
      summaryFlag.closest("div")?.parentElement?.parentElement,
    ).toContainElement(screen.getByLabelText("Summary"));
    expect(
      scaleFlag.closest("div")?.parentElement?.parentElement,
    ).toContainElement(screen.getByLabelText("Scale context"));
  });

  it("summarises flags by count and highest severity, never as confidence", async () => {
    const { fetchMock } = stubFetch(makeBatch(), [
      makeDraft({
        flags: [
          {
            type: "suggestion",
            field: "summary",
            message: "Could be tighter.",
          },
          { type: "warning", field: "outcomes", message: "This looks wrong." },
        ],
      }),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    const badge = await screen.findByTitle("Most severe: warning");
    expect(badge).toHaveTextContent("2 flags");
    expect(screen.queryByText(/confidence/i)).not.toBeInTheDocument();
  });

  it("marks a draft with no flags as clean", async () => {
    const { fetchMock } = stubFetch(makeBatch(), [makeDraft({ flags: [] })]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    expect(await screen.findByText("clean")).toBeInTheDocument();
  });

  it("rejects a draft through the reject endpoint, with no body", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(makeBatch(), [makeDraft()]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    await user.click(await screen.findByRole("button", { name: /Reject/ }));

    await waitFor(() => {
      const reject = calls.find((c) => c.path.endsWith("/reject"));
      expect(reject).toBeDefined();
      expect(reject!.method).toBe("POST");
      expect(reject!.body).toBeNull();
    });
  });

  it("skips locally without calling the backend", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(makeBatch(), [makeDraft()]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    await user.click(await screen.findByRole("button", { name: /Skip/ }));

    expect(await screen.findByText("Skipped")).toBeInTheDocument();
    // There is no "skipped" status on the backend. Writing one would be
    // inventing state; this must stay a local, session-only decision.
    expect(calls.filter((c) => c.method !== "GET")).toHaveLength(0);
  });

  it("shows progress instead of a draft list while the batch is still working", async () => {
    const { fetchMock } = stubFetch(makeBatch({ status: "enriching" }), [
      makeDraft(),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    expect(await screen.findByText(/Still working/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Summary")).not.toBeInTheDocument();
  });

  it("renders drafts for any status that is not a working one", async () => {
    // The regression this exists for: the screen first tested for a status of
    // "ready", which the database's CHECK constraint does not contain and
    // nothing writes. Stage 0 finishes at "review", so every real batch read
    // as still-extracting and no draft ever rendered. Testing the working set
    // means an unrecognised status degrades to showing the drafts.
    for (const status of ["review", "complete", "something-new"] as const) {
      const { fetchMock } = stubFetch(
        makeBatch({ status: status as ImportBatch["status"] }),
        [makeDraft()],
      );
      vi.stubGlobal("fetch", fetchMock);
      renderReview();

      expect(await screen.findByLabelText("Summary")).toBeInTheDocument();
      expect(screen.queryByText(/Still working/)).not.toBeInTheDocument();
      cleanup();
    }
  });

  it("surfaces the batch's own error text when extraction failed", async () => {
    const { fetchMock } = stubFetch(
      makeBatch({ status: "failed", error_text: "model returned no entries" }),
      [],
    );
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    expect(await screen.findByText(/Import failed/)).toBeInTheDocument();
    expect(screen.getByText("model returned no entries")).toBeInTheDocument();
  });

  // #137: Stage 0b's suggested_tags carried a typed contract and a resolve
  // endpoint (#19) with nothing in this screen rendering or using either.

  it("renders no tag section when a draft has no suggested tags", async () => {
    const { fetchMock } = stubFetch(makeBatch(), [
      makeDraft({ suggested_tags: null }),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    await screen.findByLabelText("Summary");
    expect(screen.queryByText("Suggested tags")).not.toBeInTheDocument();
  });

  it("renders suggested tags checked by default, grouped by category", async () => {
    const { fetchMock } = stubFetch(makeBatch(), [
      makeDraft({
        suggested_tags: [
          { tag_id: "t-go", name: "Go", category: "Languages" },
          {
            tag_id: "t-k8s",
            name: "Kubernetes",
            category: "Cloud & Infrastructure",
          },
        ],
      }),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    expect(await screen.findByRole("checkbox", { name: "Go" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Kubernetes" })).toBeChecked();
    expect(screen.getByText("Languages")).toBeInTheDocument();
    expect(screen.getByText("Cloud & Infrastructure")).toBeInTheDocument();
  });

  it("sends the checked suggested tag ids in the approve body", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(
      makeBatch(),
      [
        makeDraft({
          suggested_tags: [
            { tag_id: "t-go", name: "Go", category: "Languages" },
            {
              tag_id: "t-k8s",
              name: "Kubernetes",
              category: "Cloud & Infrastructure",
            },
          ],
        }),
      ],
      [makeEmployer()],
      { "employer-1": [makePosition()] },
    );
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    await screen.findByRole("checkbox", { name: "Go" });
    await approveThroughPicker(user);

    await waitFor(() => {
      expect(calls.some((c) => c.path.endsWith("/approve"))).toBe(true);
    });
    const approve = calls.find((c) => c.path.endsWith("/approve"))!;
    expect(approve.body).toEqual({
      position_id: "position-1",
      tag_ids: ["t-go", "t-k8s"],
    });
  });

  it("drops an unchecked suggested tag id from the approve body", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(
      makeBatch(),
      [
        makeDraft({
          suggested_tags: [
            { tag_id: "t-go", name: "Go", category: "Languages" },
            {
              tag_id: "t-k8s",
              name: "Kubernetes",
              category: "Cloud & Infrastructure",
            },
          ],
        }),
      ],
      [makeEmployer()],
      { "employer-1": [makePosition()] },
    );
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    await user.click(
      await screen.findByRole("checkbox", { name: "Kubernetes" }),
    );
    await approveThroughPicker(user);

    await waitFor(() => {
      expect(calls.some((c) => c.path.endsWith("/approve"))).toBe(true);
    });
    const approve = calls.find((c) => c.path.endsWith("/approve"))!;
    expect(approve.body).toEqual({
      position_id: "position-1",
      tag_ids: ["t-go"],
    });
  });

  it("omits tag_ids from the approve body when there were no suggestions", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch(
      makeBatch(),
      [makeDraft({ suggested_tags: null })],
      [makeEmployer()],
      { "employer-1": [makePosition()] },
    );
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    await approveThroughPicker(user);

    await waitFor(() => {
      expect(calls.some((c) => c.path.endsWith("/approve"))).toBe(true);
    });
    const approve = calls.find((c) => c.path.endsWith("/approve"))!;
    expect(approve.body).toEqual({ position_id: "position-1" });
  });
});
