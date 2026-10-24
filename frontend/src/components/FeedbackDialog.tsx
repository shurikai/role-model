import { useEffect, useRef, useState } from "react";
import { useLocation } from "react-router-dom";
import { useSubmitFeedback } from "../hooks/useFeedback";
import { formatApiError } from "../lib/api-client";
import type { FeedbackCategory } from "../lib/types";

const CATEGORIES: { value: FeedbackCategory; label: string }[] = [
  { value: "bug", label: "Something broke" },
  { value: "confusing", label: "Something was confusing" },
  { value: "idea", label: "An idea" },
];

/** The frontend build, set at build time; "dev" when nothing set it. */
export function appVersion(): string {
  return import.meta.env.VITE_APP_VERSION?.trim() || "dev";
}

/**
 * Product feedback from inside the app (#167): "this screen confused me", not
 * feedback on a generated resume.
 *
 * A report is only actionable if it says where the tester was and which
 * build they were on. The app knows both, so they are attached here — the
 * route, the viewport, the build — and never shown or typed.
 *
 * The modal is ConfirmDialog's: the same backdrop, panel and Escape/backdrop
 * cancel, without a second modal system. Focus lands on the first category,
 * since nothing here is destructive.
 */
export function FeedbackDialog({
  open,
  onClose,
  onSent,
}: {
  open: boolean;
  onClose: () => void;
  onSent: () => void;
}) {
  const location = useLocation();
  const submit = useSubmitFeedback();
  const [category, setCategory] = useState<FeedbackCategory | null>(null);
  const [message, setMessage] = useState("");
  const firstRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    firstRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  const canSend =
    category !== null && message.trim() !== "" && !submit.isPending;

  function send(e: React.FormEvent) {
    e.preventDefault();
    if (!canSend || category === null) return;
    submit.mutate(
      {
        category,
        message: message.trim(),
        route: location.pathname + location.search,
        app_version: appVersion(),
        viewport: `${window.innerWidth}x${window.innerHeight}`,
      },
      {
        onSuccess: () => {
          setCategory(null);
          setMessage("");
          submit.reset();
          onSent();
        },
      },
    );
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 px-4"
      onClick={onClose}
    >
      <form
        role="dialog"
        aria-modal="true"
        aria-labelledby="feedback-dialog-title"
        onSubmit={send}
        className="w-full max-w-md border border-border bg-surface p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <h2
          id="feedback-dialog-title"
          className="mb-3 font-display text-lg font-bold text-ink"
        >
          Send feedback
        </h2>

        <fieldset className="mb-3">
          <legend className="sr-only">What kind of feedback</legend>
          {CATEGORIES.map((c, i) => (
            <label
              key={c.value}
              className="mb-1 flex items-center gap-2 font-body text-sm text-ink"
            >
              <input
                ref={i === 0 ? firstRef : undefined}
                type="radio"
                name="feedback-category"
                value={c.value}
                checked={category === c.value}
                onChange={() => setCategory(c.value)}
              />
              {c.label}
            </label>
          ))}
        </fieldset>

        <label htmlFor="feedback-message" className="sr-only">
          What happened
        </label>
        <textarea
          id="feedback-message"
          rows={5}
          maxLength={4000}
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="What happened?"
          className="w-full border border-border bg-card px-2 py-1.5 font-body text-sm text-ink"
        />
        <p className="mt-1 mb-4 font-body text-xs text-ink-dim">
          We&apos;ll include which page you were on.
        </p>

        {submit.error && (
          <p role="alert" className="mb-3 font-body text-sm text-reject">
            {formatApiError(submit.error)}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="border border-border bg-surface px-4 py-2 font-body text-sm text-ink-dim"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={!canSend}
            className="bg-ink px-4 py-2 font-display text-sm font-bold text-surface disabled:opacity-50"
          >
            {submit.isPending ? "Sending…" : "Send"}
          </button>
        </div>
      </form>
    </div>
  );
}
