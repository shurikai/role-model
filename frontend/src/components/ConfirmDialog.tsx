import { useEffect, useRef, type ReactNode } from "react";

/**
 * A modal yes/no for actions that cannot be undone. Built for PR C's
 * contribution delete, which is a hard delete — tags, project links and
 * feedback go with it — so the dialog's job is to say so before it happens.
 *
 * Reversible actions should not use this. Hiding a contribution from resumes
 * is one click with no confirm, because a confirm on something you can take
 * back trains people to click through the ones you cannot.
 *
 * Focus lands on Cancel, not on the destructive button, so a stray Enter
 * keeps the data. Escape and a click on the backdrop both cancel.
 */
export function ConfirmDialog({
  open,
  title,
  children,
  confirmLabel,
  pending = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  children?: ReactNode;
  confirmLabel: string;
  pending?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    cancelRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onCancel();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onCancel]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 px-4"
      onClick={onCancel}
    >
      <div
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        className="w-full max-w-md border border-border bg-surface p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <h2
          id="confirm-dialog-title"
          className="mb-2 font-display text-lg font-bold text-ink"
        >
          {title}
        </h2>
        {children && (
          <div className="mb-5 font-body text-sm text-ink-dim">{children}</div>
        )}
        <div className="flex justify-end gap-3">
          <button
            ref={cancelRef}
            type="button"
            onClick={onCancel}
            className="border border-border bg-surface px-4 py-2 font-body text-sm text-ink-dim"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={pending}
            className="bg-reject px-4 py-2 font-display text-sm font-bold text-surface disabled:opacity-50"
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
