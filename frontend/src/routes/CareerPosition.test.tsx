import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CareerPosition } from "./CareerPosition";
import type { CareerTree, Contribution } from "../lib/types";
import {
  makeCareerEmployer,
  makeCareerPosition,
  makeContribution,
} from "../test/career";

const tree: CareerTree = {
  employers: [
    makeCareerEmployer({
      id: "e1",
      name: "Zephyr Health",
      positions: [
        makeCareerPosition({
          id: "p1",
          title: "RN III",
          industry_role: "Charge Nurse",
          started_on: "2020-02-01",
          location: "Portland, OR",
        }),
      ],
    }),
  ],
};

function stub(contributions: Contribution[]) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: unknown) => {
      const body = String(url).endsWith("/career") ? tree : contributions;
      return {
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue(body),
      } as unknown as Response;
    }),
  );
}

function renderAt(positionID: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/career/positions/${positionID}`]}>
        <Routes>
          <Route
            path="/career/positions/:positionID"
            element={<CareerPosition />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CareerPosition", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("shows the position under a breadcrumb back to the career", async () => {
    stub([]);
    renderAt("p1");

    expect(
      await screen.findByRole("heading", { level: 1, name: "RN III" }),
    ).toBeInTheDocument();
    const crumbs = screen.getByRole("navigation", { name: "Breadcrumb" });
    expect(crumbs).toHaveTextContent("Career / Zephyr Health / RN III");
    expect(
      within(crumbs).getByRole("link", { name: "Career" }),
    ).toHaveAttribute("href", "/career");
    expect(
      screen.getByText(/Zephyr Health · Feb 2020 – Present · Portland, OR/),
    ).toBeInTheDocument();
    // The normalized role is what a resume prints, so it is shown.
    expect(screen.getByText("Charge Nurse")).toBeInTheDocument();
    expect(
      await screen.findByText(/Nothing recorded for this role yet/),
    ).toBeInTheDocument();
  });

  it("renders tags as chips and expands the detail on request", async () => {
    stub([
      makeContribution({
        id: "c1",
        summary: "Cut handoff errors by a third.",
        full_description: "Wrote a structured shift-report template.",
        outcomes: "Handoff incidents fell from 12 to 8 a quarter.",
        tags: [
          { id: "t1", name: "Patient safety", category: "Clinical" },
          { id: "t2", name: "Epic", category: "Systems" },
        ],
      }),
    ]);
    renderAt("p1");

    const card = (
      await screen.findByText("Cut handoff errors by a third.")
    ).closest("article") as HTMLElement;
    const chips = within(card).getByRole("list", { name: "Tags" });
    expect(
      within(chips)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(["Patient safety", "Epic"]);

    expect(within(card).queryByText(/structured shift-report/)).toBeNull();
    await userEvent.click(
      within(card).getByRole("button", { name: "More detail" }),
    );
    expect(
      within(card).getByText(/structured shift-report/),
    ).toBeInTheDocument();
    expect(within(card).getByText(/fell from 12 to 8/)).toBeInTheDocument();
    // An empty field gets no heading.
    expect(within(card).queryByText("Scale")).toBeNull();
  });

  it("folds hidden contributions under their own heading", async () => {
    stub([
      makeContribution({ id: "c1", summary: "Shown on resumes." }),
      makeContribution({
        id: "c2",
        summary: "Kept but hidden.",
        is_active: false,
      }),
    ]);
    renderAt("p1");

    expect(await screen.findByText("Shown on resumes.")).toBeInTheDocument();
    expect(screen.queryByText("Kept but hidden.")).toBeNull();

    await userEvent.click(
      screen.getByRole("button", { name: /Hidden from resumes \(1\)/ }),
    );
    expect(screen.getByText("Kept but hidden.")).toBeInTheDocument();
  });

  it("says so when the position is not in the career", async () => {
    stub([]);
    renderAt("nope");

    expect(
      await screen.findByRole("heading", { name: "Role not found" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Back to your career" }),
    ).toHaveAttribute("href", "/career");
  });
});
