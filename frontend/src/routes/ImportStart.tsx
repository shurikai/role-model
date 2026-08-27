import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useCreateImportBatch } from "../hooks/useImport";
import { formatApiError } from "../lib/api-client";

/**
 * The on-ramp: paste career text, get a batch, go review it.
 *
 * Deliberately thin. Extraction runs synchronously inside `POST /import`, so
 * this submit can sit for a while — the button says so rather than pretending
 * the work is instant.
 */
export function ImportStart() {
  const [rawText, setRawText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const createBatch = useCreateImportBatch();
  const navigate = useNavigate();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!rawText.trim()) {
      setError("Paste something to import first.");
      return;
    }
    try {
      const batch = await createBatch.mutateAsync(rawText);
      navigate(`/import/${batch.id}`);
    } catch (err) {
      setError(formatApiError(err));
    }
  }

  return (
    <div>
      <div className="mx-auto max-w-3xl px-6 py-10">
        <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
          Stage 0 · Import
        </p>
        <h1 className="mb-1 font-display text-2xl font-bold text-ink">
          Import career history
        </h1>
        <p className="mb-6 font-body text-[13px] text-ink-dim">
          Paste a résumé, a brag document, or raw notes. Everything it produces
          is a draft you review before anything is written to your record.
        </p>

        <form onSubmit={submit}>
          <label
            htmlFor="raw-text"
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Source text
          </label>
          <textarea
            id="raw-text"
            value={rawText}
            onChange={(e) => setRawText(e.target.value)}
            rows={16}
            className="w-full border border-border bg-surface p-3 font-body text-sm text-ink"
          />

          {error && (
            <p className="mt-2 font-body text-sm text-reject">{error}</p>
          )}

          <div className="mt-4 flex items-center gap-3">
            <button
              type="submit"
              disabled={createBatch.isPending}
              className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50"
            >
              {createBatch.isPending ? "Extracting…" : "Extract entries"}
            </button>
            <Link
              to="/applications"
              className="font-body text-sm text-ink-dim underline"
            >
              Cancel
            </Link>
          </div>
        </form>
      </div>
    </div>
  );
}
