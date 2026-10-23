import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CareerAdd } from "./CareerAdd";
import type { Employer, ImportBatchSummary, Position } from "../lib/types";

function makeEmployer(id: string): Employer {
  return {
    id,
    user_id: "u1",
    name: `Employer ${id}`,
    industry: null,
    notes: null,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
  };
}

function makePosition(employerID: string): Position {
  return {
    id: `position-of-${employerID}`,
    user_id: "u1",
    employer_id: employerID,
    title: "Charge Nurse",
    industry_level: null,
    industry_role: null,
    level_rationale: null,
    started_on: "2020-01-01",
    ended_on: null,
    context_narrative: null,
    location: null,
    sort_order: 0,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
  };
}

function makeBatch(
  overrides: Partial<ImportBatchSummary> = {},
): ImportBatchSummary {
  return {
    id: "batch-1",
    status: "review",
    created_at: "2026-10-20T00:00:00Z",
    kind: "career",
    pending_count: 3,
    ...overrides,
  };
}

function stubFetch({
  employers = [],
  positionsByEmployer = {},
  batches = [],
}: {
  employers?: Employer[];
  positionsByEmployer?: Record<string, Position[]>;
  batches?: ImportBatchSummary[];
}) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: unknown) => {
      const path = String(url);
      let body: unknown = [];
      if (path.endsWith("/import")) {
        body = batches;
      } else if (path.endsWith("/career")) {
        body = {
          employers: employers.map((e) => ({
            ...e,
            positions: (positionsByEmployer[e.id] ?? []).map((p) => ({
              ...p,
              contribution_counts: { active: 0, hidden: 0 },
            })),
          })),
        };
      }
      return {
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue(body),
      } as unknown as Response;
    }),
  );
}

function renderHub() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CareerAdd />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CareerAdd", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("offers the three on-ramps by what the person has to hand", async () => {
    stubFetch({
      employers: [makeEmployer("e1")],
      positionsByEmployer: { e1: [makePosition("e1")] },
    });
    renderHub();

    expect(
      screen.getByRole("link", { name: /I have a resume or CV/ }),
    ).toHaveAttribute("href", "/import/career/new");
    expect(
      screen.getByRole("link", { name: /I'd rather talk it through/ }),
    ).toHaveAttribute("href", "/onboarding/new");
    expect(
      await screen.findByRole("link", {
        name: /Add accomplishments to a job already here/,
      }),
    ).toHaveAttribute("href", "/import/new");
  });

  // The narrow path files contributions under an existing position, so with
  // none there is nothing for it to do.
  it("disables adding accomplishments when there are no positions", async () => {
    stubFetch({});
    renderHub();

    expect(await screen.findByText(/Add a job first/)).toBeInTheDocument();
    expect(
      screen.queryByRole("link", {
        name: /Add accomplishments to a job already here/,
      }),
    ).not.toBeInTheDocument();
  });

  // An employer with no positions — a career import that approved the
  // employer and rejected its roles — is still nothing to add to.
  it("disables it when the only employer has no positions", async () => {
    stubFetch({ employers: [makeEmployer("e1")] });
    renderHub();

    expect(await screen.findByText(/Add a job first/)).toBeInTheDocument();
  });

  it("enables it when any employer has a position", async () => {
    stubFetch({
      employers: [makeEmployer("e1"), makeEmployer("e2")],
      positionsByEmployer: { e2: [makePosition("e2")] },
    });
    renderHub();

    expect(
      await screen.findByRole("link", {
        name: /Add accomplishments to a job already here/,
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Add a job first/)).not.toBeInTheDocument();
  });

  it("links unfinished imports to the review screen for their kind", async () => {
    stubFetch({
      batches: [
        makeBatch({ id: "career-batch", kind: "career", pending_count: 4 }),
        makeBatch({
          id: "narrow-batch",
          kind: "contributions",
          pending_count: 1,
        }),
      ],
    });
    renderHub();

    expect(
      await screen.findByRole("heading", { name: "Unfinished imports" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /4 to review/ })).toHaveAttribute(
      "href",
      "/import/career/career-batch",
    );
    expect(screen.getByRole("link", { name: /1 to review/ })).toHaveAttribute(
      "href",
      "/import/narrow-batch",
    );
  });

  // Finished batches and ones that never staged a draft have no review left.
  it("leaves out batches with nothing pending", async () => {
    stubFetch({
      batches: [
        makeBatch({ id: "done", pending_count: 0, status: "complete" }),
        makeBatch({ id: "failed", pending_count: 0, status: "failed" }),
      ],
    });
    renderHub();

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith(
        expect.stringMatching(/\/import$/),
        expect.anything(),
      );
    });
    expect(
      screen.queryByRole("heading", { name: "Unfinished imports" }),
    ).not.toBeInTheDocument();
  });
});
