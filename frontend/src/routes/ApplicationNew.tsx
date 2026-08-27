import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  useCreateApplication,
  useExtractSignals,
} from "../hooks/useApplications";
import { formatApiError } from "../lib/api-client";
import { Field, QuietLink, SubmitButton } from "../components/AuthCard";

export function ApplicationNew() {
  const navigate = useNavigate();
  const createApplication = useCreateApplication();
  const extractSignals = useExtractSignals();
  const [companyName, setCompanyName] = useState("");
  const [roleTitle, setRoleTitle] = useState("");
  const [jdUrl, setJdUrl] = useState("");
  const [jdText, setJdText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const application = await createApplication.mutateAsync({
        company_name: companyName,
        role_title: roleTitle,
        jd_url: jdUrl || null,
        jd_text: jdText,
      });
      try {
        await extractSignals.mutateAsync(application.id);
      } catch {
        // Signal extraction can be retried from the detail page —
        // the application itself was created successfully.
      }
      navigate(`/applications/${application.id}`);
    } catch (err) {
      setError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-6 py-10">
      <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
        Applications · New
      </p>
      <h1 className="mb-1 font-display text-2xl font-bold text-ink">
        Paste a job description
      </h1>
      <p className="mb-6 font-body text-[13px] text-ink-dim">
        This reads the posting for what it asks for — requirements, the
        capabilities behind them, seniority, and how the role describes itself —
        and stores those signals against the application. Nothing is compared
        against your career until you run the fit gate.
      </p>

      <form onSubmit={handleSubmit}>
        <Field
          id="companyName"
          label="Company"
          type="text"
          required
          value={companyName}
          onChange={setCompanyName}
        />
        <Field
          id="roleTitle"
          label="Role title"
          type="text"
          required
          value={roleTitle}
          onChange={setRoleTitle}
        />
        <Field
          id="jdUrl"
          label="Job posting URL"
          type="url"
          optional
          value={jdUrl}
          onChange={setJdUrl}
        />

        <div className="mb-4">
          <label
            htmlFor="jdText"
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Job description text
          </label>
          <textarea
            id="jdText"
            required
            rows={16}
            value={jdText}
            onChange={(e) => setJdText(e.target.value)}
            disabled={submitting}
            className="w-full border border-border bg-surface p-3 font-body text-sm text-ink disabled:opacity-60"
          />
        </div>

        {error && <p className="mb-3 font-body text-sm text-reject">{error}</p>}

        <div className="flex items-center gap-3">
          <SubmitButton
            pending={submitting}
            pendingLabel="Reading the posting…"
          >
            Create and extract signals
          </SubmitButton>
          <QuietLink to="/applications">Cancel</QuietLink>
        </div>

        {submitting && (
          <p className="mt-3 font-body text-sm text-ink-dim">
            Reading the posting in one pass. If extraction fails the application
            is still created, and you can retry it from its own page.
          </p>
        )}
      </form>
    </div>
  );
}
