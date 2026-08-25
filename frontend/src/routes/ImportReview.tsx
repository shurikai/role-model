import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import {
  isBatchWorking,
  useApproveDraft,
  useImportBatch,
  useImportDrafts,
  useRejectDraft,
  useUpdateDraft,
} from "../hooks/useImport";
import { PositionPicker } from "../components/PositionPicker";
import {
  Check,
  ChevronDown,
  ChevronUp,
  CircleDashed,
  Lightbulb,
  Sparkles,
  SkipForward,
  TriangleAlert,
  X,
  type LucideIcon,
} from "lucide-react";
import { formatApiError } from "../lib/api-client";
import {
  DRAFT_EDITABLE_FIELDS,
  type ContributionDraft,
  type DraftEditableField,
  type DraftFlag,
  type DraftFlagType,
  type ImportBatch,
} from "../lib/types";

/*
 * Flag severity, highest first.
 *
 * This ordering is a judgment call and is not derived from anything in the
 * backend — the four types are a flat vocabulary there, with no rank attached.
 * The reasoning: a `warning` says the enrichment believes something is wrong,
 * an `inference` says it wrote something the source did not say (a claim you
 * could be asked about in an interview), a `gap` says something is missing,
 * and a `suggestion` is an offer you can ignore. Wrong beats invented beats
 * absent beats optional.
 *
 * The colours step with it — rail (quiet grey), verify (blue, informational),
 * flag (orange, wants attention), reject (red, stop). The palette's three
 * "status" colours alone would force two of the four types onto one colour;
 * borrowing verify for `gap` keeps four distinct steps, and blue reads as
 * information rather than fault, which is what a missing field is.
 *
 * The glyphs are deliberately a different vocabulary from the review-status
 * set (check / x / skip): a flag is a note about a field, not a verdict on the
 * entry, and reusing a checkmark here would read as one. Sparkles for
 * `inference` because it is the thing the model wrote rather than read.
 */
const FLAG_SEVERITY: readonly DraftFlagType[] = [
  "warning",
  "inference",
  "gap",
  "suggestion",
];

const FLAG_STYLE: Record<
  DraftFlagType,
  { text: string; bg: string; border: string; Icon: LucideIcon }
> = {
  warning: {
    text: "text-reject",
    bg: "bg-reject",
    border: "border-reject",
    Icon: TriangleAlert,
  },
  inference: {
    text: "text-flag",
    bg: "bg-flag",
    border: "border-flag",
    Icon: Sparkles,
  },
  gap: {
    text: "text-verify",
    bg: "bg-verify",
    border: "border-verify",
    Icon: CircleDashed,
  },
  suggestion: {
    text: "text-rail",
    bg: "bg-rail",
    border: "border-rail",
    Icon: Lightbulb,
  },
};

function highestSeverity(flags: DraftFlag[]): DraftFlagType | null {
  for (const type of FLAG_SEVERITY) {
    if (flags.some((f) => f.type === type)) return type;
  }
  return null;
}

const FIELD_LABEL: Record<DraftEditableField, string> = {
  summary: "Summary",
  full_description: "Full description",
  outcomes: "Outcomes",
  scale_context: "Scale context",
};

/**
 * What the card shows as its state. `skipped` is local-only — there is no such
 * status on the backend, and skipping deliberately writes nothing: it means
 * "not now", and a decision that survives a refresh would be a decision.
 */
type CardState = "pending" | "approved" | "rejected" | "skipped";

const STATE_STYLE: Record<
  CardState,
  {
    label: string;
    bar: string;
    badge: string;
    tick: string;
    Icon: LucideIcon;
  } | null
> = {
  pending: null,
  approved: {
    label: "Approved",
    bar: "border-l-stamp",
    badge: "bg-stamp",
    tick: "border-stamp bg-stamp",
    Icon: Check,
  },
  rejected: {
    label: "Rejected",
    bar: "border-l-reject",
    badge: "bg-reject",
    tick: "border-reject bg-reject",
    Icon: X,
  },
  skipped: {
    label: "Skipped",
    bar: "border-l-rail",
    badge: "bg-rail",
    tick: "border-rail bg-rail",
    Icon: SkipForward,
  },
};

