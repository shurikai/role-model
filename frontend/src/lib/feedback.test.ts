import { describe, it, expect } from "vitest";
import { feedbackHref } from "./feedback";

describe("feedbackHref", () => {
  it("is null when nothing is configured", () => {
    expect(feedbackHref(undefined, "/career")).toBeNull();
    expect(feedbackHref("  ", "/career")).toBeNull();
  });

  it("uses a form URL as given", () => {
    expect(feedbackHref("https://forms.example/abc", "/career")).toBe(
      "https://forms.example/abc",
    );
  });

  it("prefills a mailto with the page the report came from", () => {
    const href = feedbackHref("mailto:me@example.com", "/career/positions/p1");
    expect(href).toMatch(/^mailto:me@example\.com\?/);
    const params = new URLSearchParams(href!.split("?")[1]);
    expect(params.get("subject")).toBe("Role Model feedback");
    expect(params.get("body")).toContain("Page: /career/positions/p1");
    // Spaces as %20, never "+", which a mail client would show literally.
    expect(href).not.toContain("+");
  });

  it("keeps a subject the deployment already set", () => {
    const href = feedbackHref("mailto:me@example.com?subject=Beta", "/x");
    expect(new URLSearchParams(href!.split("?")[1]).get("subject")).toBe(
      "Beta",
    );
  });
});
