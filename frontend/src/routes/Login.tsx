import { useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";
import { formatApiError } from "../lib/api-client";
import {
  AuthCard,
  Field,
  QuietLink,
  SubmitButton,
} from "../components/AuthCard";

export function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [bannerDismissed, setBannerDismissed] = useState(false);
  // Derived from searchParams on every render rather than captured once at
  // mount: RequireAuth's redirect and AuthContext's expiry redirect both
  // target /login, so React Router updates this component's location in
  // place instead of remounting it — a mount-time useState initializer would
  // miss the ?reason=expired that arrives via the second navigation.
  const showExpiredBanner =
    searchParams.get("reason") === "expired" && !bannerDismissed;

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(email, password);
      const redirect = searchParams.get("redirect");
      navigate(redirect || "/");
    } catch (err) {
      setError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthCard
      eyebrow="Role Model"
      title="Log in"
      footer={
        <div className="font-body text-sm text-ink-dim">
          <p>
            Don't have an account? <QuietLink to="/signup">Sign up</QuietLink>
          </p>
          {/*
            There is no reset flow yet, and a "Forgot password?" link to
            nothing is worse than none — it promises a path that does not
            exist. This says what to actually do instead, which needs shell
            access on the host, so it is honest about who can do it.
          */}
          <p className="mt-2 text-xs">
            Forgotten your password? This is self-hosted software with no reset
            flow yet — on the server, run{" "}
            <code className="font-mono">
              make reset-password EMAIL=you@example.com
            </code>
            .
          </p>
        </div>
      }
    >
      {showExpiredBanner && (
        <div className="mb-4 flex items-start justify-between gap-2 border border-flag bg-flag-highlight px-3 py-2">
          <span className="font-body text-sm text-ink">
            Your session expired. Please log in again.
          </span>
          <button
            type="button"
            onClick={() => setBannerDismissed(true)}
            className="font-mono text-sm text-ink-dim"
            aria-label="Dismiss"
          >
            &times;
          </button>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <Field
          id="email"
          label="Email"
          type="email"
          required
          autoComplete="username"
          value={email}
          onChange={setEmail}
        />
        <Field
          id="password"
          label="Password"
          type="password"
          required
          autoComplete="current-password"
          value={password}
          onChange={setPassword}
        />

        {error && <p className="mb-3 font-body text-sm text-reject">{error}</p>}

        <SubmitButton pending={submitting} pendingLabel="Logging in…" full>
          Log in
        </SubmitButton>
      </form>
    </AuthCard>
  );
}
