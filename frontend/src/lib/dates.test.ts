import { describe, it, expect } from "vitest";
import { formatMonth, formatSpan } from "./dates";

describe("dates", () => {
  // The 1st is the case `new Date("YYYY-MM-DD")` gets wrong west of UTC.
  it("formats the first of a month as that month", () => {
    expect(formatMonth("2022-02-01")).toBe("Feb 2022");
  });

  it("reads an open end as Present", () => {
    expect(formatSpan("2022-02-01", null)).toBe("Feb 2022 – Present");
    expect(formatSpan("2019-07-15", "2022-01-31")).toBe("Jul 2019 – Jan 2022");
  });

  it("does not invent a start date", () => {
    expect(formatSpan(null, "2020-06-30")).toBe("Start unknown – Jun 2020");
  });
});
