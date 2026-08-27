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

export function Signup() {
  const { signup } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signup(email, password);
      // A brand-new account has nothing in it, and the applications list is
      // no use without a career to tailor against — so a signup with no
      // redirect of its own lands on the career import instead of on an
      // empty list with no path off it. A ?redirect= still wins: someone
      // sent to sign up from a specific page is going back to that page.
      const redirect = searchParams.get("redirect");
      navigate(redirect || "/import/career/new");
    } catch (err) {
      setError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthCard
      eyebrow="Role Model"
      title="Sign up"
      intro="One account holds your career history, what you want from a role, and every application you tailor against it."
      footer={
        <p className="font-body text-sm text-ink-dim">
          Already have an account? <QuietLink to="/login">Log in</QuietLink>
        </p>
      }
    >
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
          autoComplete="new-password"
          value={password}
          onChange={setPassword}
        />

        {error && <p className="mb-3 font-body text-sm text-reject">{error}</p>}

        <SubmitButton pending={submitting} pendingLabel="Signing up…" full>
          Sign up
        </SubmitButton>
      </form>
    </AuthCard>
  );
}
