import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { FeedbackDialog } from "./FeedbackDialog";

interface Recorded {
  path: string;
  body: Record<string, unknown>;
}

function stubFetch(response: { status: number; body: unknown }) {
  const calls: Recorded[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: unknown, init?: RequestInit) => {
      calls.push({
        path: String(url).replace(/^.*\/api\/v1/, ""),
        body: JSON.parse(String(init?.body ?? "{}")),
      });
      return {
        ok: response.status < 400,
        status: response.status,
        json: vi.fn().mockResolvedValue(response.body),
      } as unknown as Response;
    }),
  );
  return calls;
}

/** The dialog as the shell uses it: open, closing on send, with a thanks. */
function Harness() {
  const [open, setOpen] = useState(true);
  const [thanked, setThanked] = useState(false);
  return (
    <>
      {thanked && <p role="status">Thanks, got it</p>}
      <FeedbackDialog
        open={open}
        onClose={() => setOpen(false)}
        onSent={() => {
          setOpen(false);
          setThanked(true);
        }}
      />
    </>
  );
}

function renderDialog(at = "/career/positions/p1?tab=history") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[at]}>
        <Harness />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function send() {
  return screen.getByRole("button", { name: "Send" });
}

describe("FeedbackDialog", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
    cleanup();
  });

  it("sends where the tester was without them typing it", async () => {
    vi.stubEnv("VITE_APP_VERSION", "a1b2c3d");
    vi.stubGlobal("innerWidth", 1280);
    vi.stubGlobal("innerHeight", 720);
    const calls = stubFetch({ status: 201, body: { id: "f1" } });
    const user = userEvent.setup();
    renderDialog();

    // None of the context is on screen to edit.
    expect(screen.queryByDisplayValue(/career/)).not.toBeInTheDocument();
    expect(
      screen.getByText("We'll include which page you were on."),
    ).toBeInTheDocument();

    await user.click(screen.getByLabelText("Something was confusing"));
    await user.type(
      screen.getByLabelText("What happened"),
      "  Lost my edit.  ",
    );
    await user.click(send());

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({
      path: "/feedback",
      body: {
        category: "confusing",
        message: "Lost my edit.",
        route: "/career/positions/p1?tab=history",
        app_version: "a1b2c3d",
        viewport: "1280x720",
      },
    });
  });

  it("says dev when the build was not tagged", async () => {
    vi.stubEnv("VITE_APP_VERSION", "");
    const calls = stubFetch({ status: 201, body: { id: "f1" } });
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByLabelText("An idea"));
    await user.type(screen.getByLabelText("What happened"), "Dark mode?");
    await user.click(send());

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0].body.app_version).toBe("dev");
  });

  it("keeps Send disabled until a category is picked and there is a message", async () => {
    stubFetch({ status: 201, body: { id: "f1" } });
    const user = userEvent.setup();
    renderDialog();

    expect(send()).toBeDisabled();
    await user.type(screen.getByLabelText("What happened"), "   ");
    expect(send()).toBeDisabled();
    await user.click(screen.getByLabelText("Something broke"));
    // Whitespace is not a message.
    expect(send()).toBeDisabled();
    await user.type(screen.getByLabelText("What happened"), "It froze.");
    expect(send()).toBeEnabled();
  });

  it("keeps the dialog and the text when sending fails", async () => {
    stubFetch({
      status: 400,
      body: { error: "route is required", code: "validation_error" },
    });
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByLabelText("Something broke"));
    await user.type(screen.getByLabelText("What happened"), "Import hung.");
    await user.click(send());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "route is required",
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("What happened")).toHaveValue("Import hung.");
    expect(screen.getByLabelText("Something broke")).toBeChecked();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("closes on success and thanks the tester", async () => {
    stubFetch({ status: 201, body: { id: "f1" } });
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByLabelText("An idea"));
    await user.type(screen.getByLabelText("What happened"), "Export to PDF.");
    await user.click(send());

    expect(await screen.findByRole("status")).toHaveTextContent(
      "Thanks, got it",
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("cancels without sending", async () => {
    const calls = stubFetch({ status: 201, body: { id: "f1" } });
    const user = userEvent.setup();
    renderDialog();

    await user.type(screen.getByLabelText("What happened"), "Never mind");
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(calls).toHaveLength(0);
  });
});
