import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "../contexts/AuthContext";
import { Login } from "./Login";

function renderLogin(initialEntries: string[] = ["/login"]) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<Login />} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function jsonResponse(ok: boolean, status: number, body: unknown): Response {
  return {
    ok,
    status,
    json: vi.fn().mockResolvedValue(body),
  } as unknown as Response;
}

describe("Login", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("submits the form with exactly the typed email and password", async () => {
    const user = userEvent.setup();
    // Login succeeding flips isAuthenticated, which also fires AuthContext's
    // background /auth/me verification query — stub a default response for
    // that so only the /auth/login call itself needs asserting on below.
    fetchMock.mockResolvedValue(jsonResponse(true, 200, {}));
    fetchMock.mockResolvedValueOnce(
      jsonResponse(true, 200, { token: "tok", user: { id: "u1", email: "person@example.com" } }),
    );
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "person@example.com");
    await user.type(screen.getByLabelText("Password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Log in" }));

    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url]) => String(url).includes("/auth/login"))).toBe(true);
    });

    const [url, init] = fetchMock.mock.calls.find(([callUrl]) =>
      String(callUrl).includes("/auth/login"),
    )!;
    expect(String(url)).toContain("/auth/login");
    expect(JSON.parse(init.body as string)).toEqual({
      email: "person@example.com",
      password: "hunter2",
    });
  });

  it("shows the verbatim backend message for code: invalid_credentials", async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValueOnce(
      jsonResponse(false, 401, { error: "invalid email or password", code: "invalid_credentials" }),
    );
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "person@example.com");
    await user.type(screen.getByLabelText("Password"), "wrong");
    await user.click(screen.getByRole("button", { name: "Log in" }));

    expect(await screen.findByText("invalid email or password")).toBeInTheDocument();
  });

  it("shows the generic fallback, not the raw message, for code: internal_error", async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValueOnce(
      jsonResponse(false, 500, { error: "panic: nil pointer at auth.go:42", code: "internal_error" }),
    );
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "person@example.com");
    await user.type(screen.getByLabelText("Password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Log in" }));

    expect(await screen.findByText("Something went wrong. Please try again.")).toBeInTheDocument();
    expect(screen.queryByText("panic: nil pointer at auth.go:42")).not.toBeInTheDocument();
  });

  it("renders the expiry banner on ?reason=expired and hides it on dismiss", async () => {
    const user = userEvent.setup();
    renderLogin(["/login?reason=expired"]);

    const banner = screen.getByText("Your session expired. Please log in again.");
    expect(banner).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Dismiss" }));

    expect(screen.queryByText("Your session expired. Please log in again.")).not.toBeInTheDocument();
  });
});
