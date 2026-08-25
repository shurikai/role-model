import { useMemo, useState } from "react";
import {
  useCreateEmployer,
  useCreatePosition,
  useEmployerPositions,
  useEmployers,
} from "../hooks/useCareer";
import { formatApiError } from "../lib/api-client";
import type { Employer, Position } from "../lib/types";

/**
 * Resolves a `position_id`, creating the employer and position if they do not
 * exist yet.
 *
 * The create path is the common one, not the fallback. Approving an imported
 * contribution needs a position row that already exists — there is no
 * lookup-or-create on the backend — and on a fresh account nothing exists at
 * all, so an import of fourteen contributions across five jobs starts with
 * zero of five matchable. A picker that could only pick would be unusable on
 * exactly the accounts this import exists to fill.
 *
 * Deliberately knows nothing about drafts or Stage 0. It takes two strings to
 * pre-fill from and hands back an id.
 */
interface PositionPickerProps {
  /** Extracted employer name, used to pre-select or pre-fill. */
  suggestedEmployerName?: string;
  /** Extracted position title, used to pre-select or pre-fill. */
  suggestedPositionTitle?: string;
  onResolved: (positionID: string) => void;
  onCancel?: () => void;
  /** Set while the parent is doing something with the resolved id. */
  busy?: boolean;
}

function sameName(a: string, b: string): boolean {
  return a.trim().toLowerCase() === b.trim().toLowerCase();
}

function positionDates(position: Position): string {
  const start = position.started_on ?? "?";
  const end = position.ended_on ?? "present";
  return `${start} → ${end}`;
}

