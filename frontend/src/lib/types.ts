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
  domain: string;
  work_type: string;
  culture_signals: string[];
  // Absent on signals extracted before screening_summary was added.
  screening_summary: ScreeningSummary | null;
}

/**
 * A preference row the anti-pattern check matched. The API returns whole
 * preference records here, not labels — `anti_pattern_hits` is a raw JSONB
 * passthrough of []db.Preference.
 */
export interface PreferenceHit {
  id: string;
  label: string;
  preference_type: string;
  sentiment: string;
  notes: string | null;
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

export interface FitReport {
  id: string;
  user_id: string;
  application_id: string;
  anti_pattern_passed: boolean;
  anti_pattern_hits: PreferenceHit[] | null;
  technical_score: number | null;
  technical_gaps: string[] | null;
  preference_score: number | null;
  preference_gaps: string[] | null;
  preference_conflicts: string[] | null;
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
