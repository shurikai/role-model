import { Link } from "react-router-dom";

/**
 * The frame Login and Signup share.
 *
 * These two pages were 90% identical markup before, down to the field
 * classes, and the halves that differed were the halves that mattered. They
 * sit outside RequireAuth so they cannot use AppShell — an unauthenticated
 * page has nothing to navigate to — which is why the full-page background
 * lives here rather than being inherited.
 */
export function AuthCard({
  eyebrow,
  title,
  intro,
  children,
  footer,
}: {
  eyebrow: string;
  title: string;
  intro?: string;
  children: React.ReactNode;
  footer: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen flex-col bg-paper">
      <div className="mx-auto w-full max-w-sm px-6 py-20">
        <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
          {eyebrow}
        </p>
        <h1 className="mb-1 font-display text-2xl font-bold text-ink">
          {title}
        </h1>
        {intro && (
          <p className="mb-6 font-body text-[13px] text-ink-dim">{intro}</p>
        )}
        <div className={intro ? "" : "mt-6"}>{children}</div>
        <div className="mt-5 border-t border-dashed border-rail pt-4">
          {footer}
        </div>
      </div>
    </div>
  );
}

/** A labelled text input, in the vocabulary the import screens established. */
export function Field({
  id,
  label,
  type,
  value,
  onChange,
  required,
  autoComplete,
  optional,
}: {
  id: string;
  label: string;
  type: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
  autoComplete?: string;
  optional?: boolean;
}) {
  return (
    <div className="mb-4">
      <label
        htmlFor={id}
        className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
      >
        {label}
        {optional && <span className="text-ink-dim"> (optional)</span>}
      </label>
      <input
        id={id}
        type={type}
        required={required}
        autoComplete={autoComplete}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full border border-border bg-surface px-3 py-2 font-body text-sm text-ink disabled:opacity-60"
      />
    </div>
  );
}

/** The primary action, matching the import screens' submit button. */
export function SubmitButton({
  pending,
  pendingLabel,
  children,
  full,
  disabled,
  title,
}: {
  pending?: boolean;
  pendingLabel?: string;
  children: React.ReactNode;
  full?: boolean;
  disabled?: boolean;
  title?: string;
}) {
  return (
    <button
      type="submit"
      disabled={pending || disabled}
      title={title}
      className={`bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50 ${
        full ? "w-full" : ""
      }`}
    >
      {pending && pendingLabel ? pendingLabel : children}
    </button>
  );
}

/** A secondary navigation link, as used beside the import screens' actions. */
export function QuietLink({
  to,
  children,
}: {
  to: string;
  children: React.ReactNode;
}) {
  return (
    <Link to={to} className="font-body text-sm text-ink-dim underline">
      {children}
    </Link>
  );
}
