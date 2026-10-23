import { describe, it, expect, vi, afterEach } from "vitest";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Career } from "./Career";
import { CareerPosition } from "./CareerPosition";
import type { CareerLevel, CareerTree, Contribution } from "../lib/types";
import {
  makeCareerEmployer,
  makeCareerPosition,
  makeContribution,
} from "../test/career";

interface Recorded {
  method: string;
  path: string;
  body: unknown;
}

const levels: CareerLevel[] = [
  { id: "l1", value: "entry", label: "Entry", rank: 1, sort_order: 0 },
  { id: "l2", value: "senior", label: "Senior", rank: 3, sort_order: 2 },
];

/**
 * Serves GETs from fixed data and records every write. A write answers with
 * whatever `writes` says for its method + path, defaulting to a 200 with an
 * echo of the body (or a 204 for DELETE).
 */
function stub({
  tree,
  contributions = [],
  writes = {},
}: {
  tree: CareerTree;
  contributions?: Contribution[];
  writes?: Record<string, { status: number; body: unknown }>;
}) {
  const calls: Recorded[] = [];
  const fetchMock = vi.fn(async (url: unknown, init?: RequestInit) => {
    const path = String(url).replace(/^.*\/api\/v1/, "");
    const method = init?.method ?? "GET";
    const body = init?.body ? JSON.parse(String(init.body)) : null;
    if (method !== "GET") calls.push({ method, path, body });

    const respond = (status: number, json: unknown) =>
      ({
        ok: status < 400,
        status,
        json: vi.fn().mockResolvedValue(json),
      }) as unknown as Response;

    if (method === "GET") {
      if (path === "/career") return respond(200, tree);
      if (path === "/vocabulary/career-levels") return respond(200, levels);
      if (path.endsWith("/contributions")) return respond(200, contributions);
      return respond(200, []);
    }
    const override = writes[`${method} ${path}`];
    if (override) return respond(override.status, override.body);
    if (method === "DELETE") return respond(204, undefined);
    // Mutations read employer_id off a position response to invalidate.
    return respond(200, { employer_id: "e1", ...body });
  });
  vi.stubGlobal("fetch", fetchMock);
  return calls;
}

