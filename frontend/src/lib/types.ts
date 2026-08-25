export interface AuthUser {
  id: string;
  email: string;
}

export interface GetUserRow {
  id: string;
  email: string;
  full_name: string | null;
  phone: string | null;
  location: string | null;
  linkedin_url: string | null;
  github_url: string | null;
  site_url: string | null;
  headline: string | null;
  created_at: string;
  updated_at: string;
}

export type ApplicationStatus =
  | "draft"
  | "applied"
  | "screen"
  | "interview"
  | "offer"
  | "rejected"
  | "withdrawn";

/**
 * Plain-language facts about a role that aren't skills-match signals —
 * the things you scan for before deciding whether a posting is worth
 * considering at all. Deliberately descriptive rather than judgmental:
 * the backend prompt is instructed not to editorialize, and neither
 * should the UI.
 */
export interface ScreeningSummary {
  location: string;
  work_arrangement: string;
  travel: string;
  industry: string;
  clearance_citizenship: string;
  other_flags: string[];
}

export interface JdSignals {
  required_skills: string[];
  preferred_skills: string[];
  seniority: string;
  // The posting's own words for its industry and its working arrangement.
  // These replaced two closed enums, `domain` and `work_type`, which were
  // recording a fraction of what the posting said: four unrelated companies
  // all extracted as domain "saas", and "remote" stood in for "fully remote
  // with occasional office visits". Absent on signals extracted before the
  // change.
  industry?: string;
  work_arrangement?: string;
  culture_signals: string[];
  // The capability-level asks a posting states in prose. A senior or staff
  // posting routinely names no technology at all, leaving both skill lists
  // correctly but uselessly empty; this is where its requirements live.
  // Absent on signals extracted before core_competencies was added.
  core_competencies?: string[];
  // Absent on signals extracted before screening_summary was added.
  screening_summary: ScreeningSummary | null;
}

/**
 * One preference row as a fit report carries it. All four preference lists —
 * matches, gaps, conflicts, and anti-pattern hits — are raw JSONB
 * passthroughs of []db.Preference, so they hold whole records rather than
 * labels.
 */
export interface PreferenceEntry {
  id: string;
  label: string;
  preference_type: string;
  sentiment: string;
  notes: string | null;
}

/**
 * Preference gaps and conflicts used to be stored as bare label strings, and
 * reports written before that changed are still in the database. Anything
 * rendering them has to handle both shapes; `preferenceLabel` is how.
 */
export type PreferenceListEntry = PreferenceEntry | string;

export function preferenceLabel(entry: PreferenceListEntry): string {
  return typeof entry === "string" ? entry : entry.label;
}

