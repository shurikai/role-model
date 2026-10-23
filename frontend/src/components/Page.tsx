import type { ReactNode } from "react";

const WIDTHS = {
  narrow: "max-w-3xl",
  default: "max-w-4xl",
  wide: "max-w-5xl",
} as const;

/**
 * The column every route renders in, and the eyebrow / heading / lede that
 * opens it.
 *
 * Every route used to own this by hand — `mx-auto max-w-* px-6 py-10`, the
 * mono eyebrow, the h1 — and the copies had drifted: Profile's lede sat 8px
 * lower than everyone else's, and ApplicationDetail's loading state was a
 * narrower column than the page it was loading.
 *
 * `actions` sits on the heading's baseline, right-aligned: a primary button,
 * or a progress counter on the review screens. `aside` renders outside the
 * padded column, to its left — ImportReview's ledger rail is why it exists.
 * The header is omitted when there is neither eyebrow nor title, which is what
 * a loading or not-found state wants.
 */
export function Page({
  width = "default",
  eyebrow,
  title,
  actions,
  lede,
  aside,
  children,
}: {
  width?: keyof typeof WIDTHS;
  eyebrow?: ReactNode;
  title?: ReactNode;
  actions?: ReactNode;
  lede?: ReactNode;
  aside?: ReactNode;
  children?: ReactNode;
}) {
  const column = (
    <>
      {(eyebrow || title) && (
        <header className="mb-6">
          {eyebrow && (
            <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
              {eyebrow}
            </p>
          )}
          {title && (
            <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
              <h1 className="font-display text-2xl font-bold text-ink">
                {title}
              </h1>
              {actions}
            </div>
          )}
          {lede && (
            <div className="mt-1 font-body text-[13px] text-ink-dim">
              {lede}
            </div>
          )}
        </header>
      )}
      {children}
    </>
  );

  if (aside) {
    return (
      <div className={`mx-auto flex ${WIDTHS[width]}`}>
        {aside}
        <div className="min-w-0 flex-1 px-6 py-10">{column}</div>
      </div>
    );
  }
  return <div className={`mx-auto ${WIDTHS[width]} px-6 py-10`}>{column}</div>;
}
