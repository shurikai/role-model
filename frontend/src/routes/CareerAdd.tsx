import { Link } from "react-router-dom";
import { Page } from "../components/Page";
import { useHasPositions } from "../hooks/useCareer";
import { useImportBatches } from "../hooks/useImport";
import { formatApiError } from "../lib/api-client";
import type { ImportBatchSummary } from "../lib/types";

/**
 * The hub behind "+ Add to career": the three on-ramps, described by what the
 * person has in hand rather than by which import path or table they use.
 *
 * They used to be three top-level nav links ("Career import", "Add
 * contributions", "Career interview"), which asked a new user to already know
 * the difference.
 */
export function CareerAdd() {
  const { hasPositions, isLoading: positionsLoading } = useHasPositions();

  return (
    <Page
      width="narrow"
      eyebrow="Career · Add"
      title="Add to your career"
      lede="Pick whichever matches what you have to hand. Everything you add can be checked before it reaches a resume."
    >
      <div className="flex flex-col gap-3">
        <OnRampCard
          to="/import/career/new"
          title="I have a resume or CV"
          body="Paste it in. Employers, roles and what you did in each are read out of it and staged for you to approve one by one."
        />
        <OnRampCard
          to="/import/new"
          title="Add accomplishments to a job already here"
          body="Paste notes, a brag document or a performance review, and file what it describes under a role you have already added."
          disabledReason={
            !positionsLoading && !hasPositions
              ? "Add a job first: start with a resume, or talk it through."
              : undefined
          }
        />
        <OnRampCard
          to="/onboarding/new"
          title="I'd rather talk it through"
          body="Answer questions one at a time about where you have worked and what you did there. No document needed."
        />
      </div>

      <UnfinishedImports />
    </Page>
  );
}

function OnRampCard({
  to,
  title,
  body,
  disabledReason,
}: {
  to: string;
  title: string;
  body: string;
  disabledReason?: string;
}) {
  const content = (
    <>
      <p className="mb-1 font-display text-base font-bold text-ink">{title}</p>
      <p className="font-body text-[13px] text-ink-dim">{body}</p>
      {disabledReason && (
        <p className="mt-2 font-body text-[13px] text-flag">{disabledReason}</p>
      )}
    </>
  );

  if (disabledReason) {
    return (
      <div
        aria-disabled="true"
        className="border border-border bg-card p-4 opacity-60"
      >
        {content}
      </div>
    );
  }
  return (
    <Link
      to={to}
      className="block border border-border bg-card p-4 hover:border-verify"
    >
      {content}
    </Link>
  );
}

/**
 * Imports with drafts still awaiting a decision. A batch with no pending
 * drafts is either finished or never staged anything, and neither has a review
 * left to resume. That filter is also what makes the server's derived `kind`
 * safe to link by: it is only reliable once a batch has drafts.
 */
function UnfinishedImports() {
  const { data, isLoading, error } = useImportBatches();
  const unfinished = (data ?? []).filter((b) => b.pending_count > 0);

  if (isLoading) return null;
  if (error) {
    return (
      <p className="mt-8 font-body text-sm text-reject">
        {formatApiError(error)}
      </p>
    );
  }
  if (unfinished.length === 0) return null;

  return (
    <section className="mt-8">
      <h2 className="mb-2 font-display text-lg font-bold text-ink">
        Unfinished imports
      </h2>
      <ul className="flex flex-col gap-2">
        {unfinished.map((b) => (
          <li key={b.id}>
            <Link
              to={reviewPath(b)}
              className="flex items-baseline justify-between gap-4 border border-border bg-card px-4 py-3 hover:border-verify"
            >
              <span className="font-body text-sm text-ink">
                {b.kind === "career" ? "Resume import" : "Accomplishments"} ·{" "}
                {new Date(b.created_at).toLocaleDateString()}
              </span>
              <span className="font-mono text-[13px] text-ink-dim">
                {b.pending_count} to review
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

function reviewPath(b: ImportBatchSummary): string {
  return b.kind === "career" ? `/import/career/${b.id}` : `/import/${b.id}`;
}
