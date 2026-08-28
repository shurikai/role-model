import { describe, it, expect, vi, afterEach } from "vitest";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Profile } from "./Profile";
import type { Preference, ProficiencyLevel, Skill } from "../lib/types";

function level(value: string, rank: number): ProficiencyLevel {
  return {
    id: `lvl-${value}`,
    value,
    label: value,
    rank,
    aliases: null,
    source: "default",
    sort_order: rank,
  };
}

function skill(over: Partial<Skill> & Pick<Skill, "id" | "name">): Skill {
  return {
    tag_id: `tag-${over.id}`,
    category: "Charting Systems",
    proficiency: "proficient",
    years_experience: null,
    is_active: true,
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...over,
  } as Skill;
}

function preference(
  over: Partial<Preference> & Pick<Preference, "id" | "label">,
): Preference {
  return {
    preference_type: "domain",
    aliases: null,
    sentiment: "positive",
    weight: 5,
    is_hard_gate: false,
    context_type: null,
    notes: null,
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...over,
  } as Preference;
}

/** Records every request so assertions can check what was actually sent. */
function stubFetch({
  skills = [],
  preferences = [],
  levels = [level("novice", 1), level("proficient", 2), level("expert", 3)],
}: {
  skills?: Skill[];
  preferences?: Preference[];
  levels?: ProficiencyLevel[];
}) {
  const calls: { method: string; path: string; body?: unknown }[] = [];
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const path = url.replace(/^.*\/api\/v1/, "");
    const method = init?.method ?? "GET";
    calls.push({
      method,
      path,
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    });

    let body: unknown = null;
    if (method === "GET" && path === "/skills") body = skills;
    else if (method === "GET" && path === "/preferences") body = preferences;
    else if (method === "GET" && path.includes("proficiency-levels"))
      body = levels;
    else body = { id: "new-id" };

    return {
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue(body),
    } as unknown as Response;
  });
  return { fetchMock, calls };
}

function renderProfile() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <Profile />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("Profile", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("groups skills by category and preferences by type", async () => {
    const { fetchMock } = stubFetch({
      skills: [
        skill({ id: "s1", name: "Epic", category: "Charting Systems" }),
        skill({ id: "s2", name: "ACLS", category: "Certifications" }),
      ],
      preferences: [
        preference({
          id: "p1",
          label: "ambulatory quality",
          preference_type: "domain",
        }),
        preference({
          id: "p2",
          label: "inpatient nights",
          preference_type: "dealbreaker",
          sentiment: "negative",
        }),
      ],
    });
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    expect(await screen.findByText("Epic")).toBeInTheDocument();
    expect(screen.getByText("Certifications")).toBeInTheDocument();
    expect(screen.getByText("Charting Systems")).toBeInTheDocument();
    expect(screen.getByText("Domain")).toBeInTheDocument();
    expect(screen.getByText("Dealbreaker")).toBeInTheDocument();
  });

  // The proficiency scale is user-owned rows, not an enum. A form that
  // hardcoded novice/proficient/expert would be guessing at a vocabulary that
  // is per-account by design.
  it("offers the account's own proficiency levels, not a hardcoded scale", async () => {
    const { fetchMock } = stubFetch({
      // The skill's own value must be ON this scale, or the "keep an unknown
      // value selectable" branch below correctly prepends it and this
      // assertion would be measuring that instead.
      skills: [skill({ id: "s1", name: "Epic", proficiency: "competent" })],
      levels: [
        level("learning", 1),
        level("competent", 2),
        level("teaches it", 3),
      ],
    });
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    const select = await screen.findByLabelText("Proficiency for Epic");
    const options = within(select)
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(options).toEqual(["learning", "competent", "teaches it"]);
  });

  // Opening the control must not silently rewrite a value that is no longer on
  // the scale — the row would change depth because someone looked at it.
  it("keeps a proficiency that is not on the current scale selectable", async () => {
    const { fetchMock } = stubFetch({
      skills: [skill({ id: "s1", name: "Epic", proficiency: "wizard" })],
      levels: [level("novice", 1), level("expert", 2)],
    });
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    const select = (await screen.findByLabelText(
      "Proficiency for Epic",
    )) as HTMLSelectElement;
    expect(select.value).toBe("wizard");
    expect(
      within(select)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toContain("wizard");
  });

  it("deactivates a skill without deleting it", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch({
      skills: [skill({ id: "s1", name: "Epic", years_experience: "6.5" })],
    });
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    await user.click(await screen.findByRole("button", { name: "Deactivate" }));

    await waitFor(() => {
      const patch = calls.find((c) => c.method === "PATCH");
      expect(patch).toBeDefined();
      expect(patch!.path).toBe("/skills/s1");
      // Depth and years have to survive a toggle that is not about them.
      expect(patch!.body).toEqual({
        proficiency: "proficient",
        years_experience: 6.5,
        is_active: false,
      });
    });
  });

  // Aliases are what decide whether a preference ever matches a posting, so a
  // row with none is worth flagging rather than rendering as complete.
  it("flags a preference that has no aliases", async () => {
    const { fetchMock } = stubFetch({
      preferences: [
        preference({ id: "p1", label: "remote-first", aliases: null }),
        preference({ id: "p2", label: "night shifts", aliases: ["overnight"] }),
      ],
    });
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    expect(
      await screen.findByText(/matches only this exact wording/),
    ).toBeInTheDocument();
    expect(screen.getByText(/also matches: overnight/)).toBeInTheDocument();
  });

  it("sends a new preference with its aliases split from the comma list", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch({});
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    await user.click(
      await screen.findByRole("button", { name: "Add a preference" }),
    );
    await user.type(screen.getByLabelText("What it is"), "inpatient nights");
    await user.type(
      screen.getByLabelText("Other wordings a posting might use"),
      "night shift, overnight rotation ,  night float ",
    );
    await user.selectOptions(
      screen.getByLabelText("Want or avoid"),
      "negative",
    );
    await user.click(screen.getByRole("button", { name: "Add preference" }));

    await waitFor(() => {
      const post = calls.find(
        (c) => c.method === "POST" && c.path === "/preferences",
      );
      expect(post).toBeDefined();
      expect(post!.body).toMatchObject({
        label: "inpatient nights",
        sentiment: "negative",
        // Trimmed, and the trailing empty segment dropped.
        aliases: ["night shift", "overnight rotation", "night float"],
      });
    });
  });

  // A gate is a named finding, not a veto. The copy has to say so, or the
  // checkbox reads as "hide this job from me".
  it("says a dealbreaker does not block anything", async () => {
    const user = userEvent.setup();
    const { fetchMock } = stubFetch({});
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    await user.click(
      await screen.findByRole("button", { name: "Add a preference" }),
    );
    expect(screen.getByText(/does not block anything/)).toBeInTheDocument();
  });

  it("tells a new account why both lists being empty matters", async () => {
    const { fetchMock } = stubFetch({});
    vi.stubGlobal("fetch", fetchMock);
    renderProfile();

    expect(
      await screen.findByText(
        /preference half of every fit report will be empty/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/No skills claimed yet/)).toBeInTheDocument();
  });
});