export interface Application {
  id: string;
  user_id: string;
  company_name: string;
  role_title: string;
  jd_url: string | null;
  jd_text: string | null;
  jd_signals: JdSignals | null;
  status: ApplicationStatus;
  applied_on: string | null;
  notes: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateApplicationRequest {
  company_name: string;
  role_title: string;
  jd_url?: string | null;
  jd_text?: string | null;
  status?: ApplicationStatus;
  notes?: string | null;
}

/**
 * One satisfied requirement from a fit report's technical scoring.
 *
 * `required_level` / `evidence_level` are present only where the posting
 * stated a depth for that requirement, which is the uncommon case. Their
 * absence means no depth was asked for — not that a depth check passed.
 */
export interface SkillMatch {
  requirement: string;
  kind: "direct" | "alias" | "category";
  category?: string;
  evidence: string[];
  required_level?: string;
  evidence_level?: string;
  level_signal?: string;
}

export interface FitReport {
  id: string;
  user_id: string;
  application_id: string;
  dealbreakers_clear: boolean;
  dealbreaker_hits: PreferenceEntry[] | null;
  capability_score: number | null;
  capability_gaps: string[] | null;
  // Requirements answered below the depth the posting asked for. Null on
  // reports predating the column, and on the many postings that state no
  // depth at all.
  capability_partial: SkillMatch[] | null;
  // Preference fit is a hit list, not a score: which preferences the JD
  // answers, which it is silent on, and which it runs against. Hard-gate
  // matches stay in dealbreaker_hits and are not repeated in conflicts.
  preference_matches: PreferenceListEntry[] | null;
  preference_gaps: PreferenceListEntry[] | null;
  preference_conflicts: PreferenceListEntry[] | null;
  narrative: string | null;
  // Copy captured at fit-evaluation time. Null for reports predating the
  // screening_summary migration.
  screening_summary: ScreeningSummary | null;
  created_at: string;
}

export interface ResumeVersion {
  id: string;
  user_id: string;
  application_id: string;
  version_number: number;
  generation_params: unknown | null;
  structured_output: unknown | null;
  generation_notes: string | null;
  submitted: boolean;
  created_at: string;
}

// ---------------------------------------------------------------------------
// Stage 0 import
// ---------------------------------------------------------------------------

/**
 * Why a field is worth a second look. Set by Stage 0b enrichment, one per
 * flag, each naming the field it is about.
 *
 * There is no confidence score anywhere in this pipeline — not on the draft,
 * not on the batch, not per field. Anything resembling one in the UI would be
 * invented here, so nothing does.
 */
export type DraftFlagType = "inference" | "gap" | "suggestion" | "warning";

export type DraftFlaggableField =
  | "employer_name"
  | "position_title"
  | "summary"
  | "full_description"
  | "outcomes"
  | "scale_context"
  | "general";

export interface DraftFlag {
  type: DraftFlagType;
  field: DraftFlaggableField;
  message: string;
}

/**
 * The four fields `PUT /import/drafts/{id}` will accept. `employer_name` and
 * `position_title` are flaggable but NOT updatable — the handler rejects an
 * unknown key with 400 rather than ignoring it, so sending one fails the whole
 * request (see `updatableDraftFields` in internal/api/handlers/import.go).
 */
export type DraftEditableField =
  "summary" | "full_description" | "outcomes" | "scale_context";

export const DRAFT_EDITABLE_FIELDS: readonly DraftEditableField[] = [
  "summary",
  "full_description",
  "outcomes",
  "scale_context",
];

export type ContributionDraftStatus = "pending" | "approved" | "rejected";

export interface ContributionDraft {
  id: string;
  user_id: string;
  batch_id: string;
  employer_name: string;
  position_title: string;
  summary: string | null;
  full_description: string | null;
  outcomes: string | null;
  scale_context: string | null;
  flags: DraftFlag[] | null;
  status: ContributionDraftStatus;
  created_at: string;
  updated_at: string;
}

/**
 * The batch lifecycle, as the database defines it
 * (`import_batches_status_check`, migration 006).
 *
 * There is no `ready`. Stage 0 ends a successful extraction at **`review`**
 * (`stage0.Service.RunExtraction`), and `complete` is what a resolved batch
 * reaches. Anything deciding "can this be reviewed yet" must test the WORKING
 * states rather than one finished state, so a status this UI has not heard of
 * degrades to reviewable rather than to a screen that waits forever.
 */
export type ImportBatchStatus =
  "pending" | "extracting" | "enriching" | "review" | "complete" | "failed";

export interface DraftCounts {
  total: number;
  pending: number;
  approved: number;
  rejected: number;
}

/**
 * `GET /import/{batchID}`: the whole batch row plus per-status draft counts.
 *
 * `raw_text` is the pasted source and comes back in full. It is the only
 * original text that exists — there is no per-draft "before" to compare a
 * field against.
 */
export interface ImportBatch {
  id: string;
  user_id: string;
  raw_text: string;
  status: ImportBatchStatus;
  error_text: string | null;
  created_at: string;
  updated_at: string;
  draft_counts: DraftCounts;
}

/**
 * `POST /import` answers with a narrower shape than the GET: no `raw_text`, no
 * timestamps, and a flat `draft_count` rather than the four-way breakdown.
 * Modelled separately because it genuinely is a different response, not an
 * ImportBatch with fields missing.
 */
export interface CreateImportBatchResponse {
  id: string;
  status: ImportBatchStatus;
  error_text?: string | null;
  draft_count: number;
}

export interface ApproveDraftResponse {
  contribution_id: string;
}

// ---------------------------------------------------------------------------
// Employers and positions
// ---------------------------------------------------------------------------

export interface Employer {
  id: string;
  user_id: string;
  name: string;
  industry: string | null;
  notes: string | null;
  created_at: string;
  updated_at: string;
}

/** Dates arrive as "YYYY-MM-DD" or null (pgtype.Date marshals that way). */
export interface Position {
  id: string;
  user_id: string;
  employer_id: string;
  title: string;
  industry_level: string | null;
  industry_role: string | null;
  level_rationale: string | null;
  started_on: string | null;
  ended_on: string | null;
  context_narrative: string | null;
  location: string | null;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface CreateEmployerRequest {
  name: string;
  industry?: string | null;
  notes?: string | null;
}

/** `started_on` is required and must be YYYY-MM-DD; the handler 400s without it. */
export interface CreatePositionRequest {
  employer_id: string;
  title: string;
  started_on: string;
  ended_on?: string | null;
  industry_level?: string | null;
  industry_role?: string | null;
  location?: string | null;
  level_rationale?: string | null;
  context_narrative?: string | null;
  sort_order?: number;
}
