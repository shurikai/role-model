import { Link } from "react-router-dom";
import { useApplications, useFitReports, useResumeVersions } from "../hooks/useApplications";
import { formatApiError } from "../lib/api-client";
import type { Application } from "../lib/types";

function PipelineStatusBadge({ application }: { application: Application }) {
  const { data: fitReports } = useFitReports(application.id);
  const { data: versions } = useResumeVersions(application.id);

  if (!application.jd_signals) {
    return (
      <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
        Draft
      </span>
    );
  }

  if (versions && versions.length > 0) {
    return (
      <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700">
        Generated
      </span>
    );
  }

  const latestFitReport = fitReports?.[0];

  if (!latestFitReport) {
    return (
      <span className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700">
        Signals extracted
      </span>
    );
  }

  // No pass/fail here anymore — the anti-pattern check is advisory, so a
  // report existing is the whole status. Its scores are what matter, and
  // they live on the detail page.
  return (
    <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">
      Fit evaluated
    </span>
  );
}

export function Applications() {
  const { data: applications, isLoading, error } = useApplications();

  return (
    <div className="mx-auto mt-12 max-w-3xl px-4">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-gray-900">Applications</h1>
        <Link
          to="/applications/new"
          className="rounded bg-gray-900 px-3 py-2 text-sm font-medium text-white"
        >
          New Application
        </Link>
      </div>

      {isLoading && <p className="text-sm text-gray-600">Loading...</p>}
      {error && <p className="text-sm text-red-600">{formatApiError(error)}</p>}

      {applications && applications.length === 0 && (
        <p className="text-sm text-gray-600">
          No applications yet. Paste a job description to get started.
        </p>
      )}

      {applications && applications.length > 0 && (
        <ul className="divide-y divide-gray-200 rounded border border-gray-200">
          {applications.map((application) => (
            <li key={application.id}>
              <Link
                to={`/applications/${application.id}`}
                className="flex items-center justify-between px-4 py-3 hover:bg-gray-50"
              >
                <div>
                  <p className="text-sm font-medium text-gray-900">
                    {application.role_title} — {application.company_name}
                  </p>
                  <p className="text-xs text-gray-500">{application.status}</p>
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
