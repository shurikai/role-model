import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import {
  useApplication,
  useExtractSignals,
  useFitReports,
  useRunFitEvaluation,
  useResumeVersions,
  useGenerateResume,
  useRenderResumeVersion,
} from "../hooks/useApplications";
import { formatApiError } from "../lib/api-client";
import { preferenceLabel } from "../lib/types";
import type {
  PreferenceListEntry,
  ResumeVersion,
  ScreeningSummary,
  SkillMatch,
} from "../lib/types";

/**
 * Requirements the profile answers but not as deeply as the posting asked.
 *
 * Rendered separately from gaps on purpose: the skill is present, so filing it
 * under gaps would say the opposite of what the scorer found. Each row names
 * the bar and what met it, since "Kafka" alone doesn't tell the reader what is
 * short about it. Plain text, no severity treatment — a partial is not a
 * lesser gap, it is a different finding.
 */
function PartialMatchList({ entries }: { entries: SkillMatch[] | null }) {
  if (!entries || entries.length === 0) return null;
  return (
    <div>
      <p className="font-mono text-[10px] tracking-widest text-rail uppercase">
        Below the level asked for
      </p>
      <ul className="ml-4 list-disc font-body text-sm text-ink-dim">
        {entries.map((m) => (
          <li key={m.requirement}>
            {m.requirement}
            {m.required_level && m.evidence_level
              ? ` — posting asks for ${m.required_level}, yours is ${m.evidence_level}`
              : null}
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * One of the preference lists a fit report carries, rendered plainly.
 *
 * A stopgap, deliberately: preference fit stopped being a score and became
 * four lists, and this is what makes them visible until the fit report gets a
 * real design pass. It reuses the technical-gaps shape — bold label, bulleted
 * list — and adds nothing else. `tone` picks between the two treatments the
 * page already had rather than introducing a severity scale.
 */
function PreferenceList({
  label,
  entries,
  tone = "neutral",
}: {
  label: string;
  entries: PreferenceListEntry[] | null;
  tone?: "neutral" | "conflict";
}) {
  if (!entries || entries.length === 0) return null;
  const conflict = tone === "conflict";
  return (
    <div>
      <p
        className={`font-mono text-[10px] tracking-widest uppercase ${
          conflict ? "text-reject" : "text-rail"
        }`}
      >
        {label}
      </p>
      <ul
        className={`ml-4 list-disc font-body text-sm ${
          conflict ? "text-reject" : "text-ink-dim"
        }`}
      >
        {entries.map((entry) => (
          <li key={preferenceLabel(entry)}>{preferenceLabel(entry)}</li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Screening facts, presented plainly. This answers "should I even consider
 * this role", which is a different question from the skills-match signals
 * below it — hence the separate callout.
 *
 * Deliberately free of sentiment styling: no red/green, no severity ordering
 * on other_flags. The backend extracts these descriptively and leaves the
 * judgment to the reader; colouring them here would put the judgment back.
 */
function ScreeningSummaryPanel({ summary }: { summary: ScreeningSummary }) {
  const fields: Array<[string, string]> = [
    ["Location", summary.location],
    ["Work arrangement", summary.work_arrangement],
    ["Travel", summary.travel],
    ["Industry", summary.industry],
    ["Clearance / citizenship", summary.clearance_citizenship],
  ];

  return (
    <div className="mb-3 border border-border bg-surface p-4">
      <p className="mb-2 font-mono text-[10px] tracking-widest text-rail uppercase">
        Screening summary
      </p>
      <dl className="font-body text-sm">
        {fields.map(([label, value]) => (
          <div key={label} className="flex gap-2">
            <dt className="text-ink-dim">{label}:</dt>
            <dd className="text-ink">{value || "—"}</dd>
          </div>
        ))}
      </dl>
      {summary.other_flags.length > 0 && (
        <div className="mt-2">
          <p className="font-body text-sm text-ink-dim">Other flags:</p>
          <ul className="ml-4 list-disc font-body text-sm text-ink">
            {summary.other_flags.map((flag) => (
              <li key={flag}>{flag}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function ResumeVersionRow({
  version,
  filenameBase,
}: {
  version: ResumeVersion;
  filenameBase: string;
}) {
  const renderVersion = useRenderResumeVersion();
  const [downloaded, setDownloaded] = useState(false);

  async function handleDownload() {
    setDownloaded(false);
    const blob = await renderVersion.mutateAsync(version.id);
    downloadBlob(blob, `${filenameBase}-v${version.version_number}.docx`);
    setDownloaded(true);
  }

  return (
    <li className="mb-2 border-y border-r border-l-[6px] border-border border-l-rail bg-card px-4 py-2.5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-rail uppercase">
            Version {version.version_number}
            {version.submitted && " · submitted"}
          </p>
          <p className="font-body text-xs text-ink-dim">
            {new Date(version.created_at).toLocaleString()}
          </p>
          {version.generation_notes && (
            <p className="mt-1 font-body text-xs text-ink-dim">
              {version.generation_notes}
            </p>
          )}
        </div>
        <div className="flex flex-shrink-0 items-center gap-2">
          {downloaded && (
            <span className="font-body text-xs text-stamp">Downloaded ✓</span>
          )}
          <button
            type="button"
            onClick={handleDownload}
            disabled={renderVersion.isPending}
            className="bg-ink px-4 py-1.5 font-display text-sm font-bold text-surface disabled:opacity-50"
          >
            {renderVersion.isPending ? "Rendering…" : "Download .docx"}
          </button>
        </div>
      </div>
      {renderVersion.isError && (
        <p className="mt-2 font-body text-sm text-reject">
          {formatApiError(renderVersion.error)}
        </p>
      )}
    </li>
  );
}

export function ApplicationDetail() {
  const { id } = useParams<{ id: string }>();
  const { data: application, isLoading, error } = useApplication(id);
  const extractSignals = useExtractSignals();
  const { data: fitReports } = useFitReports(id);
  const runFitEvaluation = useRunFitEvaluation();
  const { data: versions } = useResumeVersions(id);
  const generateResume = useGenerateResume();

  if (isLoading) {
    return (
      <p className="mx-auto max-w-3xl px-6 py-10 font-body text-sm text-ink-dim">
        Loading…
      </p>
    );
  }
  if (error || !application) {
    return (
      <p className="mx-auto max-w-3xl px-6 py-10 font-body text-sm text-reject">
        {error ? formatApiError(error) : "Application not found."}
      </p>
    );
  }

  const latestFitReport = fitReports?.[0];
  // A fit report existing is the only precondition. The anti-pattern check is
  // advisory — it no longer blocks anything on the backend, so it must not
  // block anything here either.
  const canGenerate = !!latestFitReport;
  const filenameBase = `${application.company_name}-${application.role_title}`
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-");

  return (
    <div className="mx-auto max-w-4xl px-6 py-10">
      <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
        {/* The breadcrumb is the back affordance. The shell's nav reaches the
            same place, but a detail page should not make you go looking. */}
        <Link to="/applications" className="underline">
          Applications
        </Link>{" "}
        · {application.company_name}
      </p>
      <h1 className="mb-1 font-display text-2xl font-bold text-ink">
        {application.role_title}
      </h1>
      <p className="mb-6 font-body text-[13px] text-ink-dim">
        Status: {application.status}
      </p>

      <section className="mb-6">
        <h2 className="mb-2 font-display text-lg font-bold text-ink">
          Job description signals
        </h2>
        {!application.jd_signals ? (
          <div className="border border-border bg-card p-4">
            <p className="mb-3 font-body text-sm text-ink-dim">
              Signals have not been extracted yet.
            </p>
            <button
              type="button"
              onClick={() => extractSignals.mutate(application.id)}
              disabled={extractSignals.isPending || !application.jd_text}
              className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50"
            >
              {extractSignals.isPending ? "Extracting…" : "Extract signals"}
            </button>
            {extractSignals.isError && (
              <p className="mt-2 font-body text-sm text-reject">
                {formatApiError(extractSignals.error)}
              </p>
            )}
          </div>
        ) : (
          <div className="border border-border bg-card p-4 font-body text-sm text-ink">
            {application.jd_signals.screening_summary && (
              <ScreeningSummaryPanel
                summary={application.jd_signals.screening_summary}
              />
            )}
            <p>
              <span className="font-medium">Seniority:</span>{" "}
              {application.jd_signals.seniority}
            </p>
            <p>
              <span className="font-medium">Industry:</span>{" "}
              {application.jd_signals.industry || "—"}
            </p>
            <p>
              <span className="font-medium">Work arrangement:</span>{" "}
              {application.jd_signals.work_arrangement || "—"}
            </p>
            <div>
              <span className="font-medium">Required skills:</span>{" "}
              {application.jd_signals.required_skills.join(", ") || "—"}
            </div>
            <div>
              <span className="font-medium">Preferred skills:</span>{" "}
              {application.jd_signals.preferred_skills.join(", ") || "—"}
            </div>
            <div>
              <span className="font-medium">Core competencies:</span>{" "}
              {application.jd_signals.core_competencies?.join(", ") || "—"}
            </div>
            <div>
              <span className="font-medium">Culture signals:</span>{" "}
              {application.jd_signals.culture_signals.join(", ") || "—"}
            </div>
            <button
              type="button"
              onClick={() => extractSignals.mutate(application.id)}
              disabled={extractSignals.isPending}
              className="mt-2 font-body text-xs text-ink-dim underline disabled:opacity-50"
            >
              {extractSignals.isPending
                ? "Re-extracting…"
                : "Re-extract signals"}
            </button>
            {extractSignals.isError && (
              <p className="font-body text-sm text-reject">
                {formatApiError(extractSignals.error)}
              </p>
            )}
          </div>
        )}
      </section>

      <section className="mb-6">
        <h2 className="mb-2 font-display text-lg font-bold text-ink">
          Fit report
        </h2>
        <div className="border border-border bg-card p-4">
          <button
            type="button"
            onClick={() => runFitEvaluation.mutate(application.id)}
            disabled={runFitEvaluation.isPending || !application.jd_signals}
            className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50"
          >
            {runFitEvaluation.isPending
              ? "Running…"
              : latestFitReport
                ? "Re-run fit gate"
                : "Run fit gate"}
          </button>
          {runFitEvaluation.isError && (
            <p className="mt-2 font-body text-sm text-reject">
              {formatApiError(runFitEvaluation.error)}
            </p>
          )}

          {latestFitReport && (
            <div className="mt-4 space-y-3 font-body text-sm text-ink">
              {/*
                Advisory only, and shown only when there's something to say.
                A standing "passed" badge would be noise now that nothing is
                gated on it, and "failed" would overstate a keyword match.
              */}
              {latestFitReport.dealbreaker_hits &&
                latestFitReport.dealbreaker_hits.length > 0 && (
                  <div className="border border-flag bg-flag-highlight p-3">
                    <p className="font-mono text-[10px] tracking-widest text-flag uppercase">
                      Dealbreaker flag
                    </p>
                    <p className="font-body text-xs text-ink-dim">
                      Matched against your hard-exclude preferences.
                      Informational — it does not block generation.
                    </p>
                    <ul className="mt-1 list-inside list-disc font-body text-sm text-ink">
                      {latestFitReport.dealbreaker_hits.map((hit) => (
                        <li key={hit.id}>{hit.label}</li>
                      ))}
                    </ul>
                  </div>
                )}
              <p>
                <span className="font-mono text-[10px] tracking-widest text-rail uppercase">
                  Capability score
                </span>{" "}
                {/* Null means nothing was assessed, which is not zero and not
                    100 — the em dash has to stay distinguishable from a real
                    score of any value. */}
                {latestFitReport.capability_score === null
                  ? "—"
                  : `${latestFitReport.capability_score}/100`}
              </p>
              <PartialMatchList entries={latestFitReport.capability_partial} />
              {latestFitReport.capability_gaps &&
                latestFitReport.capability_gaps.length > 0 && (
                  <div>
                    <p className="font-mono text-[10px] tracking-widest text-rail uppercase">
                      Capability gaps
                    </p>
                    <ul className="ml-4 list-disc font-body text-sm text-ink-dim">
                      {latestFitReport.capability_gaps.map((gap) => (
                        <li key={gap}>{gap}</li>
                      ))}
                    </ul>
                  </div>
                )}
              {/*
                Preference fit is a hit list, not a score. The fourth list —
                the hard-gate hits — is the anti-pattern callout above; it is
                not repeated here, since a disqualifier reading twice as two
                findings is exactly the confusion the separate lists prevent.
              */}
              <PreferenceList
                label="Preferences matched"
                entries={latestFitReport.preference_matches}
              />
              <PreferenceList
                label="Preference conflicts"
                entries={latestFitReport.preference_conflicts}
                tone="conflict"
              />
              <PreferenceList
                label="Preferences not mentioned"
                entries={latestFitReport.preference_gaps}
              />
              {latestFitReport.narrative && (
                <p className="border-t border-dashed border-rail pt-3 font-body text-sm text-ink italic">
                  {latestFitReport.narrative}
                </p>
              )}
            </div>
          )}
        </div>
      </section>

      <section className="mb-6">
        <h2 className="mb-2 font-display text-lg font-bold text-ink">
          Resume versions
        </h2>
        <div className="border border-border bg-card p-4">
          <button
            type="button"
            onClick={() => generateResume.mutate(application.id)}
            disabled={generateResume.isPending || !canGenerate}
            className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50"
            title={!canGenerate ? "Run the fit evaluation first" : undefined}
          >
            {generateResume.isPending ? "Writing…" : "Generate resume"}
          </button>
          {!canGenerate && (
            <p className="mt-2 font-body text-xs text-ink-dim">
              Run the fit evaluation first to see scores before generating.
            </p>
          )}
          {generateResume.isError && (
            <p className="mt-2 font-body text-sm text-reject">
              {formatApiError(generateResume.error)}
            </p>
          )}

          {versions && versions.length > 0 && (
            <ul className="mt-4">
              {versions.map((version) => (
                <ResumeVersionRow
                  key={version.id}
                  version={version}
                  filenameBase={filenameBase}
                />
              ))}
            </ul>
          )}
        </div>
      </section>
    </div>
  );
}
