import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Career } from "./Career";
import type { CareerTree } from "../lib/types";
import { makeCareerEmployer, makeCareerPosition } from "../test/career";

function stubCareer(tree: CareerTree) {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        ({
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue(tree),
        }) as unknown as Response,
    ),
  );
}

function renderCareer() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <Career />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("Career", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("points an empty career at Add to career", async () => {
    stubCareer({ employers: [] });
    renderCareer();

    expect(await screen.findByText("Nothing here yet")).toBeInTheDocument();
    const adds = screen.getAllByRole("link", { name: "+ Add to career" });
    expect(adds.length).toBe(2);
    for (const a of adds) expect(a).toHaveAttribute("href", "/career/add");
  });

  // A loading account must not be told it has no career, even briefly.
  it("does not show the empty state before the tree has loaded", () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    renderCareer();

    expect(screen.getByText(/Loading your career/)).toBeInTheDocument();
    expect(screen.queryByText("Nothing here yet")).not.toBeInTheDocument();
  });

  it("lists employers with their positions, linked, in the order sent", async () => {
    stubCareer({
      employers: [
        // Deliberately not alphabetical: the server's resume order is kept.
        makeCareerEmployer({
          id: "e-current",
          name: "Zephyr Health",
          industry: "Hospital system",
          positions: [
            makeCareerPosition({
              id: "p-lead",
              title: "Nurse Manager",
              started_on: "2023-04-01",
              location: "Portland, OR",
              contribution_counts: { active: 9, hidden: 1 },
            }),
            makeCareerPosition({
              id: "p-charge",
              title: "Charge Nurse",
              started_on: "2020-02-01",
              ended_on: "2023-03-31",
              contribution_counts: { active: 1, hidden: 0 },
            }),
          ],
        }),
        makeCareerEmployer({ id: "e-old", name: "Acme Clinic" }),
      ],
    });
    renderCareer();

    const headings = await screen.findAllByRole("heading", { level: 2 });
    expect(headings.map((h) => h.textContent)).toEqual([
      "Zephyr Health",
      "Acme Clinic",
    ]);

    const zephyr = screen.getByRole("region", { name: "Zephyr Health" });
    const rows = within(zephyr).getAllByRole("link");
    expect(rows.map((r) => r.getAttribute("href"))).toEqual([
      "/career/positions/p-lead",
      "/career/positions/p-charge",
    ]);
    expect(rows[0]).toHaveTextContent("Nurse Manager");
    expect(rows[0]).toHaveTextContent("Apr 2023 – Present · Portland, OR");
    expect(rows[0]).toHaveTextContent("9 contributions · 1 hidden");
    expect(rows[1]).toHaveTextContent("Feb 2020 – Mar 2023");
    expect(rows[1]).toHaveTextContent("1 contribution");
    expect(rows[1]).not.toHaveTextContent("hidden");

    // An employer kept with every role rejected still shows, and says so.
    const acme = screen.getByRole("region", { name: "Acme Clinic" });
    expect(within(acme).getByText(/No roles recorded/)).toBeInTheDocument();
    expect(screen.queryByText("Nothing here yet")).not.toBeInTheDocument();
  });
});
