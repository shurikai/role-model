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
import type { ResumeVersion, ScreeningSummary } from "../lib/types";

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
    <div className="mb-3 rounded border border-gray-300 bg-gray-50 p-4">
      <h3 className="mb-2 text-sm font-semibold text-gray-900">Screening summary</h3>
      <dl className="space-y-1 text-sm">
        {fields.map(([label, value]) => (
          <div key={label} className="flex gap-2">
            <dt className="font-medium text-gray-700">{label}:</dt>
            <dd className="text-gray-800">{value || "—"}</dd>
          </div>
        ))}
      </dl>
      {summary.other_flags.length > 0 && (
        <div className="mt-2 text-sm">
          <p className="font-medium text-gray-700">Other flags:</p>
          <ul className="ml-4 list-disc text-gray-800">
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
    <li className="rounded border border-gray-200 px-4 py-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-gray-900">
            Version {version.version_number}
            {version.submitted && (
              <span className="ml-2 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                submitted
              </span>
            )}
          </p>
          <p className="text-xs text-gray-500">{new Date(version.created_at).toLocaleString()}</p>
          {version.generation_notes && (
            <p className="mt-1 text-xs text-gray-600">{version.generation_notes}</p>
          )}
        </div>
        <div className="flex items-center gap-2">
          {downloaded && <span className="text-xs text-green-700">Downloaded ✓</span>}
          <button
            type="button"
            onClick={handleDownload}
            disabled={renderVersion.isPending}
            className="rounded bg-gray-900 px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
          >
            {renderVersion.isPending ? "Rendering..." : "Download .docx"}
          </button>
        </div>
      </div>
      {renderVersion.isError && (
        <p className="mt-2 text-sm text-red-600">{formatApiError(renderVersion.error)}</p>
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
    return <p className="mx-auto mt-12 max-w-3xl px-4 text-sm text-gray-600">Loading...</p>;
  }
  if (error || !application) {
    return (
      <p className="mx-auto mt-12 max-w-3xl px-4 text-sm text-red-600">
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
    <div className="mx-auto mt-12 max-w-3xl px-4 pb-16">
      <Link to="/applications" className="text-sm text-gray-600 underline">
        &larr; All applications
      </Link>

      <h1 className="mt-2 mb-1 text-2xl font-semibold text-gray-900">
        {application.role_title} — {application.company_name}
      </h1>
      <p className="mb-6 text-sm text-gray-500">Status: {application.status}</p>

      <section className="mb-8">
        <h2 className="mb-3 text-lg font-semibold text-gray-900">Job Description Signals</h2>
        {!application.jd_signals ? (
          <div className="rounded border border-gray-200 p-4">
            <p className="mb-3 text-sm text-gray-600">Signals have not been extracted yet.</p>
            <button
              type="button"
              onClick={() => extractSignals.mutate(application.id)}
              disabled={extractSignals.isPending || !application.jd_text}
              className="rounded bg-gray-900 px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
            >
              {extractSignals.isPending ? "Extracting..." : "Extract Signals"}
            </button>
            {extractSignals.isError && (
              <p className="mt-2 text-sm text-red-600">{formatApiError(extractSignals.error)}</p>
            )}
          </div>
        ) : (
          <div className="space-y-2 rounded border border-gray-200 p-4 text-sm">
            {application.jd_signals.screening_summary && (
              <ScreeningSummaryPanel summary={application.jd_signals.screening_summary} />
            )}
            <p>
              <span className="font-medium">Seniority:</span> {application.jd_signals.seniority}
            </p>
            <p>
              <span className="font-medium">Domain:</span> {application.jd_signals.domain}
            </p>
            <p>
              <span className="font-medium">Work type:</span> {application.jd_signals.work_type}
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
              <span className="font-medium">Culture signals:</span>{" "}
              {application.jd_signals.culture_signals.join(", ") || "—"}
            </div>
            <button
              type="button"
              onClick={() => extractSignals.mutate(application.id)}
              disabled={extractSignals.isPending}
              className="mt-2 text-xs text-gray-600 underline disabled:opacity-50"
            >
              {extractSignals.isPending ? "Re-extracting..." : "Re-extract signals"}
            </button>
            {extractSignals.isError && (
              <p className="text-sm text-red-600">{formatApiError(extractSignals.error)}</p>
            )}
          </div>
        )}
      </section>

      <section className="mb-8">
        <h2 className="mb-3 text-lg font-semibold text-gray-900">Fit Report</h2>
        <div className="rounded border border-gray-200 p-4">
          <button
            type="button"
            onClick={() => runFitEvaluation.mutate(application.id)}
            disabled={runFitEvaluation.isPending || !application.jd_signals}
            className="rounded bg-gray-900 px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
          >
            {runFitEvaluation.isPending
              ? "Running..."
              : latestFitReport
                ? "Re-run Fit Gate"
                : "Run Fit Gate"}
          </button>
          {runFitEvaluation.isError && (
            <p className="mt-2 text-sm text-red-600">{formatApiError(runFitEvaluation.error)}</p>
          )}

          {latestFitReport && (
            <div className="mt-4 space-y-3 text-sm">
              {/*
                Advisory only, and shown only when there's something to say.
                A standing "passed" badge would be noise now that nothing is
                gated on it, and "failed" would overstate a keyword match.
              */}
              {latestFitReport.anti_pattern_hits &&
                latestFitReport.anti_pattern_hits.length > 0 && (
                  <div className="rounded border border-amber-300 bg-amber-50 p-3">
                    <p className="font-medium text-amber-900">Anti-pattern flag</p>
                    <p className="text-xs text-amber-800">
                      Matched against your hard-exclude preferences. Informational — it
                      does not block generation.
                    </p>
                    <ul className="mt-1 list-inside list-disc text-amber-900">
                      {latestFitReport.anti_pattern_hits.map((hit) => (
                        <li key={hit.id}>{hit.label}</li>
                      ))}
                    </ul>
                  </div>
                )}
              <p>
                <span className="font-medium">Technical score:</span>{" "}
                {latestFitReport.technical_score === null
                  ? "—"
                  : `${latestFitReport.technical_score}/100`}
              </p>
              {latestFitReport.technical_gaps && latestFitReport.technical_gaps.length > 0 && (
                <div>
                  <span className="font-medium">Technical gaps:</span>
                  <ul className="ml-4 list-disc text-gray-700">
                    {latestFitReport.technical_gaps.map((gap) => (
                      <li key={gap}>{gap}</li>
                    ))}
                  </ul>
                </div>
              )}
              <p>
                <span className="font-medium">Preference score:</span>{" "}
                {latestFitReport.preference_score === null
                  ? "—"
                  : `${latestFitReport.preference_score}/100`}
              </p>
              {latestFitReport.preference_conflicts &&
                latestFitReport.preference_conflicts.length > 0 && (
                  <div>
                    <span className="font-medium text-red-800">Preference conflicts:</span>
                    <ul className="ml-4 list-disc text-red-700">
                      {latestFitReport.preference_conflicts.map((conflict) => (
                        <li key={conflict}>{conflict}</li>
                      ))}
                    </ul>
                  </div>
                )}
              {latestFitReport.preference_gaps &&
                latestFitReport.preference_gaps.length > 0 && (
                  <div>
                    <span className="font-medium">Preferences not mentioned:</span>
                    <ul className="ml-4 list-disc text-gray-700">
                      {latestFitReport.preference_gaps.map((gap) => (
                        <li key={gap}>{gap}</li>
                      ))}
                    </ul>
                  </div>
                )}
              {latestFitReport.narrative && (
                <p className="text-gray-700 italic">{latestFitReport.narrative}</p>
              )}
            </div>
          )}
        </div>
      </section>

      <section className="mb-8">
        <h2 className="mb-3 text-lg font-semibold text-gray-900">Resume Versions</h2>
        <div className="rounded border border-gray-200 p-4">
          <button
            type="button"
            onClick={() => generateResume.mutate(application.id)}
            disabled={generateResume.isPending || !canGenerate}
            className="rounded bg-gray-900 px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
            title={!canGenerate ? "Run the fit evaluation first" : undefined}
          >
            {generateResume.isPending ? "Generating..." : "Generate Resume"}
          </button>
          {!canGenerate && (
            <p className="mt-2 text-xs text-gray-500">
              Run the fit evaluation first to see scores before generating.
            </p>
          )}
          {generateResume.isError && (
            <p className="mt-2 text-sm text-red-600">{formatApiError(generateResume.error)}</p>
          )}

          {versions && versions.length > 0 && (
            <ul className="mt-4 space-y-2">
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