export function ImportReview() {
  const { batchID } = useParams<{ batchID: string }>();
  const batchQuery = useImportBatch(batchID);
  const draftsQuery = useImportDrafts(batchID);

  // Local, per-session, never sent anywhere. See CardState.
  const [skipped, setSkipped] = useState<ReadonlySet<string>>(new Set());

  const batch = batchQuery.data;
  const drafts = draftsQuery.data ?? [];

  const reviewed = drafts.filter(
    (d) => d.status !== "pending" || skipped.has(d.id),
  ).length;

  return (
    <div className="min-h-screen bg-paper">
      <div className="mx-auto flex max-w-5xl">
        <LedgerRail drafts={drafts} skipped={skipped} />

        <div className="flex-1 px-6 py-10">
          <header className="mb-8">
            <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
              Stage 0 · Import review
            </p>
            <div className="mb-1 flex items-baseline justify-between gap-4">
              <h1 className="font-display text-2xl font-bold text-ink">
                Review imported entries
              </h1>
              {drafts.length > 0 && (
                <span className="font-mono text-[13px] text-ink-dim">
                  {reviewed} / {drafts.length} reviewed
                </span>
              )}
            </div>
            <BatchStatusLine batch={batch} />
            {batchQuery.error && (
              <p className="font-body text-sm text-reject">
                {formatApiError(batchQuery.error)}
              </p>
            )}
          </header>

          {batch && isBatchWorking(batch.status) && (
            <p className="border border-border bg-card p-4 font-body text-sm text-ink-dim">
              Still working — extraction and enrichment are running. This page
              refreshes itself.
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

          {batch &&
            !isBatchWorking(batch.status) &&
            batch.status !== "failed" && (
              <>
                {draftsQuery.isLoading && (
                  <p className="font-body text-sm text-ink-dim">
                    Loading drafts…
                  </p>
                )}
                {draftsQuery.error && (
                  <p className="font-body text-sm text-reject">
                    {formatApiError(draftsQuery.error)}
                  </p>
                )}
                {drafts.length === 0 && !draftsQuery.isLoading && (
                  <p className="border border-border bg-card p-4 font-body text-sm text-ink-dim">
                    This batch produced no drafts.
                  </p>
                )}
                {drafts.map((draft) => (
                  <DraftCard
                    key={draft.id}
                    draft={draft}
                    batchID={batchID}
                    skipped={skipped.has(draft.id)}
                    onSkip={() =>
                      setSkipped((prev) => new Set(prev).add(draft.id))
                    }
                  />
                ))}
              </>
            )}

          <div className="mt-8 border-t border-dashed border-rail pt-5">
            <Link
              to="/applications"
              className="font-body text-sm text-ink-dim underline"
            >
              Back to applications
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}

function BatchStatusLine({ batch }: { batch: ImportBatch | undefined }) {
  if (!batch) {
    return <p className="font-body text-[13px] text-ink-dim">Loading batch…</p>;
  }
  const { total, pending, approved, rejected } = batch.draft_counts;
  return (
    <p className="font-body text-[13px] text-ink-dim">
      <span className="font-mono text-[11px] tracking-widest uppercase">
        {batch.status}
      </span>{" "}
      · {total} {total === 1 ? "entry" : "entries"} · {pending} pending,{" "}
      {approved} approved, {rejected} rejected
    </p>
  );
}

function LedgerRail({
  drafts,
  skipped,
}: {
  drafts: ContributionDraft[];
  skipped: ReadonlySet<string>;
}) {
  if (drafts.length === 0) return null;
  return (
    <div className="hidden w-8 flex-shrink-0 flex-col items-center pt-[168px] pr-4 md:flex">
      {drafts.map((draft) => {
        const state = cardState(draft, skipped.has(draft.id));
        const style = STATE_STYLE[state];
        return (
          <div key={draft.id} className="flex flex-col items-center">
            <div
              className={`h-2.5 w-2.5 border ${
                style ? style.tick : "border-rail bg-transparent"
              }`}
            />
            <div className="h-32 w-px bg-border" />
          </div>
        );
      })}
    </div>
  );
}

function cardState(draft: ContributionDraft, isSkipped: boolean): CardState {
  if (draft.status === "approved") return "approved";
  if (draft.status === "rejected") return "rejected";
  return isSkipped ? "skipped" : "pending";
}

function DraftCard({
  draft,
  batchID,
  skipped,
  onSkip,
}: {
  draft: ContributionDraft;
  batchID: string | undefined;
  skipped: boolean;
  onSkip: () => void;
}) {
  const [expanded, setExpanded] = useState(true);
  const [picking, setPicking] = useState(false);

  const updateDraft = useUpdateDraft(batchID);
  const approveDraft = useApproveDraft(batchID);
  const rejectDraft = useRejectDraft(batchID);

  const state = cardState(draft, skipped);
  const style = STATE_STYLE[state];
  const decided = state !== "pending";
  const flags = draft.flags ?? [];
  const worst = highestSeverity(flags);

  const actionError =
    approveDraft.error ?? rejectDraft.error ?? updateDraft.error;

  return (
    <div
      className={`relative mb-5 border-y border-r border-border bg-card ${
        style ? style.bar : "border-l-border"
      } border-l-[6px] ${
        state === "rejected" || state === "skipped" ? "opacity-60" : ""
      }`}
    >
      <div className="border-b border-border px-5 py-3">
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <div className="flex min-w-[52px] flex-shrink-0 flex-col items-start gap-1.5">
              <span className="font-mono text-[11px] text-rail">
                #{draft.id.slice(0, 4)}
              </span>
              <FlagSummary flags={flags} worst={worst} />
            </div>
            <div className="flex flex-col items-start gap-0.5">
              <span className="font-display text-[15px] font-bold text-ink">
                {draft.employer_name}
              </span>
              <span className="font-body text-[13px] text-ink-dim">
                {draft.position_title}
              </span>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setExpanded((e) => !e)}
            aria-expanded={expanded}
            aria-label={expanded ? "Collapse entry" : "Expand entry"}
            className="text-rail"
          >
            {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </button>
        </div>

        {style && (
          <div className="mt-2 pl-[64px]">
            <span
              className={`inline-flex items-center gap-1 px-2 py-1 font-mono text-[10px] font-medium tracking-wider text-surface uppercase ${style.badge}`}
            >
              <style.Icon size={11} strokeWidth={2.5} />
              {style.label}
            </span>
          </div>
        )}
      </div>

      {expanded && (
        <div className="px-5 py-4">
          {DRAFT_EDITABLE_FIELDS.map((field) => (
            <DraftField
              key={field}
              draft={draft}
              field={field}
              flags={flags.filter((f) => f.field === field)}
              disabled={decided}
              onSave={(value) =>
                updateDraft.mutate({
                  draftID: draft.id,
                  changes: { [field]: value },
                })
              }
            />
          ))}

          <FieldFlags
            label="About this entry"
            flags={flags.filter(
              (f) =>
                f.field === "general" ||
                f.field === "employer_name" ||
                f.field === "position_title",
            )}
          />

          {actionError && (
            <p className="mt-3 font-body text-sm text-reject">
              {formatApiError(actionError)}
            </p>
          )}

          {picking && (
            <div className="mt-4">
              <PositionPicker
                suggestedEmployerName={draft.employer_name}
                suggestedPositionTitle={draft.position_title}
                busy={approveDraft.isPending}
                onCancel={() => setPicking(false)}
                onResolved={(positionID) =>
                  approveDraft.mutate(
                    { draftID: draft.id, positionID },
                    { onSuccess: () => setPicking(false) },
                  )
                }
              />
            </div>
          )}

          {!decided && !picking && (
            <div className="flex gap-2 border-t border-border pt-3">
              <button
                type="button"
                onClick={() => setPicking(true)}
                className="flex items-center gap-1.5 bg-stamp px-3 py-1.5 font-body text-xs font-medium text-surface"
              >
                <Check size={13} /> Approve
              </button>
              <button
                type="button"
                disabled={rejectDraft.isPending}
                onClick={() => rejectDraft.mutate(draft.id)}
                className="flex items-center gap-1.5 border border-border px-3 py-1.5 font-body text-xs font-medium text-reject disabled:opacity-50"
              >
                <X size={13} /> Reject
              </button>
              <button
                type="button"
                onClick={onSkip}
                className="ml-auto flex items-center gap-1.5 px-3 py-1.5 font-body text-xs text-rail"
              >
                <SkipForward size={13} /> Skip for now
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * What sits where the reference mockup put a confidence badge.
 *
 * It is a flag count and it is named one. There is no confidence signal in
 * this pipeline, and deriving a percentage from flag counts would look like
 * one — a number the backend never computed, that a reader would reasonably
 * trust.
 */
function FlagSummary({
  flags,
  worst,
}: {
  flags: DraftFlag[];
  worst: DraftFlagType | null;
}) {
  if (flags.length === 0 || !worst) {
    return (
      <span className="px-2 py-0.5 font-mono text-[10px] tracking-wider text-stamp uppercase">
        clean
      </span>
    );
  }
  return (
    <span
      className={`px-2 py-0.5 font-mono text-[10px] tracking-wider text-surface uppercase ${FLAG_STYLE[worst].bg}`}
      title={`Most severe: ${worst}`}
    >
      {flags.length} {flags.length === 1 ? "flag" : "flags"}
    </span>
  );
}

function DraftField({
  draft,
  field,
  flags,
  disabled,
  onSave,
}: {
  draft: ContributionDraft;
  field: DraftEditableField;
  flags: DraftFlag[];
  disabled: boolean;
  onSave: (value: string | null) => void;
}) {
  const stored = draft[field] ?? "";
  const [value, setValue] = useState(stored);

  // Save on blur rather than on a debounce timer: one request per field the
  // user actually touched, and no half-typed sentence racing a keystroke.
  function handleBlur() {
    if (value === stored) return;
    onSave(value.trim() === "" ? null : value);
  }

  const fieldID = `${draft.id}-${field}`;

  return (
    <div className="mb-4">
      <label
        htmlFor={fieldID}
        className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
      >
        {FIELD_LABEL[field]}
      </label>
      <textarea
        id={fieldID}
        value={value}
        disabled={disabled}
        rows={field === "full_description" ? 5 : 2}
        onChange={(e) => setValue(e.target.value)}
        onBlur={handleBlur}
        className="w-full border border-border bg-surface p-3 font-body text-sm text-ink disabled:opacity-70"
      />
      <FieldFlags flags={flags} />
    </div>
  );
}

function FieldFlags({ flags, label }: { flags: DraftFlag[]; label?: string }) {
  if (flags.length === 0) return null;
  return (
    <div className="mt-2">
      {label && (
        <p className="mb-1 font-mono text-[10px] tracking-widest text-rail uppercase">
          {label}
        </p>
      )}
      {flags.map((flag, i) => {
        const style = FLAG_STYLE[flag.type];
        return (
          <div
            key={i}
            className={`mt-1 flex items-start gap-2 border-l-2 border-dashed pl-2 ${style.border}`}
          >
            <span className={`mt-0.5 flex-shrink-0 ${style.text}`}>
              <style.Icon size={12} />
            </span>
            <span
              className={`font-mono text-[10px] tracking-wider uppercase ${style.text}`}
            >
              {flag.type}
            </span>
            <span className="font-body text-xs text-ink-dim">
              {flag.message}
            </span>
          </div>
        );
      })}
    </div>
  );
}
