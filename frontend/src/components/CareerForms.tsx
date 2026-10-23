import { useId, useState, type ReactNode } from "react";
import {
  useCareerLevels,
  useCreateContribution,
  useCreatePosition,
  useUpdateContribution,
  useUpdateEmployer,
  useUpdatePosition,
} from "../hooks/useCareer";
import { formatApiError } from "../lib/api-client";
import {
  contributionUpdate,
  employerUpdate,
  fromMonth,
  nullable,
  positionUpdate,
  toMonth,
} from "../lib/career-edit";
import type { Contribution, Employer, Position } from "../lib/types";

/**
 * The edit and add forms for the career view (#118). Each one owns its
 * mutation, and every update body is built by laying the form over the
 * fetched entity (lib/career-edit), because the endpoints replace the whole
 * row and a field the form does not show would otherwise be nulled.
 */

/** The small inline action buttons beside a heading or on a card. */
export const textButton =
  "font-body text-[13px] text-verify hover:underline disabled:opacity-50";

const inputClass =
  "mt-1 w-full border border-border bg-surface px-2 py-1.5 font-body text-sm text-ink";

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="mb-3">
      <label htmlFor={id} className="font-body text-xs text-ink-dim">
        {label}
        {hint && <span className="ml-1 text-rail">({hint})</span>}
      </label>
      {children(id)}
    </div>
  );
}

