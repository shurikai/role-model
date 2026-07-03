import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

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

  return <Outlet />;
}
