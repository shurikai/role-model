import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ImportReview } from "./ImportReview";
import type { ContributionDraft, ImportBatch } from "../lib/types";

const BATCH_ID = "batch-1";

function makeBatch(overrides: Partial<ImportBatch> = {}): ImportBatch {
  return {
    id: BATCH_ID,
    user_id: "u1",
    raw_text: "pasted resume text",
    status: "ready",
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
    employer_name: "Daugherty Business Solutions",
    position_title: "Senior Backend Engineer",
    summary: "Decomposed a monolith into services.",
    full_description: "Longer description of the same work.",
    outcomes: null,
    scale_context: null,
    flags: null,
    status: "pending",
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
 * Routes the two GETs the screen issues and records every write, so a test can
 * assert what was actually sent — the PUT body in particular, since the
 * handler 400s on any key outside the four editable fields.
 */
function stubFetch(batch: ImportBatch, drafts: ContributionDraft[]) {
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
    } else if (method === "GET" && path.includes("/employers")) {
      body = [];
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

  it("shows progress instead of a draft list while the batch is not ready", async () => {
    const { fetchMock } = stubFetch(makeBatch({ status: "enriching" }), [
      makeDraft(),
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderReview();

    expect(await screen.findByText(/Still working/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Summary")).not.toBeInTheDocument();
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
});
