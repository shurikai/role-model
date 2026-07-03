import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route, useLocation } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "../contexts/AuthContext";
import { RequireAuth } from "../components/RequireAuth";
import { setSession } from "../lib/session";

function LocationProbe() {
  const location = useLocation();
  return (
    <div>
      <div data-testid="pathname">{location.pathname}</div>
      <div data-testid="search">{location.search}</div>
    </div>
  );
}

function renderProtectedApp() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/dashboard"]}>
        <AuthProvider>
          <Routes>
            <Route element={<RequireAuth />}>
              <Route path="/dashboard" element={<div>Protected content</div>} />
            </Route>
            <Route path="/login" element={<LocationProbe />} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("session expiry redirect", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("lands on /login with both redirect and reason=expired when the background session check 401s, with no dropped param from a competing navigation", async () => {
    setSession({ token: "tok", user: { id: "u1", email: "a@example.com" } });
    // AuthProvider's background `/auth/me` verification query fires this,
    // the same in-flight call that would 401 in production on token expiry.
    fetchMock.mockResolvedValue({
      ok: false,
      status: 401,
      json: vi.fn().mockResolvedValue({ error: "unauthorized", code: "unauthorized" }),
    } as unknown as Response);

    renderProtectedApp();

    expect(screen.getByText("Protected content")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByTestId("pathname")).toHaveTextContent("/login");
    });

    const search = new URLSearchParams(screen.getByTestId("search").textContent ?? "");
    expect(search.get("redirect")).toBe("/dashboard");
    expect(search.get("reason")).toBe("expired");
  });
});
