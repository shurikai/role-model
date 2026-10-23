import type { ReactNode } from "react";

/**
 * What a list says when there is nothing in it yet: what would be here, and
 * the one thing to do about it. Built for the career view (#118), where an
 * empty record is the first thing a new account sees and has to point
 * somewhere rather than just read as blank.
 */
export function EmptyState({
  title,
  children,
  action,
}: {
  title: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="border border-dashed border-rail bg-card p-6">
      <p className="mb-1 font-display text-base font-bold text-ink">{title}</p>
      {children && (
        <div className="font-body text-[13px] text-ink-dim">{children}</div>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
