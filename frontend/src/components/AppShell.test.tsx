import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
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
              <Route path="/applications" element={<p>Applications page</p>} />
              <Route path="/applications/new" element={<p>New page</p>} />
              <Route path="/import/new" element={<p>Narrow import</p>} />
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

  // The narrow import path had no link from anywhere and was reachable only
  // by typing its URL.
  it("links to both import paths and to the applications list", async () => {
    renderAt("/applications");
    await screen.findByText("Applications page");

    const hrefs = screen
      .getAllByRole("link")
      .map((a) => a.getAttribute("href"));
    for (const href of ["/applications", "/import/career/new", "/import/new"]) {
      expect(hrefs).toContain(href);
    }
  });

  // `end` on every NavLink: the routes nest, so /applications would otherwise
  // stay marked current while /applications/new is open.
  it("marks only the open route as current", async () => {
    renderAt("/applications/new");
    await screen.findByText("New page");

    const applications = screen.getByRole("link", { name: "Applications" });
    expect(applications.className).toContain("border-transparent");
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
