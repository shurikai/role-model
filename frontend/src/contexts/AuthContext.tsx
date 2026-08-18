import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { apiFetch, setUnauthorizedHandler } from "../lib/api-client";
import { getSession, setSession, clearSession } from "../lib/session";
import type { AuthUser, GetUserRow } from "../lib/types";

interface AuthResponse {
  token: string;
  user: AuthUser;
}

interface AuthContextValue {
  user: AuthUser | null;
  isAuthenticated: boolean;
  sessionExpired: boolean;
  login: (email: string, password: string) => Promise<void>;
  signup: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const [user, setUser] = useState<AuthUser | null>(
    () => getSession()?.user ?? null,
  );
  const [sessionExpired, setSessionExpired] = useState(false);
  const isAuthenticated = user !== null;

  useEffect(() => {
    // Only clears session state and flags *why* — RequireAuth is the sole
    // component that calls navigate() for the unauthenticated case. Calling
    // navigate() here too raced with RequireAuth's own redirect (both fire
    // off the same setUser(null) transition): RequireAuth's declarative
    // <Navigate> was landing after this imperative call and clobbering the
    // URL, silently dropping the reason=expired param.
    setUnauthorizedHandler(() => {
      clearSession();
      setUser(null);
      setSessionExpired(true);
    });
  }, []);

  useQuery({
    queryKey: ["me"],
    queryFn: async () => {
      const me = await apiFetch<GetUserRow>("/auth/me");
      const session = getSession();
      if (session) {
        setSession({
          ...session,
          user: { id: me.id, email: me.email },
        });
      }
      return me;
    },
    enabled: isAuthenticated,
    retry: false,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });

  async function login(email: string, password: string): Promise<void> {
    const res = await apiFetch<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    setSession({ token: res.token, user: res.user });
    setUser(res.user);
    setSessionExpired(false);
  }

  async function signup(email: string, password: string): Promise<void> {
    const res = await apiFetch<AuthResponse>("/auth/signup", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    setSession({ token: res.token, user: res.user });
    setUser(res.user);
    setSessionExpired(false);
  }

  function logout(): void {
    clearSession();
    setUser(null);
    navigate("/login");
  }

  return (
    <AuthContext.Provider
      value={{ user, isAuthenticated, sessionExpired, login, signup, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
