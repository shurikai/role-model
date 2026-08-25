import { useState } from "react";

/**
 * Edits one entity draft's payload.
 *
 * Derived from the payload object rather than written five times, once per
 * kind. The five payload shapes differ in their fields but not in their
 * types — strings, a number, a boolean, and two collections — and a per-kind
 * form would be five components to keep in step with five Go structs. This
 * renders whatever the draft actually carries, which also means a kind added
 * later is editable the day it is added rather than the day someone writes its
 * form.
 *
 * It always emits a COMPLETE object: it starts from the payload it was given
 * and replaces values in place. The endpoint replaces rather than merges and
 * rejects fields it does not know for that kind, so a partial object would
 * silently drop whatever it omitted.
 */

/**
 * Dependency pointers, which name another DRAFT rather than carrying data.
 * Shown, because a reviewer should see that a position is attached to
 * something, but not editable: retyping a draft id by hand is how a
 * contribution ends up parented to nothing, and the graph is the one thing in
 * this payload that is not the person's to hand-edit.
 */
const READ_ONLY_KEYS = new Set(["employer_draft", "position_draft"]);

interface PayloadEditorProps {
  payload: Record<string, unknown>;
  onSave: (payload: Record<string, unknown>) => void;
  onCancel: () => void;
  busy?: boolean;
  /** Server-side rejection, e.g. a 422 from the payload validator. */
  error?: string | null;
}

type Draftable = Record<string, string>;

/** Collections round-trip as JSON text; scalars round-trip as themselves. */
function toText(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value, null, 2);
  return String(value);
}

function isCollection(value: unknown): boolean {
  return typeof value === "object" && value !== null;
}

export function PayloadEditor({
  payload,
  onSave,
  onCancel,
  busy = false,
  error,
}: PayloadEditorProps) {
  const [fields, setFields] = useState<Draftable>(() => {
    const initial: Draftable = {};
    for (const [key, value] of Object.entries(payload)) {
      initial[key] = toText(value);
    }
    return initial;
  });
  const [localError, setLocalError] = useState<string | null>(null);

  function set(key: string, value: string) {
    setFields((prev) => ({ ...prev, [key]: value }));
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setLocalError(null);

    const next: Record<string, unknown> = {};
    for (const [key, original] of Object.entries(payload)) {
      const text = fields[key] ?? "";

      if (isCollection(original)) {
        if (text.trim() === "") {
          next[key] = null;
          continue;
        }
        try {
          next[key] = JSON.parse(text);
        } catch {
          setLocalError(`${key} is not valid JSON.`);
          return;
        }
        continue;
      }

      if (typeof original === "number") {
        if (text.trim() === "") {
          next[key] = null;
          continue;
        }
        const n = Number(text);
        if (Number.isNaN(n)) {
          setLocalError(`${key} must be a number.`);
          return;
        }
        next[key] = n;
        continue;
      }

      if (typeof original === "boolean") {
        next[key] = text === "true";
        continue;
      }

      // Strings, and nulls that were strings. An emptied field goes back as
      // null rather than "": the Go payloads use *string for every optional
      // field, and "" would store an empty string where the absence was meant.
      next[key] = text === "" ? null : text;
    }

    onSave(next);
  }

  return (
    <form
      onSubmit={submit}
      className="mt-3 border border-border bg-surface p-3"
    >
      {Object.entries(payload).map(([key, original]) => {
        const readOnly = READ_ONLY_KEYS.has(key);
        const id = `payload-${key}`;
        const collection = isCollection(original);

        return (
          <div key={key} className="mb-2">
            <label
              htmlFor={id}
              className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
            >
              {key.replace(/_/g, " ")}
              {readOnly && <span className="ml-2 text-rail">(linked)</span>}
            </label>

            {typeof original === "boolean" ? (
              <select
                id={id}
                value={fields[key]}
                disabled={readOnly || busy}
                onChange={(e) => set(key, e.target.value)}
                className="w-full border border-border bg-card px-2 py-1 font-body text-sm text-ink"
              >
                <option value="true">true</option>
                <option value="false">false</option>
              </select>
            ) : collection ? (
              <textarea
                id={id}
                value={fields[key]}
                rows={4}
                disabled={readOnly || busy}
                onChange={(e) => set(key, e.target.value)}
                className="w-full border border-border bg-card px-2 py-1 font-mono text-xs text-ink disabled:opacity-60"
              />
            ) : (
              <input
                id={id}
                value={fields[key]}
                disabled={readOnly || busy}
                onChange={(e) => set(key, e.target.value)}
                className="w-full border border-border bg-card px-2 py-1 font-body text-sm text-ink disabled:opacity-60"
              />
            )}
          </div>
        );
      })}

      {(localError || error) && (
        <p className="mb-2 font-body text-sm text-reject">
          {localError ?? error}
        </p>
      )}

      <div className="flex gap-2">
        <button
          type="submit"
          disabled={busy}
          className="bg-verify px-3 py-1.5 font-body text-xs font-medium text-surface disabled:opacity-50"
        >
          {busy ? "Saving…" : "Save"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-3 py-1.5 font-body text-xs text-rail"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
