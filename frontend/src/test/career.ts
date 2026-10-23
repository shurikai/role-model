import type {
  CareerEmployer,
  CareerPosition,
  Contribution,
} from "../lib/types";

export function makeCareerPosition(
  overrides: Partial<CareerPosition> = {},
): CareerPosition {
  return {
    id: "p1",
    user_id: "u1",
    employer_id: "e1",
    title: "Charge Nurse",
    industry_level: null,
    industry_role: null,
    level_rationale: null,
    started_on: "2020-02-01",
    ended_on: null,
    context_narrative: null,
    location: null,
    sort_order: 0,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
    contribution_counts: { active: 0, hidden: 0 },
    ...overrides,
  };
}

export function makeCareerEmployer(
  overrides: Partial<CareerEmployer> = {},
): CareerEmployer {
  return {
    id: "e1",
    user_id: "u1",
    name: "St. Elsewhere",
    industry: null,
    notes: null,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
    positions: [],
    ...overrides,
  };
}

export function makeContribution(
  overrides: Partial<Contribution> = {},
): Contribution {
  return {
    id: "c1",
    user_id: "u1",
    position_id: "p1",
    summary: "Cut handoff errors by standardizing shift report.",
    full_description: "Wrote and rolled out a structured handoff template.",
    outcomes: null,
    scale_context: null,
    is_active: true,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
    tags: [],
    ...overrides,
  };
}
