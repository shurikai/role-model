import { Link } from "react-router-dom";
import {
  useApplications,
  useFitReports,
  useResumeVersions,
} from "../hooks/useApplications";
import { formatApiError } from "../lib/api-client";
import type { Application } from "../lib/types";

/**
 * How far an application has travelled through the pipeline.
 *
 * The four states are stages, not verdicts, so they read on one scale rather
 * than as good/bad: rail for untouched, verify for signals read, stamp for
 * evaluated, ink for the end of the line. There is deliberately no pass/fail
 * colour — the fit gate is advisory and nothing is blocked on it, so a red
 * badge here would assert a judgment the backend does not make.
 */
function PipelineStatusBadge({ application }: { application: Application }) {
  const { data: fitReports } = useFitReports(application.id);
  const { data: versions } = useResumeVersions(application.id);

  // Order preserved from the original: no signals reads as Draft whatever
  // else exists, because everything downstream is derived from them.
  let label = "Draft";
  let tone = "bg-rail";

  if (application.jd_signals) {
    if (versions && versions.length > 0) {
      label = "Generated";
      tone = "bg-ink";
    } else if (fitReports?.[0]) {
      label = "Fit evaluated";
      tone = "bg-stamp";
    } else {
      label = "Signals extracted";
      tone = "bg-verify";
    }
  }

  return (
    <span
      className={`flex-shrink-0 px-2 py-1 font-mono text-[10px] tracking-wider text-surface uppercase ${tone}`}
    >
      {label}
    </span>
  );
}

export function Applications() {
  const { data: applications, isLoading, error } = useApplications();

  return (
    <div className="mx-auto max-w-4xl px-6 py-10">
      <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
        Applications
      </p>
      <div className="mb-6 flex flex-wrap items-baseline justify-between gap-3">
        <h1 className="font-display text-2xl font-bold text-ink">
          Everywhere you have applied
        </h1>
        <Link
          to="/applications/new"
          className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface"
        >
          New application
        </Link>
      </div>

      {isLoading && <p className="font-body text-sm text-ink-dim">Loading…</p>}
      {error && (
        <p className="border border-reject bg-card p-4 font-body text-sm text-ink">
          {formatApiError(error)}
        </p>
      )}

      {applications && applications.length === 0 && (
        <div className="border border-border bg-card p-4">
          <p className="font-body text-sm text-ink-dim">
            No applications yet. Paste a job description to get started.
          </p>
          {/*
            An account with no applications is usually an account with no
            career in it either, and a job description is not much use without
            one. There is no "does this user have career data" signal to gate
            on, so this offers the import rather than asserting it is needed.
          */}
          <p className="mt-2 font-body text-sm text-ink-dim">
            If you have not brought your career in yet,{" "}
            <Link to="/import/career/new" className="text-ink underline">
              start there
            </Link>
            .
          </p>
        </div>
      )}

      {applications && applications.length > 0 && (
        <ul>
          {applications.map((application) => (
            <li key={application.id} className="mb-2">
              <Link
                to={`/applications/${application.id}`}
                className="flex items-start justify-between gap-3 border-y border-r border-l-[6px] border-border border-l-rail bg-card px-4 py-2.5"
              >
                <div>
                  <p className="font-mono text-[10px] tracking-widest text-rail uppercase">
                    {application.company_name}
                  </p>
                  <p className="font-display text-[15px] font-bold text-ink">
                    {application.role_title}
                  </p>
                  <p className="font-body text-xs text-ink-dim">
                    {application.status}
                  </p>
                </div>
                <PipelineStatusBadge application={application} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