function FormShell({
  heading,
  onSubmit,
  onCancel,
  pending,
  submitLabel,
  error,
  children,
}: {
  heading: string;
  onSubmit: () => void;
  onCancel: () => void;
  pending: boolean;
  submitLabel: string;
  error: string | null;
  children: ReactNode;
}) {
  return (
    <form
      aria-label={heading}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      className="border border-verify bg-card p-4"
    >
      <p className="mb-3 font-mono text-[10px] tracking-widest text-rail uppercase">
        {heading}
      </p>
      {children}
      {error && (
        <p role="alert" className="mb-3 font-body text-sm text-reject">
          {error}
        </p>
      )}
      <div className="flex gap-3">
        <button
          type="submit"
          disabled={pending}
          className="bg-ink px-4 py-2 font-display text-sm font-bold text-surface disabled:opacity-50"
        >
          {pending ? "Saving…" : submitLabel}
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

export function EmployerForm({
  employer,
  onDone,
}: {
  employer: Employer;
  onDone: () => void;
}) {
  const update = useUpdateEmployer();
  const [name, setName] = useState(employer.name);
  const [industry, setIndustry] = useState(employer.industry ?? "");
  const [notes, setNotes] = useState(employer.notes ?? "");
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setError(null);
    if (!name.trim()) {
      setError("An employer name is required.");
      return;
    }
    try {
      await update.mutateAsync({
        id: employer.id,
        ...employerUpdate(employer, {
          name: name.trim(),
          industry: nullable(industry),
          notes: nullable(notes),
        }),
      });
      onDone();
    } catch (err) {
      setError(formatApiError(err));
    }
  }

  return (
    <FormShell
      heading="Edit employer"
      submitLabel="Save employer"
      onSubmit={submit}
      onCancel={onDone}
      pending={update.isPending}
      error={error}
    >
      <Field label="Name">
        {(id) => (
          <input
            id={id}
            value={name}
            onChange={(e) => setName(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <Field label="Industry">
        {(id) => (
          <input
            id={id}
            value={industry}
            onChange={(e) => setIndustry(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <Field label="Notes">
        {(id) => (
          <textarea
            id={id}
            rows={2}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
    </FormShell>
  );
}

/**
 * Edits `position` when given one, otherwise adds a position under
 * `employerID`. Every field of the update request is here except sort_order,
 * which has no control and goes back as it came.
 */
export function PositionForm({
  position,
  employerID,
  onDone,
}: {
  position?: Position;
  employerID: string;
  onDone: () => void;
}) {
  const update = useUpdatePosition();
  const create = useCreatePosition();
  const levels = useCareerLevels().data ?? [];

  const [title, setTitle] = useState(position?.title ?? "");
  const [industryRole, setIndustryRole] = useState(
    position?.industry_role ?? "",
  );
  const [industryLevel, setIndustryLevel] = useState(
    position?.industry_level ?? "",
  );
  const [location, setLocation] = useState(position?.location ?? "");
  const [started, setStarted] = useState(toMonth(position?.started_on ?? null));
  const [ended, setEnded] = useState(toMonth(position?.ended_on ?? null));
  const [context, setContext] = useState(position?.context_narrative ?? "");
  const [rationale, setRationale] = useState(position?.level_rationale ?? "");
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setError(null);
    if (!title.trim()) {
      setError("A title is required.");
      return;
    }
    // The handler 400s without a start date; catching it here keeps the
    // message specific.
    if (!started) {
      setError("A start month is required.");
      return;
    }
    if (ended && ended < started) {
      setError("The end month is before the start month.");
      return;
    }

    const fields = {
      title: title.trim(),
      industry_role: nullable(industryRole),
      industry_level: nullable(industryLevel),
      location: nullable(location),
      started_on: fromMonth(started, position?.started_on ?? null)!,
      ended_on: fromMonth(ended, position?.ended_on ?? null),
      context_narrative: nullable(context),
      level_rationale: nullable(rationale),
    };
    try {
      if (position) {
        await update.mutateAsync({
          id: position.id,
          ...positionUpdate(position, fields),
        });
      } else {
        await create.mutateAsync({ employer_id: employerID, ...fields });
      }
      onDone();
    } catch (err) {
      setError(formatApiError(err));
    }
  }

  const levelKnown =
    industryLevel === "" || levels.some((l) => l.value === industryLevel);

  return (
    <FormShell
      heading={position ? "Edit position" : "New position"}
      submitLabel={position ? "Save position" : "Add position"}
      onSubmit={submit}
      onCancel={onDone}
      pending={update.isPending || create.isPending}
      error={error}
    >
      <Field label="Title" hint="as the employer wrote it">
        {(id) => (
          <input
            id={id}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <Field label="On resumes as" hint="optional; the title is used if blank">
        {(id) => (
          <input
            id={id}
            value={industryRole}
            onChange={(e) => setIndustryRole(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <div className="grid gap-x-4 sm:grid-cols-2">
        <Field label="Level">
          {(id) => (
            <select
              id={id}
              value={industryLevel}
              onChange={(e) => setIndustryLevel(e.target.value)}
              className={inputClass}
            >
              <option value="">Not set</option>
              {!levelKnown && (
                // A value not on the current ladder still has to be
                // selectable, or opening the form would silently rewrite it.
                <option value={industryLevel}>{industryLevel}</option>
              )}
              {levels.map((l) => (
                <option key={l.value} value={l.value}>
                  {l.label}
                </option>
              ))}
            </select>
          )}
        </Field>
        <Field label="Location">
          {(id) => (
            <input
              id={id}
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              className={inputClass}
            />
          )}
        </Field>
        <Field label="Started">
          {(id) => (
            <input
              id={id}
              type="month"
              value={started}
              onChange={(e) => setStarted(e.target.value)}
              className={inputClass}
            />
          )}
        </Field>
        <Field label="Ended" hint="leave empty if current">
          {(id) => (
            <input
              id={id}
              type="month"
              value={ended}
              onChange={(e) => setEnded(e.target.value)}
              className={inputClass}
            />
          )}
        </Field>
      </div>
      <Field label="Context" hint="the team, the setting, the scale">
        {(id) => (
          <textarea
            id={id}
            rows={3}
            value={context}
            onChange={(e) => setContext(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <Field label="Why this level">
        {(id) => (
          <textarea
            id={id}
            rows={2}
            value={rationale}
            onChange={(e) => setRationale(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
    </FormShell>
  );
}

/**
 * Edits `contribution` when given one, otherwise adds one under `positionID`.
 * is_active is not a field here: hiding is its own one-click control on the
 * card, and an edit keeps whatever it was.
 */
export function ContributionForm({
  contribution,
  positionID,
  onDone,
}: {
  contribution?: Contribution;
  positionID: string;
  onDone: () => void;
}) {
  const update = useUpdateContribution(positionID);
  const create = useCreateContribution(positionID);

  const [summary, setSummary] = useState(contribution?.summary ?? "");
  const [full, setFull] = useState(contribution?.full_description ?? "");
  const [outcomes, setOutcomes] = useState(contribution?.outcomes ?? "");
  const [scale, setScale] = useState(contribution?.scale_context ?? "");
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setError(null);
    if (!summary.trim() || !full.trim()) {
      setError("A summary and a full description are both required.");
      return;
    }
    const fields = {
      summary: summary.trim(),
      full_description: full.trim(),
      outcomes: nullable(outcomes),
      scale_context: nullable(scale),
    };
    try {
      if (contribution) {
        await update.mutateAsync({
          id: contribution.id,
          ...contributionUpdate(contribution, fields),
        });
      } else {
        await create.mutateAsync(fields);
      }
      onDone();
    } catch (err) {
      setError(formatApiError(err));
    }
  }

  return (
    <FormShell
      heading={contribution ? "Edit contribution" : "New contribution"}
      submitLabel={contribution ? "Save contribution" : "Add contribution"}
      onSubmit={submit}
      onCancel={onDone}
      pending={update.isPending || create.isPending}
      error={error}
    >
      <Field label="Summary" hint="one line, what you did">
        {(id) => (
          <textarea
            id={id}
            rows={2}
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <Field label="Full description">
        {(id) => (
          <textarea
            id={id}
            rows={4}
            value={full}
            onChange={(e) => setFull(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <Field label="Outcomes" hint="what changed because of it">
        {(id) => (
          <textarea
            id={id}
            rows={2}
            value={outcomes}
            onChange={(e) => setOutcomes(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
      <Field label="Scale" hint="how big: people, volume, money, reach">
        {(id) => (
          <textarea
            id={id}
            rows={2}
            value={scale}
            onChange={(e) => setScale(e.target.value)}
            className={inputClass}
          />
        )}
      </Field>
    </FormShell>
  );
}
