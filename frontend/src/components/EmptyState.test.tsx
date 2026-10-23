import { describe, it, expect, afterEach } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { EmptyState } from "./EmptyState";

describe("EmptyState", () => {
  afterEach(cleanup);

  it("says what would be here and offers the action", () => {
    render(
      <EmptyState
        title="Nothing here yet"
        action={<button type="button">Add to career</button>}
      >
        Employers and roles you add appear here.
      </EmptyState>,
    );

    expect(screen.getByText("Nothing here yet")).toBeInTheDocument();
    expect(
      screen.getByText("Employers and roles you add appear here."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add to career" }),
    ).toBeInTheDocument();
  });
});
