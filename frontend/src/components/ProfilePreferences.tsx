import { useState } from "react";
import {
  usePreferences,
  useCreatePreference,
  useUpdatePreference,
  useDeletePreference,
} from "../hooks/useProfile";
import { formatApiError } from "../lib/api-client";
import type {
  Preference,
  PreferenceRequest,
  PreferenceType,
  PreferenceSentiment,
} from "../lib/types";

/**
 * What each type is checked against, in the posting.
 *
 * Shown in the form rather than buried in a doc because routing is the thing
 * that goes wrong: a row typed by topic rather than by what it is *about* is
 * checked against a field that can never answer it, and then reads as an
 * unmet preference forever. The dealbreaker/core_practice split is the one
 * that misfires most, so it is stated as the question to ask yourself.
 */
const TYPE_HELP: Record<PreferenceType, string> = {
  domain:
    "The subject matter or industry — checked against what the posting says it does.",
  role_shape:
    "The shape of the job itself — team size, ownership, on-call, contact with people.",
  culture: "How the organisation works — remote, async, process-heavy.",
  dealbreaker:
    "A thing whose PRESENCE is the objection. “I will not work anywhere that does X.”",
  core_practice:
    "A thing whose PROMINENCE is the objection. “Some X is fine; a job that is mostly X is not.”",
};

const TYPES: PreferenceType[] = [
  "domain",
  "role_shape",
  "culture",
  "dealbreaker",
  "core_practice",
];

const TYPE_LABEL: Record<PreferenceType, string> = {
  domain: "Domain",
  role_shape: "Role shape",
  culture: "Culture",
  dealbreaker: "Dealbreaker",
  core_practice: "Core practice",
};

function emptyDraft(): PreferenceRequest {
  return {
    preference_type: "domain",
    label: "",
    aliases: [],
    sentiment: "positive",
    weight: 5,
    is_hard_gate: false,
  };
}

function toDraft(p: Preference): PreferenceRequest {
  return {
    preference_type: p.preference_type,
    label: p.label,
    aliases: p.aliases ?? [],
    sentiment: p.sentiment,
    weight: p.weight,
    is_hard_gate: p.is_hard_gate,
    context_type: p.context_type,
    notes: p.notes,
  };
}

export function ProfilePreferences() {
  const { data: preferences, isLoading, error } = usePreferences();
  const [adding, setAdding] = useState(false);
  const [editingID, setEditingID] = useState<string | null>(null);

  const byType = new Map<PreferenceType, Preference[]>();
  for (const p of preferences ?? []) {
    byType.set(p.preference_type, [
      ...(byType.get(p.preference_type) ?? []),
      p,
    ]);
  }

  return (
    <section className="mb-10">
      <div className="mb-1 flex flex-wrap items-baseline justify-between gap-3">
        <h2 className="font-display text-lg font-bold text-ink">Preferences</h2>
        {!adding && (
          <button
            type="button"
            onClick={() => setAdding(true)}
            className="bg-ink px-4 py-1.5 font-display text-sm font-bold text-surface"
          >
            Add a preference
          </button>
        )}
      </div>
      <p className="mb-4 font-body text-[13px] text-ink-dim">
        What you want and what you will not do. The fit gate reports these
        against every posting — as matches, as things the posting is silent
        about, as conflicts, and separately as dealbreakers it tripped.
      </p>

      {isLoading && <p className="font-body text-sm text-ink-dim">Loading…</p>}
      {error && (
        <p className="border border-reject bg-card p-4 font-body text-sm text-ink">
          {formatApiError(error)}
        </p>
      )}

      {adding && (
        <PreferenceForm
          initial={emptyDraft()}
          onDone={() => setAdding(false)}
          onCancel={() => setAdding(false)}
        />
      )}

      {preferences && preferences.length === 0 && !adding && (
        <div className="border border-border bg-card p-4">
          <p className="font-body text-sm text-ink-dim">
            Nothing stated yet, so the preference half of every fit report will
            be empty. A career import proposes these from what you said you
            wanted; you can also add them here.
          </p>
        </div>
      )}

      {TYPES.filter((t) => byType.has(t)).map((type) => (
        <div key={type} className="mb-5">
          <p className="mb-1 font-mono text-[10px] tracking-widest text-rail uppercase">
            {TYPE_LABEL[type]}
          </p>
          {(byType.get(type) ?? []).map((p) =>
            editingID === p.id ? (
              <PreferenceForm
                key={p.id}
                id={p.id}
                initial={toDraft(p)}
                onDone={() => setEditingID(null)}
                onCancel={() => setEditingID(null)}
              />
            ) : (
              <PreferenceRow
                key={p.id}
                preference={p}
                onEdit={() => setEditingID(p.id)}
              />
            ),
          )}
        </div>
      ))}
    </section>
  );
}

