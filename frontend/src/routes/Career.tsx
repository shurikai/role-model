import { useState } from "react";
import { Link } from "react-router-dom";
import {
  EmployerForm,
  PositionForm,
  textButton,
} from "../components/CareerForms";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { EmptyState } from "../components/EmptyState";
import { Page } from "../components/Page";
import { useCareerTree, useDeleteEmployer } from "../hooks/useCareer";
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
 * The record itself: every employer, each listing its positions. Employers
 * are edited in place here; positions open their own page.
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
  const [mode, setMode] = useState<"view" | "edit" | "add-position">("view");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const del = useDeleteEmployer();

  // An employer with positions cannot be deleted (the server 409s), so the
  // control is not offered rather than offered and refused.
  const deletable = employer.positions.length === 0;

  return (
    <section aria-labelledby={`employer-${employer.id}`}>
      {mode === "edit" ? (
        <div className="mb-3">
          <EmployerForm employer={employer} onDone={() => setMode("view")} />
        </div>
      ) : (
        <div className="mb-2 flex flex-wrap items-baseline justify-between gap-x-4">
          <div className="flex flex-wrap items-baseline gap-x-3">
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
          <div className="flex gap-3">
            <button
              type="button"
              onClick={() => setMode("edit")}
              aria-label={`Edit ${employer.name}`}
              className={textButton}
            >
              Edit
            </button>
            {deletable && (
              <button
                type="button"
                onClick={() => setConfirmingDelete(true)}
                aria-label={`Delete ${employer.name}`}
                className="font-body text-[13px] text-reject hover:underline"
              >
                Delete
              </button>
            )}
          </div>
        </div>
      )}
      {del.error && (
        <p role="alert" className="mb-2 font-body text-sm text-reject">
          {formatApiError(del.error)}
        </p>
      )}

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

      {mode === "add-position" ? (
        <div className="mt-2">
          <PositionForm
            employerID={employer.id}
            onDone={() => setMode("view")}
          />
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setMode("add-position")}
          aria-label={`Add a position at ${employer.name}`}
          className={`mt-2 ${textButton}`}
        >
          + Add position
        </button>
      )}

      <ConfirmDialog
        open={confirmingDelete}
        title={`Delete ${employer.name}?`}
        confirmLabel="Delete employer"
        pending={del.isPending}
        onCancel={() => setConfirmingDelete(false)}
        onConfirm={() =>
          del.mutate(employer.id, {
            onSettled: () => setConfirmingDelete(false),
          })
        }
      >
        This removes the employer from your career. It has no roles under it, so
        nothing else goes with it.
      </ConfirmDialog>
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
