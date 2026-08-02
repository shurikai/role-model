import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ErrorBoundary } from "./ErrorBoundary";

function Boom(): never {
  throw new Error("render exploded");
}

function renderBoundary(children: React.ReactNode) {
  return render(
    <MemoryRouter>
      <ErrorBoundary>{children}</ErrorBoundary>
    </MemoryRouter>,
  );
}

describe("ErrorBoundary", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("renders children when nothing throws", () => {
    renderBoundary(<p>all good</p>);
    expect(screen.getByText("all good")).toBeInTheDocument();
  });

  it("catches a render error instead of unmounting to a blank page", () => {
    // React logs the caught error; silence it so the run stays readable.
    vi.spyOn(console, "error").mockImplementation(() => {});

    renderBoundary(<Boom />);

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByText("render exploded")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to applications" })).toBeInTheDocument();
  });
});