function renderAt(path: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/career" element={<Career />} />
          <Route
            path="/career/positions/:positionID"
            element={<CareerPosition />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const position = makeCareerPosition({
  id: "p1",
  employer_id: "e1",
  title: "RN III",
  industry_level: "senior",
  location: "Portland, OR",
  started_on: "2020-02-15",
  ended_on: "2023-03-31",
  // None of these three has a visible effect on the edits below, and all of
  // them must survive a save.
  sort_order: 4,
  level_rationale: "Charge duties on every shift.",
  context_narrative: "24-bed med-surg unit.",
});

function treeWith(
  ...positions: ReturnType<typeof makeCareerPosition>[]
): CareerTree {
  return {
    employers: [
      makeCareerEmployer({
        id: "e1",
        name: "Zephyr Health",
        industry: "Hospital system",
        notes: "Left for a management role.",
        positions,
      }),
    ],
  };
}

describe("editing an employer", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("sends the whole employer, not just the field that changed", async () => {
    const user = userEvent.setup();
    const calls = stub({ tree: treeWith(position) });
    renderAt("/career");

    await user.click(
      await screen.findByRole("button", { name: "Edit Zephyr Health" }),
    );
    const form = screen.getByRole("form", { name: "Edit employer" });
    const name = within(form).getByLabelText("Name");
    await user.clear(name);
    await user.type(name, "Zephyr Health Partners");
    await user.click(
      within(form).getByRole("button", { name: "Save employer" }),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({
      method: "PATCH",
      path: "/employers/e1",
      body: {
        name: "Zephyr Health Partners",
        industry: "Hospital system",
        notes: "Left for a management role.",
      },
    });
    await waitFor(() =>
      expect(
        screen.queryByRole("form", { name: "Edit employer" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("offers delete only for an employer with no positions", async () => {
    stub({ tree: treeWith(position) });
    renderAt("/career");

    await screen.findByRole("button", { name: "Edit Zephyr Health" });
    expect(
      screen.queryByRole("button", { name: "Delete Zephyr Health" }),
    ).not.toBeInTheDocument();
  });

  it("deletes after confirming, and surfaces a 409 in the server's words", async () => {
    const user = userEvent.setup();
    const calls = stub({
      tree: treeWith(),
      writes: {
        "DELETE /employers/e1": {
          status: 409,
          body: {
            error: "employer has positions and cannot be deleted",
            code: "has_dependents",
          },
        },
      },
    });
    renderAt("/career");

    await user.click(
      await screen.findByRole("button", { name: "Delete Zephyr Health" }),
    );
    const dialog = screen.getByRole("alertdialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Delete employer" }),
    );

    expect(calls).toEqual([
      { method: "DELETE", path: "/employers/e1", body: null },
    ]);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "employer has positions and cannot be deleted",
    );
  });

  it("adds a position under the employer", async () => {
    const user = userEvent.setup();
    const calls = stub({ tree: treeWith() });
    renderAt("/career");

    await user.click(
      await screen.findByRole("button", {
        name: "Add a position at Zephyr Health",
      }),
    );
    const form = screen.getByRole("form", { name: "New position" });
    await user.type(within(form).getByLabelText(/^Title/), "Staff Nurse");
    // jsdom has no month picker; typing the value is what a browser submits.
    await user.type(within(form).getByLabelText("Started"), "2018-06");
    await user.click(
      within(form).getByRole("button", { name: "Add position" }),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0].method).toBe("POST");
    expect(calls[0].path).toBe("/positions");
    expect(calls[0].body).toMatchObject({
      employer_id: "e1",
      title: "Staff Nurse",
      started_on: "2018-06-01",
      ended_on: null,
    });
  });
});

describe("editing a position", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("keeps fields the form leaves alone, including the stored day", async () => {
    const user = userEvent.setup();
    const calls = stub({ tree: treeWith(position) });
    renderAt("/career/positions/p1");

    await user.click(
      await screen.findByRole("button", { name: "Edit position" }),
    );
    const form = screen.getByRole("form", { name: "Edit position" });
    const location = within(form).getByLabelText("Location");
    await user.clear(location);
    await user.type(location, "Seattle, WA");
    await user.click(
      within(form).getByRole("button", { name: "Save position" }),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({
      method: "PATCH",
      path: "/positions/p1",
      body: {
        title: "RN III",
        industry_level: "senior",
        industry_role: null,
        location: "Seattle, WA",
        level_rationale: "Charge duties on every shift.",
        // Untouched months keep the day they were stored with.
        started_on: "2020-02-15",
        ended_on: "2023-03-31",
        context_narrative: "24-bed med-surg unit.",
        // Not on the form at all.
        sort_order: 4,
      },
    });
  });

  it("refuses an end before the start without calling the server", async () => {
    const user = userEvent.setup();
    const calls = stub({ tree: treeWith(position) });
    renderAt("/career/positions/p1");

    await user.click(
      await screen.findByRole("button", { name: "Edit position" }),
    );
    const form = screen.getByRole("form", { name: "Edit position" });
    const ended = within(form).getByLabelText(/^Ended/);
    await user.clear(ended);
    await user.type(ended, "2019-01");
    await user.click(
      within(form).getByRole("button", { name: "Save position" }),
    );

    expect(await within(form).findByRole("alert")).toHaveTextContent(
      "before the start",
    );
    expect(calls).toHaveLength(0);
  });

  it("offers delete only for a position with nothing under it", async () => {
    stub({
      tree: treeWith({
        ...position,
        contribution_counts: { active: 0, hidden: 1 },
      }),
      contributions: [makeContribution({ is_active: false })],
    });
    renderAt("/career/positions/p1");

    await screen.findByRole("button", { name: "Edit position" });
    expect(
      screen.queryByRole("button", { name: "Delete position" }),
    ).not.toBeInTheDocument();
  });

  it("surfaces a 409 on position delete", async () => {
    const user = userEvent.setup();
    const calls = stub({
      tree: treeWith(position),
      writes: {
        "DELETE /positions/p1": {
          status: 409,
          body: {
            error: "position has contributions and cannot be deleted",
            code: "has_dependents",
          },
        },
      },
    });
    renderAt("/career/positions/p1");

    await user.click(
      await screen.findByRole("button", { name: "Delete position" }),
    );
    await user.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: "Delete position",
      }),
    );

    expect(calls).toEqual([
      { method: "DELETE", path: "/positions/p1", body: null },
    ]);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "position has contributions and cannot be deleted",
    );
  });
});

