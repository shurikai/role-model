import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";
import { AppShell } from "./AppShell";

export function RequireAuth() {
  const { isAuthenticated, sessionExpired } = useAuth();
  const location = useLocation();

  if (!isAuthenticated) {
    const params = new URLSearchParams({ redirect: location.pathname });
    if (sessionExpired) {
      params.set("reason", "expired");
    }
    return <Navigate to={`/login?${params.toString()}`} replace />;
  }

  // The shell wraps the Outlet rather than each page, so it is impossible to
  // add an authenticated route that renders without navigation — which is how
  // /import/new ended up unreachable.
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
