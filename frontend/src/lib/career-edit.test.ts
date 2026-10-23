import { describe, it, expect } from "vitest";
import {
  contributionUpdate,
  fromMonth,
  nullable,
  positionUpdate,
  toMonth,
} from "./career-edit";
import { makeCareerPosition, makeContribution } from "../test/career";

describe("career edit bodies", () => {
  // The whole point: the update endpoints are full replacement.
  it("carries a position's fields the form does not touch", () => {
    const position = makeCareerPosition({
      sort_order: 3,
      level_rationale: "Led a team of eight.",
      context_narrative: "Night shift, 24-bed unit.",
    });
    const body = positionUpdate(position, { title: "Nurse Manager" });
    expect(body.title).toBe("Nurse Manager");
    expect(body.sort_order).toBe(3);
    expect(body.level_rationale).toBe("Led a team of eight.");
    expect(body.context_narrative).toBe("Night shift, 24-bed unit.");
    expect(body.started_on).toBe(position.started_on);
  });

  it("toggles is_active without touching the text", () => {
    const c = makeContribution({ outcomes: "Errors fell by a third." });
    expect(contributionUpdate(c, { is_active: false })).toEqual({
      summary: c.summary,
      full_description: c.full_description,
      outcomes: "Errors fell by a third.",
      scale_context: null,
      is_active: false,
    });
  });

  it("reads blank text as null", () => {
    expect(nullable("   ")).toBeNull();
    expect(nullable(" Portland ")).toBe("Portland");
  });
});

describe("month fields", () => {
  it("keeps the stored day when the month is unchanged", () => {
    expect(fromMonth(toMonth("2023-03-31"), "2023-03-31")).toBe("2023-03-31");
  });

  it("sends a changed month as the 1st", () => {
    expect(fromMonth("2023-05", "2023-03-31")).toBe("2023-05-01");
    expect(fromMonth("2023-05", null)).toBe("2023-05-01");
  });

  it("reads an empty month as no date", () => {
    expect(fromMonth("", "2023-03-31")).toBeNull();
  });
});
