import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ChevronDown, ChevronRight } from "lucide-react";
import {
  ContributionForm,
  PositionForm,
  textButton,
} from "../components/CareerForms";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { Page } from "../components/Page";
import {
  useCareerTree,
  useDeleteContribution,
  useDeletePosition,
  usePositionContributions,
  useUpdateContribution,
} from "../hooks/useCareer";
import { contributionUpdate } from "../lib/career-edit";
import { formatApiError } from "../lib/api-client";
import { formatSpan } from "../lib/dates";
import type { Contribution } from "../lib/types";

/**
 * One position and everything done in it. Two requests: the career tree
 * (usually already cached from /career) for the position and its employer,
 * and the position's contributions with their tags.
 *
 * Hidden contributions are kept but never reach a generated resume. They are
 * folded away under their own heading, not dropped, so the page still shows
 * everything that was written.
 *
 * Hiding is the primary way to take a contribution off resumes: one click,
 * no confirm, reversible. Delete is permanent and sits behind ConfirmDialog.
 * That ordering is deliberate — what an interview or an import wrote wrong is
 * usually wrong-ish, not junk.
 */
export function CareerPosition() {
  const { positionID } = useParams<{ positionID: string }>();
  const tree = useCareerTree();
  const contributions = usePositionContributions(positionID);
  const navigate = useNavigate();
  const deletePosition = useDeletePosition();
  const [editing, setEditing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const employer = tree.data?.employers.find((e) =>
    e.positions.some((p) => p.id === positionID),
  );
  const position = employer?.positions.find((p) => p.id === positionID);

  if (tree.isLoading) {
    return (
      <Page>
        <p className="font-body text-sm text-ink-dim">Loading…</p>
      </Page>
    );
  }
  if (tree.error) {
    return (
      <Page>
        <p className="font-body text-sm text-reject">
          {formatApiError(tree.error)}
        </p>
      </Page>
    );
  }
  if (!employer || !position) {
    return (
      <Page eyebrow={<Breadcrumb />} title="Role not found">
        <p className="font-body text-sm text-ink-dim">
          This role is not in your career.{" "}
          <Link to="/career" className="text-ink underline">
            Back to your career
          </Link>
        </p>
      </Page>
    );
  }

  const all = contributions.data ?? [];
  const active = all.filter((c) => c.is_active);
  const hidden = all.filter((c) => !c.is_active);
  // A position with contributions, hidden ones included, cannot be deleted
  // (the server 409s), so the control is not offered.
  const { active: nActive, hidden: nHidden } = position.contribution_counts;
  const deletable = nActive + nHidden === 0 && all.length === 0;

  return (
    <Page
      eyebrow={
        <Breadcrumb employer={employer.name} position={position.title} />
      }
      title={position.title}
      actions={
        !editing && (
          <div className="flex gap-3">
            <button
              type="button"
              onClick={() => setEditing(true)}
              className={textButton}
            >
              Edit position
            </button>
            {deletable && (
              <button
                type="button"
                onClick={() => setConfirmingDelete(true)}
                className="font-body text-[13px] text-reject hover:underline"
              >
                Delete position
              </button>
            )}
          </div>
        )
      }
      lede={
        <>
          {employer.name} · {formatSpan(position.started_on, position.ended_on)}
          {position.location && ` · ${position.location}`}
        </>
      }
    >
      {editing ? (
        <div className="mb-8">
          <PositionForm
            position={position}
            employerID={employer.id}
            onDone={() => setEditing(false)}
          />
        </div>
      ) : (
        <PositionDetails
          industryRole={position.industry_role}
          industryLevel={position.industry_level}
          context={position.context_narrative}
        />
      )}
      {deletePosition.error && (
        <p role="alert" className="mb-4 font-body text-sm text-reject">
          {formatApiError(deletePosition.error)}
        </p>
      )}

      <h2 className="mb-2 font-display text-lg font-bold text-ink">
        What you did here
      </h2>
      {contributions.isLoading && (
        <p className="font-body text-sm text-ink-dim">Loading contributions…</p>
      )}
      {contributions.error && (
        <p className="font-body text-sm text-reject">
          {formatApiError(contributions.error)}
        </p>
      )}
      {contributions.data && all.length === 0 && (
        <p className="border border-dashed border-rail bg-card p-4 font-body text-[13px] text-ink-dim">
          Nothing recorded for this role yet. Add one below, or{" "}
          <Link to="/career/add" className="text-ink underline">
            import several at once
          </Link>
          .
        </p>
      )}
      {contributions.data && all.length > 0 && active.length === 0 && (
        <p className="mb-3 font-body text-[13px] text-ink-dim">
          Every contribution here is hidden from resumes.
        </p>
      )}
      <ul className="flex flex-col gap-3">
        {active.map((c) => (
          <li key={c.id}>
            <ContributionCard contribution={c} positionID={position.id} />
          </li>
        ))}
      </ul>

      {contributions.data &&
        (adding ? (
          <div className="mt-3">
            <ContributionForm
              positionID={position.id}
              onDone={() => setAdding(false)}
            />
          </div>
        ) : (
          <button
            type="button"
            onClick={() => setAdding(true)}
            className={`mt-3 ${textButton}`}
          >
            + Add contribution
          </button>
        ))}

      {hidden.length > 0 && (
        <HiddenContributions contributions={hidden} positionID={position.id} />
      )}

      <ConfirmDialog
        open={confirmingDelete}
        title={`Delete ${position.title}?`}
        confirmLabel="Delete position"
        pending={deletePosition.isPending}
        onCancel={() => setConfirmingDelete(false)}
        onConfirm={() =>
          deletePosition.mutate(position, {
            onSuccess: () => navigate("/career"),
            onSettled: () => setConfirmingDelete(false),
          })
        }
      >
        This removes the role from your career. Nothing is recorded under it, so
        nothing else goes with it.
      </ConfirmDialog>
    </Page>
  );
}

function Breadcrumb({
  employer,
  position,
}: {
  employer?: string;
  position?: string;
}) {
  return (
    <nav aria-label="Breadcrumb">
      <Link to="/career" className="hover:underline">
        Career
      </Link>
      {employer && <> / {employer}</>}
      {position && <> / {position}</>}
    </nav>
  );
}

/**
 * The normalized role is what a rendered resume prints, with the verbatim
 * title as the fallback, so it is worth showing when it says something the
 * title does not.
 */
function PositionDetails({
  industryRole,
  industryLevel,
  context,
}: {
  industryRole: string | null;
  industryLevel: string | null;
  context: string | null;
}) {
  if (!industryRole && !industryLevel && !context) return null;
  return (
    <dl className="mb-8 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 font-body text-[13px]">
      {industryRole && (
        <>
          <dt className="text-ink-dim">On resumes as</dt>
          <dd className="text-ink">{industryRole}</dd>
        </>
      )}
      {industryLevel && (
        <>
          <dt className="text-ink-dim">Level</dt>
          <dd className="text-ink">{industryLevel}</dd>
        </>
      )}
      {context && (
        <>
          <dt className="text-ink-dim">Context</dt>
          <dd className="whitespace-pre-line text-ink">{context}</dd>
        </>
      )}
    </dl>
  );
}

function ContributionCard({
  contribution,
  positionID,
}: {
  contribution: Contribution;
  positionID: string;
}) {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const update = useUpdateContribution(positionID);
  const del = useDeleteContribution(positionID);

  if (editing) {
    return (
      <ContributionForm
        contribution={contribution}
        positionID={positionID}
        onDone={() => setEditing(false)}
      />
    );
  }

  const details: [string, string | null][] = [
    ["Full description", contribution.full_description],
    ["Outcomes", contribution.outcomes],
    ["Scale", contribution.scale_context],
  ];
  const present = details.filter(([, v]) => v && v.trim() !== "");
  const error = update.error ?? del.error;

  return (
    <article className="border border-border bg-card p-4">
      <p className="font-body text-sm text-ink">{contribution.summary}</p>
      {contribution.tags.length > 0 && (
        <ul aria-label="Tags" className="mt-2 flex flex-wrap gap-1.5">
          {contribution.tags.map((t) => (
            <li
              key={t.id}
              title={t.category}
              className="border border-border bg-surface px-2 py-0.5 font-mono text-[11px] text-ink-dim"
            >
              {t.name}
            </li>
          ))}
        </ul>
      )}
      {open && present.length > 0 && (
        <dl className="mt-3 flex flex-col gap-2 border-t border-dashed border-rail pt-3 font-body text-[13px]">
          {present.map(([label, value]) => (
            <div key={label}>
              <dt className="font-mono text-[11px] tracking-widest text-ink-dim uppercase">
                {label}
              </dt>
              <dd className="whitespace-pre-line text-ink">{value}</dd>
            </div>
          ))}
        </dl>
      )}
      {error && (
        <p role="alert" className="mt-2 font-body text-sm text-reject">
          {formatApiError(error)}
        </p>
      )}
      <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
        {present.length > 0 && (
          <button
            type="button"
            aria-expanded={open}
            onClick={() => setOpen((o) => !o)}
            className={textButton}
          >
            {open ? "Less" : "More detail"}
          </button>
        )}
        <button
          type="button"
          onClick={() => setEditing(true)}
          className={textButton}
        >
          Edit
        </button>
        <button
          type="button"
          disabled={update.isPending}
          onClick={() =>
            update.mutate({
              id: contribution.id,
              ...contributionUpdate(contribution, {
                is_active: !contribution.is_active,
              }),
            })
          }
          className={textButton}
        >
          {contribution.is_active ? "Hide from resumes" : "Show on resumes"}
        </button>
        <button
          type="button"
          onClick={() => setConfirmingDelete(true)}
          className="font-body text-[13px] text-reject hover:underline"
        >
          Delete
        </button>
      </div>

      <ConfirmDialog
        open={confirmingDelete}
        title="Delete this contribution?"
        confirmLabel="Delete permanently"
        pending={del.isPending}
        onCancel={() => setConfirmingDelete(false)}
        onConfirm={() =>
          del.mutate(contribution.id, {
            onSettled: () => setConfirmingDelete(false),
          })
        }
      >
        This cannot be undone: the contribution, its tags and any feedback on it
        are removed for good. To keep it off resumes but keep the record, hide
        it instead.
      </ConfirmDialog>
    </article>
  );
}

function HiddenContributions({
  contributions,
  positionID,
}: {
  contributions: Contribution[];
  positionID: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <section className="mt-8">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-1 font-display text-base font-bold text-ink-dim hover:text-ink"
      >
        {open ? (
          <ChevronDown aria-hidden size={16} />
        ) : (
          <ChevronRight aria-hidden size={16} />
        )}
        Hidden from resumes ({contributions.length})
      </button>
      {open && (
        <ul className="mt-3 flex flex-col gap-3 opacity-75">
          {contributions.map((c) => (
            <li key={c.id}>
              <ContributionCard contribution={c} positionID={positionID} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
