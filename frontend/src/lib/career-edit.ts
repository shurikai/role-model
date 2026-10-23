import type {
  Contribution,
  Employer,
  Position,
  UpdateContributionRequest,
  UpdateEmployerRequest,
  UpdatePositionRequest,
} from "./types";

/**
 * Request bodies for the career edit forms.
 *
 * Every update endpoint here is full replacement: a field missing from the
 * PATCH body is written as null. So a body is always the fetched entity with
 * the form's fields laid over it, never the form alone, and a field the form
 * does not show (a position's sort_order) goes back exactly as it came.
 */

/** Blank and whitespace-only text is no value, not an empty string. */
export function nullable(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

export function employerUpdate(
  employer: Employer,
  form: Partial<UpdateEmployerRequest>,
): UpdateEmployerRequest {
  return {
    name: employer.name,
    industry: employer.industry,
    notes: employer.notes,
    ...form,
  };
}

/**
 * started_on is NOT NULL in the schema and the handler 400s without it, but
 * the type allows null because the JSON does. A position read back without
 * one cannot be saved until the form supplies it.
 */
export function positionUpdate(
  position: Position,
  form: Partial<UpdatePositionRequest>,
): UpdatePositionRequest {
  return {
    title: position.title,
    industry_level: position.industry_level,
    industry_role: position.industry_role,
    location: position.location,
    level_rationale: position.level_rationale,
    started_on: position.started_on ?? "",
    ended_on: position.ended_on,
    context_narrative: position.context_narrative,
    sort_order: position.sort_order,
    ...form,
  };
}

export function contributionUpdate(
  contribution: Contribution,
  form: Partial<UpdateContributionRequest>,
): UpdateContributionRequest {
  return {
    summary: contribution.summary,
    full_description: contribution.full_description,
    outcomes: contribution.outcomes,
    scale_context: contribution.scale_context,
    is_active: contribution.is_active,
    ...form,
  };
}

/** "2022-02-15" → "2022-02", the value an <input type="month"> takes. */
export function toMonth(date: string | null): string {
  return date ? date.slice(0, 7) : "";
}

/**
 * The form edits months and the API stores days. A month the user did not
 * change keeps the stored day — saving an untouched form must not move an
 * end date of 2023-03-31 to 2023-03-01 — and a changed one is sent as the
 * 1st, which is how Stage 0 writes dates it only knows to the month.
 */
export function fromMonth(
  month: string,
  original: string | null,
): string | null {
  if (month === "") return null;
  if (original && toMonth(original) === month) return original;
  return `${month}-01`;
}