function PreferenceRow({
  preference,
  onEdit,
}: {
  preference: Preference;
  onEdit: () => void;
}) {
  const del = useDeletePreference();
  const [confirming, setConfirming] = useState(false);
  const negative = preference.sentiment === "negative";

  return (
    <div
      className={`mb-2 border-y border-r border-l-[6px] border-border bg-card px-4 py-2.5 ${
        negative ? "border-l-reject" : "border-l-stamp"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-display text-[15px] font-bold text-ink">
            {preference.label}
          </p>
          <p className="font-body text-xs text-ink-dim">
            {negative ? "Negative" : "Positive"} · weight {preference.weight}
            {preference.is_hard_gate && " · dealbreaker"}
          </p>
          {preference.aliases && preference.aliases.length > 0 ? (
            <p className="mt-1 font-body text-xs text-ink-dim">
              also matches: {preference.aliases.join(", ")}
            </p>
          ) : (
            // Not decoration. A preference with no aliases matches only its
            // own wording, and postings rarely use yours.
            <p className="mt-1 font-body text-xs text-flag">
              no aliases — matches only this exact wording
            </p>
          )}
        </div>
        <div className="flex flex-shrink-0 items-center gap-2">
          {preference.is_hard_gate && (
            <span className="bg-reject px-2 py-1 font-mono text-[10px] tracking-wider text-surface uppercase">
              Gate
            </span>
          )}
          <button
            type="button"
            onClick={onEdit}
            className="border border-border bg-surface px-3 py-1 font-body text-xs text-ink-dim"
          >
            Edit
          </button>
          {confirming ? (
            <>
              <button
                type="button"
                onClick={() => del.mutate(preference.id)}
                disabled={del.isPending}
                className="bg-reject px-3 py-1 font-display text-xs font-bold text-surface disabled:opacity-50"
              >
                {del.isPending ? "Deleting…" : "Really delete"}
              </button>
              <button
                type="button"
                onClick={() => setConfirming(false)}
                className="font-body text-xs text-ink-dim underline"
              >
                Keep
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={() => setConfirming(true)}
              className="border border-border bg-surface px-3 py-1 font-body text-xs text-ink-dim"
            >
              Delete
            </button>
          )}
        </div>
      </div>
      {del.isError && (
        <p className="mt-2 font-body text-xs text-reject">
          {formatApiError(del.error)}
        </p>
      )}
    </div>
  );
}

function PreferenceForm({
  id,
  initial,
  onDone,
  onCancel,
}: {
  id?: string;
  initial: PreferenceRequest;
  onDone: () => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState<PreferenceRequest>(initial);
  const [aliasText, setAliasText] = useState(initial.aliases.join(", "));
  const create = useCreatePreference();
  const update = useUpdatePreference();
  const pending = create.isPending || update.isPending;
  const error = create.error ?? update.error;

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const body: PreferenceRequest = {
      ...draft,
      aliases: aliasText
        .split(",")
        .map((a) => a.trim())
        .filter(Boolean),
    };
    const done = { onSuccess: onDone };
    if (id) {
      update.mutate({ id, ...body }, done);
    } else {
      create.mutate(body, done);
    }
  }

  return (
    <form
      onSubmit={submit}
      className="mb-3 border border-verify bg-card px-4 py-3"
    >
      <div className="mb-3">
        <label
          htmlFor={`pref-label-${id ?? "new"}`}
          className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
        >
          What it is
        </label>
        <input
          id={`pref-label-${id ?? "new"}`}
          required
          value={draft.label}
          onChange={(e) => setDraft({ ...draft, label: e.target.value })}
          placeholder="ambulatory quality improvement"
          className="w-full border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
        />
        <p className="mt-1 font-body text-xs text-ink-dim">
          Write the thing itself, not its negation — “night shifts”, not “no
          night shifts”. Two labels that share vocabulary with their opposite
          both match the same posting.
        </p>
      </div>

      <div className="mb-3 flex flex-wrap gap-3">
        <div>
          <label
            htmlFor={`pref-type-${id ?? "new"}`}
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Checked against
          </label>
          <select
            id={`pref-type-${id ?? "new"}`}
            value={draft.preference_type}
            onChange={(e) =>
              setDraft({
                ...draft,
                preference_type: e.target.value as PreferenceType,
              })
            }
            className="border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
          >
            {TYPES.map((t) => (
              <option key={t} value={t}>
                {TYPE_LABEL[t]}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label
            htmlFor={`pref-sentiment-${id ?? "new"}`}
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Want or avoid
          </label>
          <select
            id={`pref-sentiment-${id ?? "new"}`}
            value={draft.sentiment}
            onChange={(e) =>
              setDraft({
                ...draft,
                sentiment: e.target.value as PreferenceSentiment,
              })
            }
            className="border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
          >
            <option value="positive">Want</option>
            <option value="negative">Avoid</option>
          </select>
        </div>

        <div>
          <label
            htmlFor={`pref-weight-${id ?? "new"}`}
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            How much (1–10)
          </label>
          <input
            id={`pref-weight-${id ?? "new"}`}
            type="number"
            min={1}
            max={10}
            value={draft.weight}
            onChange={(e) =>
              setDraft({ ...draft, weight: Number(e.target.value) })
            }
            className="w-24 border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
          />
        </div>
      </div>

      <p className="mb-3 font-body text-xs text-ink-dim">
        {TYPE_HELP[draft.preference_type]}
      </p>

      <div className="mb-3">
        <label
          htmlFor={`pref-aliases-${id ?? "new"}`}
          className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
        >
          Other wordings a posting might use
        </label>
        <input
          id={`pref-aliases-${id ?? "new"}`}
          value={aliasText}
          onChange={(e) => setAliasText(e.target.value)}
          placeholder="night shift, overnight rotation, night float"
          className="w-full border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
        />
        <p className="mt-1 font-body text-xs text-ink-dim">
          Comma separated, and worth the effort: this is what decides whether a
          preference ever matches. “inpatient nights” reaches a posting reading
          “three twelve-hour night shifts” only through these.
        </p>
      </div>

      <label className="mb-3 flex items-start gap-2 font-body text-sm text-ink">
        <input
          type="checkbox"
          checked={draft.is_hard_gate}
          onChange={(e) =>
            setDraft({ ...draft, is_hard_gate: e.target.checked })
          }
          className="mt-1"
        />
        <span>
          This one rules a job out
          <span className="block font-body text-xs text-ink-dim">
            Reported as a named dealbreaker rather than counted with the rest.
            It does not block anything — the fit report still runs and the
            resume still generates.
          </span>
        </span>
      </label>

      {error && (
        <p className="mb-2 font-body text-sm text-reject">
          {formatApiError(error)}
        </p>
      )}

      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={pending}
          className="bg-ink px-5 py-2 font-display text-sm font-bold text-surface disabled:opacity-50"
        >
          {pending ? "Saving…" : id ? "Save changes" : "Add preference"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="font-body text-sm text-ink-dim underline"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
