import { describe, it, expect, vi, afterEach } from "vitest";
import { StrictMode } from "react";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Onboarding } from "./Onboarding";
import type { OnboardingTurnResponse } from "../lib/types";

interface Recorded {
  body: unknown;
}

/**
 * Returns one canned response per POST, in order — the mount-time "start a
 * new interview" call and every subsequent submit both hit the same
 * endpoint, so the test scripts the sequence rather than branching on path.
 */
function stubFetch(responses: OnboardingTurnResponse[]) {
  const calls: Recorded[] = [];
  const queue = [...responses];
  const fetchMock = vi.fn(async (_url: unknown, init?: RequestInit) => {
    calls.push({ body: init?.body ? JSON.parse(String(init.body)) : null });
    const body = queue.shift();
    if (!body) {
      return {
        ok: false,
        status: 500,
        json: vi.fn().mockResolvedValue({
          error: "no more turns",
          code: "internal_error",
        }),
      } as unknown as Response;
    }
    return {
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue(body),
    } as unknown as Response;
  });
  return { fetchMock, calls };
}

function renderOnboarding() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/onboarding/new"]}>
        <Routes>
          <Route path="/onboarding/new" element={<Onboarding />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

// Deliberately separate from renderOnboarding: the real app renders under
// <StrictMode> (main.tsx) and this suite otherwise never does, which is
// exactly the gap that let the mount-time turn ship as a useMutation fired
// from a useEffect — every other test here passed, and the button still
// hung on "Sending…" forever in a real browser. StrictMode's dev-only
// double-invoke of effects is what exposed it: useMutation's pending state
// never resolved, even though the request itself completed. See
// useStartOnboarding's docstring for the fix (a useQuery instead).
function renderOnboardingInStrictMode() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={["/onboarding/new"]}>
          <Routes>
            <Route path="/onboarding/new" element={<Onboarding />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </StrictMode>,
  );
}

describe("Onboarding", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  it("asks the first question on mount, with no session id yet", async () => {
    const { fetchMock, calls } = stubFetch([
      { session_id: "s1", reply: "What company did you work at?", done: false },
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderOnboarding();

    expect(
      await screen.findByText("What company did you work at?"),
    ).toBeInTheDocument();
    expect(calls[0].body).toEqual({});
  });

  it("still resolves the mount-time turn under StrictMode, exactly once", async () => {
    const { fetchMock, calls } = stubFetch([
      { session_id: "s1", reply: "What company did you work at?", done: false },
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderOnboardingInStrictMode();

    expect(
      await screen.findByText("What company did you work at?"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Sending…")).not.toBeInTheDocument();
    expect(calls).toHaveLength(1);
  });

  it("sends the typed answer with the session id echoed back", async () => {
    const user = userEvent.setup();
    const { fetchMock, calls } = stubFetch([
      { session_id: "s1", reply: "What company did you work at?", done: false },
      { session_id: "s1", reply: "What was your title there?", done: false },
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderOnboarding();

    await screen.findByText("What company did you work at?");
    await user.type(screen.getByLabelText("Your answer"), "Acme Corp");
    await user.click(screen.getByRole("button", { name: "Send" }));

    await screen.findByText("What was your title there?");
    expect(calls[1].body).toEqual({ session_id: "s1", message: "Acme Corp" });
    // The person's own answer stays visible in the transcript.
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
  });

  it("shows a completion state and hides the input once done", async () => {
    const user = userEvent.setup();
    const { fetchMock } = stubFetch([
      { session_id: "s1", reply: "Anything else?", done: false },
      { session_id: "s1", reply: "Thanks — that's everything.", done: true },
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderOnboarding();

    await screen.findByText("Anything else?");
    await user.type(screen.getByLabelText("Your answer"), "done");
    await user.click(screen.getByRole("button", { name: "Send" }));

    expect(
      await screen.findByText(/that's everything for now/i),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Your answer")).not.toBeInTheDocument();
  });

  it("surfaces a failed turn as readable text", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
      json: vi.fn().mockResolvedValue({
        error: "failed to reach the onboarding agent",
        code: "onboarding_failed",
      }),
    } as unknown as Response);
    vi.stubGlobal("fetch", fetchMock);
    renderOnboarding();

    expect(
      await screen.findByText("failed to reach the onboarding agent"),
    ).toBeInTheDocument();
  });

  it("does not send an empty answer", async () => {
    const { fetchMock } = stubFetch([
      { session_id: "s1", reply: "What company did you work at?", done: false },
    ]);
    vi.stubGlobal("fetch", fetchMock);
    renderOnboarding();

    await screen.findByText("What company did you work at?");
    const sendButton = screen.getByRole("button", { name: "Send" });
    expect(sendButton).toBeDisabled();

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1); // only the mount-time call
    });
  });
});