describe("editing a contribution", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  const contribution = makeContribution({
    id: "c1",
    summary: "Cut handoff errors by a third.",
    full_description: "Wrote a structured shift-report template.",
    outcomes: "Incidents fell from 12 to 8 a quarter.",
    scale_context: "Four units, 120 nurses.",
  });

  function card() {
    return screen
      .getByText("Cut handoff errors by a third.")
      .closest("article") as HTMLElement;
  }

  it("hides in one click, with no confirm and the text untouched", async () => {
    const user = userEvent.setup();
    const calls = stub({
      tree: treeWith(position),
      contributions: [contribution],
    });
    renderAt("/career/positions/p1");

    await screen.findByText("Cut handoff errors by a third.");
    await user.click(
      within(card()).getByRole("button", { name: "Hide from resumes" }),
    );

    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({
      method: "PATCH",
      path: "/contributions/c1",
      body: {
        summary: "Cut handoff errors by a third.",
        full_description: "Wrote a structured shift-report template.",
        outcomes: "Incidents fell from 12 to 8 a quarter.",
        scale_context: "Four units, 120 nurses.",
        is_active: false,
      },
    });
  });

  it("shows a hidden one on resumes again", async () => {
    const user = userEvent.setup();
    const calls = stub({
      tree: treeWith(position),
      contributions: [{ ...contribution, is_active: false }],
    });
    renderAt("/career/positions/p1");

    await user.click(
      await screen.findByRole("button", { name: /Hidden from resumes \(1\)/ }),
    );
    await user.click(
      within(card()).getByRole("button", { name: "Show on resumes" }),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0].body).toMatchObject({ is_active: true });
  });

  it("edits in place and keeps is_active as it was", async () => {
    const user = userEvent.setup();
    const calls = stub({
      tree: treeWith(position),
      contributions: [{ ...contribution, is_active: false }],
    });
    renderAt("/career/positions/p1");

    await user.click(
      await screen.findByRole("button", { name: /Hidden from resumes \(1\)/ }),
    );
    await user.click(within(card()).getByRole("button", { name: "Edit" }));
    const form = screen.getByRole("form", { name: "Edit contribution" });
    const outcomes = within(form).getByLabelText(/^Outcomes/);
    await user.clear(outcomes);
    await user.click(
      within(form).getByRole("button", { name: "Save contribution" }),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({
      method: "PATCH",
      path: "/contributions/c1",
      body: {
        summary: "Cut handoff errors by a third.",
        full_description: "Wrote a structured shift-report template.",
        // Cleared means none, not an empty string.
        outcomes: null,
        scale_context: "Four units, 120 nurses.",
        is_active: false,
      },
    });
  });

  it("deletes only through the confirm, which says it is permanent", async () => {
    const user = userEvent.setup();
    const calls = stub({
      tree: treeWith(position),
      contributions: [contribution],
    });
    renderAt("/career/positions/p1");

    await screen.findByText("Cut handoff errors by a third.");
    await user.click(within(card()).getByRole("button", { name: "Delete" }));
    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent("cannot be undone");

    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(calls).toHaveLength(0);

    await user.click(within(card()).getByRole("button", { name: "Delete" }));
    await user.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: "Delete permanently",
      }),
    );
    expect(calls).toEqual([
      { method: "DELETE", path: "/contributions/c1", body: null },
    ]);
  });

  it("adds a contribution to the position", async () => {
    const user = userEvent.setup();
    const calls = stub({ tree: treeWith(position) });
    renderAt("/career/positions/p1");

    await user.click(
      await screen.findByRole("button", { name: "+ Add contribution" }),
    );
    const form = screen.getByRole("form", { name: "New contribution" });
    await user.type(
      within(form).getByLabelText(/^Summary/),
      "Trained new hires.",
    );
    await user.type(
      within(form).getByLabelText("Full description"),
      "Ran orientation for 14 new graduates.",
    );
    await user.click(
      within(form).getByRole("button", { name: "Add contribution" }),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({
      method: "POST",
      path: "/contributions",
      body: {
        position_id: "p1",
        summary: "Trained new hires.",
        full_description: "Ran orientation for 14 new graduates.",
        outcomes: null,
        scale_context: null,
      },
    });
  });
});
