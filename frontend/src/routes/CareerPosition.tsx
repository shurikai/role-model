import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Page } from "../components/Page";
import { useCareerTree, usePositionContributions } from "../hooks/useCareer";
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
 */
export function CareerPosition() {
  const { positionID } = useParams<{ positionID: string }>();
  const tree = useCareerTree();
  const contributions = usePositionContributions(positionID);

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

  return (
    <Page
      eyebrow={
        <Breadcrumb employer={employer.name} position={position.title} />
      }
      title={position.title}
      lede={
        <>
          {employer.name} · {formatSpan(position.started_on, position.ended_on)}
          {position.location && ` · ${position.location}`}
        </>
      }
    >
      <PositionDetails
        industryRole={position.industry_role}
        industryLevel={position.industry_level}
        context={position.context_narrative}
      />

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
          Nothing recorded for this role yet.{" "}
          <Link to="/career/add" className="text-ink underline">
            Add to your career
          </Link>
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
            <ContributionCard contribution={c} />
          </li>
        ))}
      </ul>

      {hidden.length > 0 && <HiddenContributions contributions={hidden} />}
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

function ContributionCard({ contribution }: { contribution: Contribution }) {
  const [open, setOpen] = useState(false);
  const details: [string, string | null][] = [
    ["Full description", contribution.full_description],
    ["Outcomes", contribution.outcomes],
    ["Scale", contribution.scale_context],
  ];
  const present = details.filter(([, v]) => v && v.trim() !== "");

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
      {present.length > 0 && (
        <>
          <button
            type="button"
            aria-expanded={open}
            onClick={() => setOpen((o) => !o)}
            className="mt-3 font-body text-[13px] text-verify hover:underline"
          >
            {open ? "Less" : "More detail"}
          </button>
          {open && (
            <dl className="mt-2 flex flex-col gap-2 border-t border-dashed border-rail pt-3 font-body text-[13px]">
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
        </>
      )}
    </article>
  );
}

function HiddenContributions({
  contributions,
}: {
  contributions: Contribution[];
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
              <ContributionCard contribution={c} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
