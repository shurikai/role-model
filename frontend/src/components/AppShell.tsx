import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";
import { feedbackHref } from "../lib/feedback";

/**
 * The frame every authenticated page renders inside.
 *
 * There was no nav of any kind before this. `logout()` had been defined on
 * AuthContext since the auth shell landed and was called by nothing, so a
 * signed-in user could not sign out except by clearing localStorage — and
 * `/import/new`, the narrow import path, was reachable only by typing its URL.
 * Both are symptoms of the same absence rather than two separate omissions.
 *
 * The shell owns `min-h-screen bg-paper` so pages own only their own column
 * width. A page keeping its own `min-h-screen` inside this would stack a full
 * viewport under the header and give every screen a scrollbar it did not need.
 */
export function AppShell({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const { pathname } = useLocation();
  const feedback = feedbackHref(import.meta.env.VITE_FEEDBACK_URL, pathname);

  return (
    <div className="flex min-h-screen flex-col bg-paper">
      <header className="border-b border-border bg-card">
        <div className="mx-auto flex max-w-4xl flex-wrap items-center gap-x-6 gap-y-2 px-6 py-3">
          <Link
            to="/career"
            className="font-display text-sm font-bold tracking-tight text-ink"
          >
            Role Model
          </Link>

          <nav className="flex items-center gap-4">
            <ShellLink to="/career" sections={CAREER_SECTIONS}>
              Career
            </ShellLink>
            <ShellLink to="/applications" sections={["/applications"]}>
              Applications
            </ShellLink>
            {/*
              "Profile" read as name and contact details to someone who does
              not know the data model. Label only: the route stays /profile.
            */}
            <ShellLink to="/profile" sections={["/profile"]}>
              Skills &amp; preferences
            </ShellLink>
          </nav>
          {/*
            The three on-ramps (career import, add contributions, career
            interview) used to be three of five top-level links, all doing one
            job by different means. They live behind this one action now, on a
            hub that explains which to pick.
          */}
          <Link
            to="/career/add"
            className="bg-ink px-3 py-1.5 font-display text-xs font-bold text-surface"
          >
            + Add to career
          </Link>

          <div className="ml-auto flex items-center gap-3">
            {feedback && (
              <a
                href={feedback}
                target="_blank"
                rel="noreferrer"
                className="font-body text-xs text-verify underline"
              >
                Feedback
              </a>
            )}
            {user && (
              <span className="font-mono text-[10px] tracking-widest text-rail uppercase">
                {user.email}
              </span>
            )}
            <button
              type="button"
              onClick={logout}
              className="border border-border bg-surface px-3 py-1.5 font-body text-xs text-ink-dim"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      <main className="flex-1">{children}</main>
    </div>
  );
}

/**
 * The on-ramps are reached from /career/add and write to the career, so the
 * Career tab stays current while one is open.
 */
const CAREER_SECTIONS = ["/career", "/import", "/onboarding"];

/**
 * Current-ness is by section prefix, not exact path. The links used to carry
 * `end`, which was right while every nav target was a leaf; Career has nested
 * routes (/career/positions/:id) that must keep it marked. Matching on a path
 * segment boundary keeps /applications-archive, say, out of /applications.
 */
function ShellLink({
  to,
  sections,
  children,
}: {
  to: string;
  sections: string[];
  children: React.ReactNode;
}) {
  const { pathname } = useLocation();
  const current = sections.some(
    (s) => pathname === s || pathname.startsWith(s + "/"),
  );
  return (
    <Link
      to={to}
      aria-current={current ? "page" : undefined}
      className={`font-body text-[13px] ${
        current
          ? "border-b-2 border-verify pb-0.5 text-ink"
          : "border-b-2 border-transparent pb-0.5 text-ink-dim"
      }`}
    >
      {children}
    </Link>
  );
}
