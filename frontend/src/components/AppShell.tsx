import { NavLink, Link } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

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

  return (
    <div className="flex min-h-screen flex-col bg-paper">
      <header className="border-b border-border bg-card">
        <div className="mx-auto flex max-w-4xl flex-wrap items-center gap-x-6 gap-y-2 px-6 py-3">
          <Link
            to="/applications"
            className="font-display text-sm font-bold tracking-tight text-ink"
          >
            Role Model
          </Link>

          <nav className="flex items-center gap-4">
            <ShellLink to="/applications">Applications</ShellLink>
            <ShellLink to="/import/career/new">Career import</ShellLink>
            {/*
              The narrow path: contributions against employers and positions
              that already exist. Named for what it does rather than for which
              of the two import routes it happens to be, since "import" alone
              cannot distinguish them.
            */}
            <ShellLink to="/import/new">Add contributions</ShellLink>
          </nav>

          <div className="ml-auto flex items-center gap-3">
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
 * `end` on every link because the paths nest: /applications would otherwise
 * stay marked active while /applications/new is open, and /import/new sits
 * under the same prefix as /import/career/new.
 */
function ShellLink({
  to,
  children,
}: {
  to: string;
  children: React.ReactNode;
}) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        `font-body text-[13px] ${
          isActive
            ? "border-b-2 border-verify pb-0.5 text-ink"
            : "border-b-2 border-transparent pb-0.5 text-ink-dim"
        }`
      }
    >
      {children}
    </NavLink>
  );
}
