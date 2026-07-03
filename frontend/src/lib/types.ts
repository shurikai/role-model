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
