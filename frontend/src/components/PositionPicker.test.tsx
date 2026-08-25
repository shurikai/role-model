import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { PositionPicker } from "./PositionPicker";
import type { Employer, Position } from "../lib/types";

const EMPLOYER: Employer = {
  id: "emp-1",
  user_id: "u1",
  name: "Continental Freightways",
  industry: null,
  notes: null,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const POSITION: Position = {
  id: "pos-1",
  user_id: "u1",
  employer_id: "emp-1",
  title: "Senior Backend Engineer",
  industry_level: null,
  industry_role: null,
  level_rationale: null,
  started_on: "2018-03-01",
  ended_on: null,
  context_narrative: null,
  location: null,
  sort_order: 0,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

interface Recorded {
  method: string;
  path: string;
  body: unknown;
}

function stubFetch(employers: Employer[], positions: Position[]) {
  const calls: Recorded[] = [];
  const fetchMock = vi.fn(async (url: unknown, init?: RequestInit) => {
    const path = String(url);
    const method = init?.method ?? "GET";
    const body = init?.body ? JSON.parse(String(init.body)) : null;
    calls.push({ method, path, body });

    let responseBody: unknown = null;
    if (method === "GET" && path.includes("/positions")) {
      responseBody = positions;
    } else if (method === "GET") {
      responseBody = employers;
    } else if (path.endsWith("/employers")) {
      responseBody = { ...EMPLOYER, id: "emp-new", name: body.name };
    } else {
      responseBody = {
        ...POSITION,
        id: "pos-new",
        employer_id: body.employer_id,
        title: body.title,
      };
    }

    return {
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue(responseBody),
    } as unknown as Response;
  });
  return { fetchMock, calls };
}

function renderPicker(
  props: Partial<Parameters<typeof PositionPicker>[0]> = {},
) {
  const onResolved = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <PositionPicker
        suggestedEmployerName="Nimbus Health"
        suggestedPositionTitle="Staff Engineer"
        onResolved={onResolved}
        {...props}
      />
    </QueryClientProvider>,
  );
  return { onResolved };
}

describe("PositionPicker", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("creates the employer and the position when neither exists", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch([], []);
    vi.stubGlobal("fetch", fetchMock);
    const { onResolved } = renderPicker();

    await user.click(
      await screen.findByRole("button", { name: /create new/i }),
    );

    // Pre-filled from the draft, so the empty-account case is a confirm
    // rather than a blank form.
    expect(screen.getByLabelText("Employer name")).toHaveValue("Nimbus Health");
    expect(screen.getByLabelText("Position title")).toHaveValue(
      "Staff Engineer",
    );

    await user.type(screen.getByLabelText("Started on"), "2021-06-01");
    await user.click(screen.getByRole("button", { name: /Create and attach/ }));

    await waitFor(() => expect(onResolved).toHaveBeenCalledWith("pos-new"));

    const employerPost = calls.find(
      (c) => c.method === "POST" && c.path.endsWith("/employers"),
    );
    const positionPost = calls.find(
      (c) => c.method === "POST" && c.path.endsWith("/positions"),
    );
    expect(employerPost!.body).toEqual({ name: "Nimbus Health" });
    // The position must hang off the employer that was just created, not off
    // a stale or empty id.
    expect(positionPost!.body).toMatchObject({
      employer_id: "emp-new",
      title: "Staff Engineer",
      started_on: "2021-06-01",
      ended_on: null,
    });
  });

  it("refuses to submit without a start date, and writes nothing", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch([], []);
    vi.stubGlobal("fetch", fetchMock);
    renderPicker();

    await user.click(
      await screen.findByRole("button", { name: /create new/i }),
    );
    await user.click(screen.getByRole("button", { name: /Create and attach/ }));

    expect(
      await screen.findByText("A start date is required."),
    ).toBeInTheDocument();
    // The backend parses started_on strictly and 400s on an empty string;
    // catching it here keeps a required-field mistake from reading as a
    // server error.
    expect(calls.filter((c) => c.method === "POST")).toHaveLength(0);
  });

  it("resolves an existing position without creating anything", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch([EMPLOYER], [POSITION]);
    vi.stubGlobal("fetch", fetchMock);
    const { onResolved } = renderPicker({
      suggestedEmployerName: EMPLOYER.name,
      suggestedPositionTitle: POSITION.title,
    });

    // The name match pre-selects the employer, so its positions load without
    // a click.
    await user.click(
      await screen.findByRole("button", { name: /Senior Backend Engineer/ }),
    );

    expect(onResolved).toHaveBeenCalledWith("pos-1");
    expect(calls.filter((c) => c.method === "POST")).toHaveLength(0);
  });
});
