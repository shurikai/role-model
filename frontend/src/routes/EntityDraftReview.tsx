import { useMemo, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { Check, TriangleAlert, X, Pencil } from "lucide-react";
import {
  useApproveEntityDraft,
  useEntityDrafts,
  useRejectEntityDraft,
  useResolveBatch,
  useUpdateEntityDraft,
} from "../hooks/useIntake";
import { isBatchWorking, useImportBatch } from "../hooks/useImport";
import { PayloadEditor } from "../components/PayloadEditor";
import { formatApiError } from "../lib/api-client";
import type { EntityDraft, EntityDraftStatus } from "../lib/types";

/**
 * Review queue for the wide import path: a whole career staged as drafts, with
 * the dependency edges that say what has to be written before what.
 *
 * The tree is built from `depends_on`, never from the order the list arrives
 * in. Extraction emits whatever order the model produced, and the resolver
 * exists precisely because that order means nothing.
 */

const KIND_LABEL: Record<string, string> = {
  employer: "Employer",
  position: "Position",
  contribution: "Contribution",
  skill: "Skill",
  preference: "Preference",
};

const STATUS_STYLE: Record<
  EntityDraftStatus,
  { label: string; badge: string; bar: string }
> = {
  pending: { label: "Pending", badge: "bg-rail", bar: "border-l-border" },
  approved: { label: "Approved", badge: "bg-stamp", bar: "border-l-stamp" },
  rejected: { label: "Rejected", badge: "bg-reject", bar: "border-l-reject" },
};

/** A draft's headline, pulled from whichever payload field carries its name. */
function draftTitle(draft: EntityDraft): string {
  const p = draft.payload ?? {};
  const first = (...keys: string[]) => {
    for (const k of keys) {
      const v = p[k];
      if (typeof v === "string" && v.trim() !== "") return v;
    }
    return null;
  };
  return (
    first("name", "title", "summary", "label", "tag") ?? `${draft.kind} draft`
  );
}

function flagLines(draft: EntityDraft): string[] {
  const flags = draft.flags;
  if (!flags) return [];
  const lines: string[] = [];
  for (const c of flags.preference_collisions ?? []) {
    lines.push(`Collides with an existing preference: ${c}`);
  }
  for (const c of flags.new_categories ?? []) {
    lines.push(
      `Would create a new category "${c}", which starts with no competency vocabulary behind it.`,
    );
  }
  return lines;
}

/**
 * Everything that would be orphaned by rejecting this draft, following
 * `depends_on` transitively — rejecting an employer strands its positions, and
 * each of those strands its own contributions.
 *
 * Computed here rather than asked for, because the whole batch is already
 * loaded and the edges are on it. Nothing is cascaded: the resolver reports
 * orphans by name at the next resolve, and this exists so the reviewer knows
 * that before deciding rather than after.
 */
function dependentsOf(draftID: string, drafts: EntityDraft[]): EntityDraft[] {
  const found = new Map<string, EntityDraft>();
  let frontier = [draftID];

  while (frontier.length > 0) {
    const next: string[] = [];
    for (const draft of drafts) {
      if (found.has(draft.id) || draft.id === draftID) continue;
      if (draft.status !== "pending") continue;
      if (!(draft.depends_on ?? []).some((dep) => frontier.includes(dep))) {
        continue;
      }
      found.set(draft.id, draft);
      next.push(draft.id);
    }
    frontier = next;
  }
  return [...found.values()];
}

function describeDependents(dependents: EntityDraft[]): string {
  const counts = new Map<string, number>();
  for (const d of dependents) {
    counts.set(d.kind, (counts.get(d.kind) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([kind, n]) => `${n} ${n === 1 ? kind : `${kind}s`}`)
    .join(" and ");
}

export function EntityDraftReview() {
  const { batchID } = useParams<{ batchID: string }>();
  const draftsQuery = useEntityDrafts(batchID);
  const batchQuery = useImportBatch(batchID);
  const resolveBatch = useResolveBatch(batchID);

  const drafts = useMemo(() => draftsQuery.data ?? [], [draftsQuery.data]);

  const grouped = useMemo(() => {
    const byID = new Map(drafts.map((d) => [d.id, d]));
    const employers = drafts.filter((d) => d.kind === "employer");
    const positions = drafts.filter((d) => d.kind === "position");
    const contributions = drafts.filter((d) => d.kind === "contribution");

    const parentOf = (draft: EntityDraft, parentKind: string) =>
      (draft.depends_on ?? []).find(
        (dep) => byID.get(dep)?.kind === parentKind,
      ) ?? null;

    const positionsByEmployer = new Map<string, EntityDraft[]>();
    const orphanPositions: EntityDraft[] = [];
    for (const position of positions) {
      const employerID = parentOf(position, "employer");
      if (!employerID) {
        orphanPositions.push(position);
        continue;
      }
      positionsByEmployer.set(employerID, [
        ...(positionsByEmployer.get(employerID) ?? []),
        position,
      ]);
    }

    const contributionsByPosition = new Map<string, EntityDraft[]>();
    const orphanContributions: EntityDraft[] = [];
    for (const contribution of contributions) {
      const positionID = parentOf(contribution, "position");
      if (!positionID) {
        orphanContributions.push(contribution);
        continue;
      }
      contributionsByPosition.set(positionID, [
        ...(contributionsByPosition.get(positionID) ?? []),
        contribution,
      ]);
    }

    // Anything this screen has no place for still has to appear. A draft that
    // vanishes from the queue without becoming a row is the exact failure the
    // whole draft substrate exists to prevent, and `kind` is deliberately not
    // a database CHECK — so an unknown kind is expected, not impossible.
    const known = new Set([
      "employer",
      "position",
      "contribution",
      "skill",
      "preference",
    ]);
    const unplaced = [
      ...orphanPositions,
      ...orphanContributions,
      ...drafts.filter((d) => !known.has(d.kind)),
    ];

    return {
      employers,
      positionsByEmployer,
      contributionsByPosition,
      skills: drafts.filter((d) => d.kind === "skill"),
      preferences: drafts.filter((d) => d.kind === "preference"),
      unplaced,
    };
  }, [drafts]);

  const pendingCount = drafts.filter((d) => d.status === "pending").length;
  const batch = batchQuery.data;
  const working = batch ? isBatchWorking(batch.status) : false;

  return (
    <div className="min-h-screen bg-paper">
      <div className="mx-auto max-w-4xl px-6 py-10">
        <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
          Career import · Review
        </p>
        <div className="mb-1 flex items-baseline justify-between gap-4">
          <h1 className="font-display text-2xl font-bold text-ink">
            Review your career
          </h1>
          {drafts.length > 0 && (
            <span className="font-mono text-[13px] text-ink-dim">
              {drafts.length - pendingCount} / {drafts.length} decided
            </span>
          )}
        </div>
        <p className="mb-6 font-body text-[13px] text-ink-dim">
          Nothing here is in your record yet. Approve what is right, fix what is
          not, reject what should not be there.
        </p>

        {working && (
          <p className="border border-border bg-card p-4 font-body text-sm text-ink-dim">
            Still extracting. This page refreshes itself.
          </p>
        )}
        {batch?.status === "failed" && (
          <p className="border border-reject bg-card p-4 font-body text-sm text-ink">
            Import failed.{" "}
            <span className="text-ink-dim">
              {batch.error_text ?? "No reason was recorded."}
            </span>
          </p>
        )}

        {draftsQuery.isLoading && (
          <p className="font-body text-sm text-ink-dim">Loading drafts…</p>
        )}
        {draftsQuery.error && (
          <p className="font-body text-sm text-reject">
            {formatApiError(draftsQuery.error)}
          </p>
        )}
        {!draftsQuery.isLoading && drafts.length === 0 && !working && (
          <p className="border border-border bg-card p-4 font-body text-sm text-ink-dim">
            This batch produced no drafts.
          </p>
        )}

        {grouped.employers.map((employer) => (
          <section key={employer.id} className="mb-6">
            <DraftCard draft={employer} drafts={drafts} batchID={batchID} />
            <div className="mt-2 ml-6 border-l border-border pl-4">
              {(grouped.positionsByEmployer.get(employer.id) ?? []).map(
                (position) => (
                  <PositionBlock
                    key={position.id}
                    position={position}
                    contributions={
                      grouped.contributionsByPosition.get(position.id) ?? []
                    }
                    drafts={drafts}
                    batchID={batchID}
                  />
                ),
              )}
              {(grouped.positionsByEmployer.get(employer.id) ?? []).length ===
                0 && (
                <p className="font-body text-sm text-ink-dim">
                  No positions drafted for this employer.
                </p>
              )}
            </div>
          </section>
        ))}

        <FlatGroup
          title="Skills"
          drafts={grouped.skills}
          all={drafts}
          batchID={batchID}
        />
        <FlatGroup
          title="Preferences"
          drafts={grouped.preferences}
          all={drafts}
          batchID={batchID}
        />
        <FlatGroup
          title="Not attached to anything"
          drafts={grouped.unplaced}
          all={drafts}
          batchID={batchID}
          note="These name a parent that is not in this batch, or a kind this screen does not know. They are shown so nothing disappears unreviewed."
        />

        <div className="mt-8 border-t border-dashed border-rail pt-5">
          <button
            type="button"
            disabled={resolveBatch.isPending || pendingCount === 0}
            onClick={() => resolveBatch.mutate()}
            className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50"
          >
            {resolveBatch.isPending
              ? "Writing…"
              : `Approve all ${pendingCount} remaining`}
          </button>
          <Link
            to="/applications"
            className="ml-4 font-body text-sm text-ink-dim underline"
          >
            Done for now
          </Link>

          {resolveBatch.error && (
            <p className="mt-3 font-body text-sm text-reject">
              {formatApiError(resolveBatch.error)}
            </p>
          )}
          {resolveBatch.data && (
            <p className="mt-3 font-body text-sm text-ink-dim">
              Wrote {Object.keys(resolveBatch.data.resolved).length} rows.
              {Object.keys(resolveBatch.data.unresolved).length > 0 && (
                <span className="text-reject">
                  {" "}
                  {Object.keys(resolveBatch.data.unresolved).length} could not
                  be written — see the drafts still marked pending.
                </span>
              )}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

function PositionBlock({
  position,
  contributions,
  drafts,
  batchID,
}: {
  position: EntityDraft;
  contributions: EntityDraft[];
  drafts: EntityDraft[];
  batchID: string | undefined;
}) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="mb-4">
      <DraftCard draft={position} drafts={drafts} batchID={batchID} />
      {contributions.length > 0 && (
        <div className="mt-1 ml-4">
          <button
            type="button"
            onClick={() => setExpanded((e) => !e)}
            aria-expanded={expanded}
            className="font-mono text-[11px] tracking-wider text-verify uppercase"
          >
            {expanded ? "▾" : "▸"} {contributions.length}{" "}
            {contributions.length === 1 ? "contribution" : "contributions"}{" "}
            drafted
          </button>
          {expanded && (
            <div className="mt-2 border-l border-border pl-4">
              {contributions.map((contribution) => (
                <DraftCard
                  key={contribution.id}
                  draft={contribution}
                  drafts={drafts}
                  batchID={batchID}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function FlatGroup({
  title,
  drafts,
  all,
  batchID,
  note,
}: {
  title: string;
  drafts: EntityDraft[];
  all: EntityDraft[];
  batchID: string | undefined;
  note?: string;
}) {
  if (drafts.length === 0) return null;
  return (
    <section className="mb-6">
      <h2 className="mb-1 font-display text-lg font-bold text-ink">{title}</h2>
      {note && <p className="mb-2 font-body text-xs text-ink-dim">{note}</p>}
      {drafts.map((draft) => (
        <DraftCard
          key={draft.id}
          draft={draft}
          drafts={all}
          batchID={batchID}
        />
      ))}
    </section>
  );
}

function DraftCard({
  draft,
  drafts,
  batchID,
}: {
  draft: EntityDraft;
  drafts: EntityDraft[];
  batchID: string | undefined;
}) {
  const approve = useApproveEntityDraft(batchID);
  const reject = useRejectEntityDraft(batchID);
  const update = useUpdateEntityDraft(batchID);

  const [editing, setEditing] = useState(false);
  const [confirmingReject, setConfirmingReject] = useState(false);

  const style = STATUS_STYLE[draft.status] ?? STATUS_STYLE.pending;
  const flags = flagLines(draft);
  const pending = draft.status === "pending";
  const dependents = dependentsOf(draft.id, drafts);

  function rejectNow() {
    setConfirmingReject(false);
    reject.mutate(draft.id);
  }

  return (
    <div
      className={`mb-2 border-y border-r border-border border-l-[6px] bg-card ${style.bar} ${
        flags.length > 0 ? "border-t-flag border-r-flag border-b-flag" : ""
      } ${draft.status === "rejected" ? "opacity-60" : ""}`}
    >
      <div className="flex items-start justify-between gap-3 px-4 py-2.5">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-rail uppercase">
            {KIND_LABEL[draft.kind] ?? draft.kind}
          </p>
          <p className="font-display text-[15px] font-bold text-ink">
            {draftTitle(draft)}
          </p>
        </div>
        <span
          className={`px-2 py-1 font-mono text-[10px] tracking-wider text-surface uppercase ${style.badge}`}
        >
          {style.label}
        </span>
      </div>

      {flags.length > 0 && (
        <div className="border-t border-border px-4 py-2">
          {flags.map((line, i) => (
            <p
              key={i}
              className="flex items-start gap-2 font-body text-xs text-ink-dim"
            >
              <span className="mt-0.5 flex-shrink-0 text-flag">
                <TriangleAlert size={12} />
              </span>
              {line}
            </p>
          ))}
        </div>
      )}

      {approve.error && (
        <p className="border-t border-border px-4 py-2 font-body text-xs text-reject">
          {formatApiError(approve.error)}
        </p>
      )}
      {reject.error && (
        <p className="border-t border-border px-4 py-2 font-body text-xs text-reject">
          {formatApiError(reject.error)}
        </p>
      )}

      {editing && draft.payload && (
        <div className="px-4 pb-3">
          <PayloadEditor
            payload={draft.payload}
            busy={update.isPending}
            error={update.error ? formatApiError(update.error) : null}
            onCancel={() => setEditing(false)}
            onSave={(payload) =>
              update.mutate(
                { draftID: draft.id, payload },
                { onSuccess: () => setEditing(false) },
              )
            }
          />
        </div>
      )}

      {confirmingReject && (
        <div className="border-t border-border px-4 py-3">
          <p className="mb-2 font-body text-sm text-ink">
            {describeDependents(dependents)} depend on this. Rejecting it leaves{" "}
            {dependents.length === 1 ? "it" : "them"} with nothing to attach to,
            and {dependents.length === 1 ? "it" : "they"} will be reported as
            unresolved instead of being written.
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={rejectNow}
              className="bg-reject px-3 py-1.5 font-body text-xs font-medium text-surface"
            >
              Reject anyway
            </button>
            <button
              type="button"
              onClick={() => setConfirmingReject(false)}
              className="px-3 py-1.5 font-body text-xs text-rail"
            >
              Keep it
            </button>
          </div>
        </div>
      )}

      {pending && !editing && !confirmingReject && (
        <div className="flex gap-2 border-t border-border px-4 py-2">
          <button
            type="button"
            disabled={approve.isPending}
            onClick={() => approve.mutate(draft.id)}
            className="flex items-center gap-1.5 bg-stamp px-3 py-1.5 font-body text-xs font-medium text-surface disabled:opacity-50"
          >
            <Check size={13} /> Approve
          </button>
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="flex items-center gap-1.5 border border-ink px-3 py-1.5 font-body text-xs font-medium text-ink"
          >
            <Pencil size={13} /> Edit
          </button>
          <button
            type="button"
            disabled={reject.isPending}
            onClick={() =>
              dependents.length > 0 ? setConfirmingReject(true) : rejectNow()
            }
            className="flex items-center gap-1.5 border border-border px-3 py-1.5 font-body text-xs font-medium text-reject disabled:opacity-50"
          >
            <X size={13} /> Reject
          </button>
        </div>
      )}
    </div>
  );
}