export function PositionPicker({
  suggestedEmployerName = "",
  suggestedPositionTitle = "",
  onResolved,
  onCancel,
  busy = false,
}: PositionPickerProps) {
  const employersQuery = useEmployers();
  const employers = useMemo(
    () => employersQuery.data ?? [],
    [employersQuery.data],
  );

  const [search, setSearch] = useState(suggestedEmployerName);
  const [selectedEmployerID, setSelectedEmployerID] = useState<string | null>(
    null,
  );
  const [creating, setCreating] = useState(false);

  // A name match is a suggestion, not a decision: it pre-selects the employer
  // so the common case is a confirm, and the user can still pick another.
  const suggestedEmployer: Employer | undefined = useMemo(
    () =>
      suggestedEmployerName
        ? employers.find((e) => sameName(e.name, suggestedEmployerName))
        : undefined,
    [employers, suggestedEmployerName],
  );

  const employerID = selectedEmployerID ?? suggestedEmployer?.id ?? null;
  const positionsQuery = useEmployerPositions(employerID ?? undefined);
  const positions = positionsQuery.data ?? [];

  const filteredEmployers = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return employers;
    return employers.filter((e) => e.name.toLowerCase().includes(q));
  }, [employers, search]);

  const selectedEmployer = employers.find((e) => e.id === employerID);

  if (creating) {
    return (
      <CreatePositionForm
        employer={selectedEmployer}
        suggestedEmployerName={suggestedEmployerName}
        suggestedPositionTitle={suggestedPositionTitle}
        onCreated={onResolved}
        onBack={() => setCreating(false)}
        busy={busy}
      />
    );
  }

  return (
    <div className="border border-border bg-surface p-4">
      <p className="mb-3 font-mono text-[10px] tracking-widest text-rail uppercase">
        Attach to position
      </p>

      {employersQuery.isLoading && (
        <p className="font-body text-sm text-ink-dim">Loading employers…</p>
      )}
      {employersQuery.error && (
        <p className="font-body text-sm text-reject">
          {formatApiError(employersQuery.error)}
        </p>
      )}

      {!employersQuery.isLoading && (
        <>
          <label
            className="font-body text-xs text-ink-dim"
            htmlFor="employer-search"
          >
            Employer
          </label>
          <input
            id="employer-search"
            type="search"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setSelectedEmployerID(null);
            }}
            placeholder="Search employers…"
            className="mt-1 mb-2 w-full border border-border bg-card px-2 py-1.5 font-body text-sm text-ink"
          />

          {filteredEmployers.length === 0 ? (
            <p className="mb-2 font-body text-sm text-ink-dim">
              {employers.length === 0
                ? "No employers yet."
                : "No employer matches that search."}
            </p>
          ) : (
            <ul className="mb-3 max-h-40 overflow-y-auto border border-border">
              {filteredEmployers.map((employer) => {
                const active = employer.id === employerID;
                return (
                  <li key={employer.id}>
                    <button
                      type="button"
                      onClick={() => setSelectedEmployerID(employer.id)}
                      aria-pressed={active}
                      className={`block w-full px-2 py-1.5 text-left font-body text-sm ${
                        active
                          ? "bg-verify text-surface"
                          : "bg-card text-ink hover:bg-surface"
                      }`}
                    >
                      {employer.name}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}

          {employerID && (
            <>
              <p className="font-body text-xs text-ink-dim">
                Position at {selectedEmployer?.name}
              </p>
              {positionsQuery.isLoading && (
                <p className="font-body text-sm text-ink-dim">
                  Loading positions…
                </p>
              )}
              {positions.length === 0 && !positionsQuery.isLoading && (
                <p className="mt-1 mb-2 font-body text-sm text-ink-dim">
                  This employer has no positions yet.
                </p>
              )}
              {positions.length > 0 && (
                <ul className="mt-1 mb-3 border border-border">
                  {positions.map((position) => (
                    <li key={position.id}>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => onResolved(position.id)}
                        className="flex w-full items-baseline justify-between bg-card px-2 py-1.5 text-left font-body text-sm text-ink hover:bg-surface disabled:opacity-50"
                      >
                        <span
                          className={
                            suggestedPositionTitle &&
                            sameName(position.title, suggestedPositionTitle)
                              ? "font-medium"
                              : ""
                          }
                        >
                          {position.title}
                        </span>
                        <span className="font-mono text-[11px] text-rail">
                          {positionDates(position)}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </>
          )}

          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setCreating(true)}
              className="border border-ink px-3 py-1.5 font-body text-xs font-medium text-ink"
            >
              None of these — create new
            </button>
            {onCancel && (
              <button
                type="button"
                onClick={onCancel}
                className="ml-auto px-3 py-1.5 font-body text-xs text-rail"
              >
                Cancel
              </button>
            )}
          </div>
        </>
      )}
    </div>
  );
}

interface CreatePositionFormProps {
  /** Pre-selected employer, if the user picked one before choosing to create. */
  employer: Employer | undefined;
  suggestedEmployerName: string;
  suggestedPositionTitle: string;
  onCreated: (positionID: string) => void;
  onBack: () => void;
  busy: boolean;
}

function CreatePositionForm({
  employer,
  suggestedEmployerName,
  suggestedPositionTitle,
  onCreated,
  onBack,
  busy,
}: CreatePositionFormProps) {
  const createEmployer = useCreateEmployer();
  const createPosition = useCreatePosition();

  const [useExisting, setUseExisting] = useState(!!employer);
  const [employerName, setEmployerName] = useState(suggestedEmployerName);
  const [title, setTitle] = useState(suggestedPositionTitle);
  const [startedOn, setStartedOn] = useState("");
  const [endedOn, setEndedOn] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);

  const pending = createEmployer.isPending || createPosition.isPending || busy;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setLocalError(null);

    if (!title.trim()) {
      setLocalError("A position title is required.");
      return;
    }
    // The backend parses started_on as YYYY-MM-DD and 400s on anything else,
    // including an empty string. Catching it here keeps the message specific.
    if (!startedOn) {
      setLocalError("A start date is required.");
      return;
    }
    if (!useExisting && !employerName.trim()) {
      setLocalError("An employer name is required.");
      return;
    }

    try {
      const employerID = useExisting
        ? employer!.id
        : (await createEmployer.mutateAsync({ name: employerName.trim() })).id;

      const position = await createPosition.mutateAsync({
        employer_id: employerID,
        title: title.trim(),
        started_on: startedOn,
        ended_on: endedOn || null,
      });
      onCreated(position.id);
    } catch (err) {
      setLocalError(formatApiError(err));
    }
  }

  return (
    <form onSubmit={submit} className="border border-border bg-surface p-4">
      <p className="mb-3 font-mono text-[10px] tracking-widest text-rail uppercase">
        New position
      </p>

      {employer && (
        <label className="mb-2 flex items-center gap-2 font-body text-sm text-ink">
          <input
            type="checkbox"
            checked={useExisting}
            onChange={(e) => setUseExisting(e.target.checked)}
          />
          Use existing employer “{employer.name}”
        </label>
      )}

      {!useExisting && (
        <>
          <label
            className="font-body text-xs text-ink-dim"
            htmlFor="new-employer"
          >
            Employer name
          </label>
          <input
            id="new-employer"
            value={employerName}
            onChange={(e) => setEmployerName(e.target.value)}
            className="mt-1 mb-2 w-full border border-border bg-card px-2 py-1.5 font-body text-sm text-ink"
          />
        </>
      )}

      <label className="font-body text-xs text-ink-dim" htmlFor="new-title">
        Position title
      </label>
      <input
        id="new-title"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        className="mt-1 mb-2 w-full border border-border bg-card px-2 py-1.5 font-body text-sm text-ink"
      />

      <div className="mb-3 flex gap-3">
        <div className="flex-1">
          <label className="font-body text-xs text-ink-dim" htmlFor="new-start">
            Started on
          </label>
          <input
            id="new-start"
            type="date"
            value={startedOn}
            onChange={(e) => setStartedOn(e.target.value)}
            className="mt-1 w-full border border-border bg-card px-2 py-1.5 font-body text-sm text-ink"
          />
        </div>
        <div className="flex-1">
          <label className="font-body text-xs text-ink-dim" htmlFor="new-end">
            Ended on <span className="text-rail">(blank if current)</span>
          </label>
          <input
            id="new-end"
            type="date"
            value={endedOn}
            onChange={(e) => setEndedOn(e.target.value)}
            className="mt-1 w-full border border-border bg-card px-2 py-1.5 font-body text-sm text-ink"
          />
        </div>
      </div>

      {localError && (
        <p className="mb-2 font-body text-sm text-reject">{localError}</p>
      )}

      <div className="flex gap-2">
        <button
          type="submit"
          disabled={pending}
          className="bg-stamp px-3 py-1.5 font-body text-xs font-medium text-surface disabled:opacity-50"
        >
          {pending ? "Creating…" : "Create and attach"}
        </button>
        <button
          type="button"
          onClick={onBack}
          className="px-3 py-1.5 font-body text-xs text-rail"
        >
          Back
        </button>
      </div>
    </form>
  );
}
