import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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
import { AuthProvider } from "../contexts/AuthContext";
import { RequireAuth } from "./RequireAuth";
import { setSession, getSession } from "../lib/session";

function renderAt(path: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<p>Login page</p>} />
            <Route element={<RequireAuth />}>
              <Route path="/career" element={<p>Career page</p>} />
              <Route path="/career/add" element={<p>Add page</p>} />
              <Route
                path="/career/positions/:id"
                element={<p>Position page</p>}
              />
              <Route path="/applications" element={<p>Applications page</p>} />
              <Route path="/applications/new" element={<p>New page</p>} />
              <Route path="/import/new" element={<p>Narrow import</p>} />
              <Route path="/onboarding/new" element={<p>Onboarding</p>} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AppShell", () => {
  beforeEach(() => {
    setSession({
      token: "a-token",
      user: { id: "u1", email: "nurse@example.com" },
    });
    // AuthProvider revalidates the stored session against /auth/me on mount.
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: vi
          .fn()
          .mockResolvedValue({ id: "u1", email: "nurse@example.com" }),
      } as unknown as Response),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
    cleanup();
  });

  // logout() had been defined on AuthContext since the auth shell landed and
  // was called by nothing, so a signed-in user could not sign out except by
  // clearing localStorage by hand.
  it("signs the user out and clears the stored session", async () => {
    const user = userEvent.setup();
    renderAt("/applications");

    await screen.findByText("Applications page");
    expect(getSession()).not.toBeNull();

    await user.click(screen.getByRole("button", { name: "Sign out" }));

    await waitFor(() => {
      expect(screen.getByText("Login page")).toBeInTheDocument();
    });
    expect(getSession()).toBeNull();
  });

  // Three areas plus one action. The on-ramps used to be three of five
  // top-level links; they are reached through /career/add now.
  it("links to the three areas and the add hub, and no on-ramp", async () => {
    renderAt("/applications");
    await screen.findByText("Applications page");

    const nav = screen.getByRole("navigation");
    const navHrefs = within(nav)
      .getAllByRole("link")
      .map((a) => a.getAttribute("href"));
    expect(navHrefs).toEqual(["/career", "/applications", "/profile"]);
    expect(
      screen.getByRole("link", { name: "Skills & preferences" }),
    ).toHaveAttribute("href", "/profile");
    expect(
      screen.getByRole("link", { name: "+ Add to career" }),
    ).toHaveAttribute("href", "/career/add");

    const allHrefs = screen
      .getAllByRole("link")
      .map((a) => a.getAttribute("href"));
    for (const onRamp of [
      "/import/career/new",
      "/import/new",
      "/onboarding/new",
    ]) {
      expect(allHrefs).not.toContain(onRamp);
    }
  });

  // The links used to carry `end`, which would drop Career's current state
  // on any nested career route.
  it("keeps Career current on a nested career route", async () => {
    renderAt("/career/positions/p1");
    await screen.findByText("Position page");

    expect(screen.getByRole("link", { name: "Career" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByRole("link", { name: "Applications" }),
    ).not.toHaveAttribute("aria-current");
  });

  // The on-ramps write to the career, so opening one keeps Career current.
  it("keeps Career current inside an on-ramp", async () => {
    renderAt("/import/new");
    await screen.findByText("Narrow import");

    expect(screen.getByRole("link", { name: "Career" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("keeps Applications current on a nested applications route", async () => {
    renderAt("/applications/new");
    await screen.findByText("New page");

    expect(screen.getByRole("link", { name: "Applications" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: "Career" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("does not render for an unauthenticated visitor", async () => {
    localStorage.clear();
    renderAt("/applications");

    await waitFor(() => {
      expect(screen.getByText("Login page")).toBeInTheDocument();
    });
    expect(
      screen.queryByRole("button", { name: "Sign out" }),
    ).not.toBeInTheDocument();
  });
});
