import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "../contexts/AuthContext";
import { Signup } from "./Signup";

function renderSignup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/signup"]}>
        <AuthProvider>
          <Routes>
            <Route path="/signup" element={<Signup />} />
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

describe("Signup", () => {
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
    // Signup succeeding flips isAuthenticated, which also fires AuthContext's
    // background /auth/me verification query — stub a default response for
    // that so only the /auth/signup call itself needs asserting on below.
    fetchMock.mockResolvedValue(jsonResponse(true, 200, {}));
    fetchMock.mockResolvedValueOnce(
      jsonResponse(true, 201, { token: "tok", user: { id: "u1", email: "new@example.com" } }),
    );
    renderSignup();

    await user.type(screen.getByLabelText("Email"), "new@example.com");
    await user.type(screen.getByLabelText("Password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Sign up" }));

    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url]) => String(url).includes("/auth/signup"))).toBe(true);
    });

    const [url, init] = fetchMock.mock.calls.find(([callUrl]) =>
      String(callUrl).includes("/auth/signup"),
    )!;
    expect(String(url)).toContain("/auth/signup");
    expect(JSON.parse(init.body as string)).toEqual({
      email: "new@example.com",
      password: "hunter2",
    });
  });

  it("shows the verbatim backend message for code: invalid_credentials", async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValueOnce(
      jsonResponse(false, 409, { error: "invalid email or password", code: "invalid_credentials" }),
    );
    renderSignup();

    await user.type(screen.getByLabelText("Email"), "new@example.com");
    await user.type(screen.getByLabelText("Password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Sign up" }));

    expect(await screen.findByText("invalid email or password")).toBeInTheDocument();
  });

  it("shows the generic fallback, not the raw message, for code: internal_error", async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValueOnce(
      jsonResponse(false, 500, { error: "panic: nil pointer at auth.go:42", code: "internal_error" }),
    );
    renderSignup();

    await user.type(screen.getByLabelText("Email"), "new@example.com");
    await user.type(screen.getByLabelText("Password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Sign up" }));

    expect(await screen.findByText("Something went wrong. Please try again.")).toBeInTheDocument();
    expect(screen.queryByText("panic: nil pointer at auth.go:42")).not.toBeInTheDocument();
  });
});
