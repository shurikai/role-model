import { Link } from "react-router-dom";
import { EmptyState } from "../components/EmptyState";
import { Page } from "../components/Page";
import { useCareerTree } from "../hooks/useCareer";
import { formatApiError } from "../lib/api-client";
import { formatSpan } from "../lib/dates";
import type { CareerEmployer, CareerPosition } from "../lib/types";

const addLink = (
  <Link
    to="/career/add"
    className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface"
  >
    + Add to career
  </Link>
);

/**
 * The record itself: every employer, each listing its positions. Read-only;
 * editing is the next PR (#118).
 *
 * Employers come in the order the server sends them, which is resume order —
 * current first, then by most recent end date — because that is how people
 * think about their own history. Re-sorting here would undo it.
 *
 * There is no employer page. An employer is a heading here and a breadcrumb
 * segment on a position; it has too little content of its own to earn a
 * screen.
 */
export function Career() {
  const { data, isLoading, error } = useCareerTree();
  const employers = data?.employers ?? [];

  return (
    <Page
      eyebrow="Career"
      title="Your career"
      actions={addLink}
      lede="Where you have worked, the roles you held, and what you did in each. Resumes are built from what is here."
    >
      {isLoading && (
        <p className="font-body text-sm text-ink-dim">Loading your career…</p>
      )}
      {error && (
        <p className="font-body text-sm text-reject">{formatApiError(error)}</p>
      )}
      {/* Only once the tree has loaded: telling an account with ten years of
          history that it has none, even for a moment, is worse than a
          loading line. */}
      {data && employers.length === 0 && (
        <EmptyState title="Nothing here yet" action={addLink}>
          Add a resume, talk it through, or start with one job. Everything you
          add shows up here, and resumes are built from it.
        </EmptyState>
      )}
      <div className="flex flex-col gap-8">
        {employers.map((employer) => (
          <EmployerSection key={employer.id} employer={employer} />
        ))}
      </div>
    </Page>
  );
}

function EmployerSection({ employer }: { employer: CareerEmployer }) {
  return (
    <section aria-labelledby={`employer-${employer.id}`}>
      <div className="mb-2 flex flex-wrap items-baseline gap-x-3">
        <h2
          id={`employer-${employer.id}`}
          className="font-display text-lg font-bold text-ink"
        >
          {employer.name}
        </h2>
        {employer.industry && (
          <span className="font-body text-[13px] text-ink-dim">
            {employer.industry}
          </span>
        )}
      </div>
      {employer.positions.length === 0 ? (
        <p className="border border-dashed border-rail bg-card px-4 py-3 font-body text-[13px] text-ink-dim">
          No roles recorded here yet.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {employer.positions.map((position) => (
            <li key={position.id}>
              <PositionRow position={position} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function PositionRow({ position }: { position: CareerPosition }) {
  const { active, hidden } = position.contribution_counts;
  return (
    <Link
      to={`/career/positions/${position.id}`}
      className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border border-border bg-card px-4 py-3 hover:border-verify"
    >
      <span className="min-w-0">
        <span className="block font-body text-sm font-semibold text-ink">
          {position.title}
        </span>
        <span className="block font-body text-[13px] text-ink-dim">
          {formatSpan(position.started_on, position.ended_on)}
          {position.location && ` · ${position.location}`}
        </span>
      </span>
      <span className="font-mono text-[13px] text-ink-dim">
        {contributionCountLabel(active, hidden)}
      </span>
    </Link>
  );
}

function contributionCountLabel(active: number, hidden: number): string {
  const shown = `${active} ${active === 1 ? "contribution" : "contributions"}`;
  return hidden > 0 ? `${shown} · ${hidden} hidden` : shown;
}
