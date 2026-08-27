import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useStartCareerImport } from "../hooks/useIntake";
import { formatApiError } from "../lib/api-client";

/**
 * The on-ramp for a brand-new account: paste a career, get a review queue.
 *
 * Distinct from `/import/new`, which stages contributions against employers
 * and positions that already exist. This one stages those too, which is what
 * makes it the path a new signup can actually use.
 */
export function CareerImportStart() {
  const [rawText, setRawText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const startImport = useStartCareerImport();
  const navigate = useNavigate();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!rawText.trim()) {
      setError("Paste something to import first.");
      return;
    }
    try {
      const batch = await startImport.mutateAsync(rawText);
      navigate(`/import/career/${batch.id}`);
    } catch (err) {
      setError(formatApiError(err));
    }
  }

  return (
    <div>
      <div className="mx-auto max-w-3xl px-6 py-10">
        <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
          Career import
        </p>
        <h1 className="mb-1 font-display text-2xl font-bold text-ink">
          Start with your career
        </h1>
        <p className="mb-6 font-body text-[13px] text-ink-dim">
          Paste a résumé, a CV, a work history, or notes. This reads employers,
          positions, what you did in each, and the skills behind them — and
          stages all of it for you to review. Nothing is written to your record
          until you approve it.
        </p>

        <form onSubmit={submit}>
          <label
            htmlFor="career-text"
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Career text
          </label>
          <textarea
            id="career-text"
            value={rawText}
            onChange={(e) => setRawText(e.target.value)}
            rows={18}
            disabled={startImport.isPending}
            className="w-full border border-border bg-surface p-3 font-body text-sm text-ink disabled:opacity-60"
          />

          {error && (
            <p className="mt-2 font-body text-sm text-reject">{error}</p>
          )}

          <div className="mt-4 flex items-center gap-3">
            <button
              type="submit"
              disabled={startImport.isPending}
              className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50"
            >
              {startImport.isPending
                ? "Reading your career…"
                : "Read my career"}
            </button>
            <Link
              to="/applications"
              className="font-body text-sm text-ink-dim underline"
            >
              Skip for now
            </Link>
          </div>

          {/*
            A whole career through a model is not a page load. Saying what is
            happening and roughly how long is the difference between waiting
            and assuming it broke.
          */}
          {startImport.isPending && (
            <p className="mt-3 font-body text-sm text-ink-dim">
              Reading the whole thing in one pass — employers, positions,
              contributions and skills. This usually takes under a minute.
              Leaving this page cancels nothing, but you will need the link to
              come back.
            </p>
          )}
        </form>
      </div>
    </div>
  );
}
