import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  useCreateApplication,
  useExtractSignals,
} from "../hooks/useApplications";
import { formatApiError } from "../lib/api-client";

export function ApplicationNew() {
  const navigate = useNavigate();
  const createApplication = useCreateApplication();
  const extractSignals = useExtractSignals();
  const [companyName, setCompanyName] = useState("");
  const [roleTitle, setRoleTitle] = useState("");
  const [jdUrl, setJdUrl] = useState("");
  const [jdText, setJdText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const application = await createApplication.mutateAsync({
        company_name: companyName,
        role_title: roleTitle,
        jd_url: jdUrl || null,
        jd_text: jdText,
      });
      try {
        await extractSignals.mutateAsync(application.id);
      } catch {
        // Signal extraction can be retried from the detail page —
        // the application itself was created successfully.
      }
      navigate(`/applications/${application.id}`);
    } catch (err) {
      setError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto mt-12 max-w-2xl px-4">
      <h1 className="mb-6 text-2xl font-semibold text-gray-900">
        New Application
      </h1>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label
            htmlFor="companyName"
            className="block text-sm font-medium text-gray-700"
          >
            Company
          </label>
          <input
            id="companyName"
            type="text"
            required
            value={companyName}
            onChange={(e) => setCompanyName(e.target.value)}
            className="mt-1 block w-full rounded border border-gray-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label
            htmlFor="roleTitle"
            className="block text-sm font-medium text-gray-700"
          >
            Role title
          </label>
          <input
            id="roleTitle"
            type="text"
            required
            value={roleTitle}
            onChange={(e) => setRoleTitle(e.target.value)}
            className="mt-1 block w-full rounded border border-gray-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label
            htmlFor="jdUrl"
            className="block text-sm font-medium text-gray-700"
          >
            Job posting URL <span className="text-gray-400">(optional)</span>
          </label>
          <input
            id="jdUrl"
            type="url"
            value={jdUrl}
            onChange={(e) => setJdUrl(e.target.value)}
            className="mt-1 block w-full rounded border border-gray-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label
            htmlFor="jdText"
            className="block text-sm font-medium text-gray-700"
          >
            Job description text
          </label>
          <textarea
            id="jdText"
            required
            rows={12}
            value={jdText}
            onChange={(e) => setJdText(e.target.value)}
            className="mt-1 block w-full rounded border border-gray-300 px-3 py-2 text-sm"
          />
        </div>

        {error && <p className="text-sm text-red-600">{error}</p>}

        <button
          type="submit"
          disabled={submitting}
          className="w-full rounded bg-gray-900 px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
        >
          {submitting ? "Creating..." : "Create & Extract Signals"}
        </button>
      </form>
    </div>
  );
}
