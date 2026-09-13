import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  useSendOnboardingTurn,
  useStartOnboarding,
} from "../hooks/useOnboarding";
import { formatApiError } from "../lib/api-client";

interface Turn {
  role: "agent" | "user";
  text: string;
}

interface SavedSession {
  sessionId: string;
  transcript: Turn[];
  done: boolean;
}

// Namespaced like session.ts's own key. sessionStorage, not localStorage: an
// abandoned interview shouldn't resurrect itself in a new tab days later, but
// a refresh in the SAME tab is exactly the case this exists for — before
// this, a refresh mid-interview discarded sessionId and the transcript from
// component state with nothing recording where to resume, orphaning the
// interview (which the onboarding agent had already checkpointed
// server-side) and silently starting a brand new one over it.
const STORAGE_KEY = "role_model_onboarding";

function loadSavedSession(): SavedSession | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as SavedSession) : null;
  } catch {
    // Private browsing, quota, storage disabled -- losing resume-on-refresh
    // is an acceptable degradation, not worth surfacing as an error.
    return null;
  }
}

function saveSession(session: SavedSession): void {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  } catch {
    // Same as above.
  }
}

function clearSavedSession(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Same as above.
  }
}

/**
 * The conversational on-ramp (#117), parallel to Stage 0's paste-a-document
 * path (ImportStart.tsx) for someone with nothing to paste.
 *
 * Unlike ImportReview's batch-status polling, there is no background job to
 * wait on here — a turn is a direct request/response, just a slower one.
 */
export function Onboarding() {
  // Read once, synchronously, at the first render of a real mount -- not in
  // an effect -- so the very first render already knows whether to resume or
  // start fresh, rather than flashing a fresh "What company did you work at?"
  // for one frame before a restore kicks in.
  const [restored] = useState(() => loadSavedSession());

  const [transcript, setTranscript] = useState<Turn[]>(
    () => restored?.transcript ?? [],
  );
  const [sessionId, setSessionId] = useState<string | null>(
    () => restored?.sessionId ?? null,
  );
  const [done, setDone] = useState(() => restored?.done ?? false);
  const [input, setInput] = useState("");
  const [error, setError] = useState<string | null>(null);

  // One real interview per real mount of this screen — see useStartOnboarding
  // for why this has to be a query rather than a mutation-in-an-effect. Only
  // ever used as a local React Query cache key, never sent anywhere, so it
  // doesn't need crypto.randomUUID()'s uniqueness guarantee — which is just
  // as well, since that API only exists in a secure context (HTTPS or
  // localhost) and this app is routinely opened over plain HTTP on a LAN IP.
  const [instanceKey] = useState(() => `${Date.now()}-${Math.random()}`);
  const start = useStartOnboarding(instanceKey, restored === null);
  const sendTurn = useSendOnboardingTurn();

  // Copies the query's result into the transcript exactly once it arrives.
  // Idempotent by construction (setting state to the same values twice is a
  // no-op re-render), which is what a StrictMode double-invoke needs here,
  // rather than a ref guard around an imperative call. Never runs at all when
  // a session was restored, since the query above is disabled in that case.
  useEffect(() => {
    if (!start.data || sessionId) return;
    setSessionId(start.data.session_id);
    setDone(start.data.done);
    setTranscript([{ role: "agent", text: start.data.reply }]);
  }, [start.data, sessionId]);

  // Persists on every change, and stops persisting a finished interview —
  // otherwise revisiting the screen after finishing would keep restoring the
  // same completed conversation forever instead of starting a new one.
  useEffect(() => {
    if (!sessionId) return;
    if (done) {
      clearSavedSession();
      return;
    }
    saveSession({ sessionId, transcript, done });
  }, [sessionId, transcript, done]);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!input.trim() || !sessionId || done) return;
    setError(null);

    const message = input.trim();
    setTranscript((prev) => [...prev, { role: "user", text: message }]);
    setInput("");

    sendTurn.mutate(
      { session_id: sessionId, message },
      {
        onSuccess: (turn) => {
          setDone(turn.done);
          setTranscript((prev) => [
            ...prev,
            { role: "agent", text: turn.reply },
          ]);
        },
        onError: (err) => setError(formatApiError(err)),
      },
    );
  }

  return (
    <div>
      <div className="mx-auto max-w-3xl px-6 py-10">
        <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
          Career interview
        </p>
        <h1 className="mb-1 font-display text-2xl font-bold text-ink">
          Tell me about your work
        </h1>
        <p className="mb-6 font-body text-[13px] text-ink-dim">
          Answer one question at a time. Nothing is written to your record until
          you confirm it here in the conversation.
        </p>

        <div className="mb-4 flex flex-col gap-3">
          {transcript.map((turn, i) => (
            <div
              key={i}
              className={`max-w-[85%] border p-3 font-body text-sm ${
                turn.role === "agent"
                  ? "self-start border-border bg-card text-ink"
                  : "self-end border-verify bg-surface text-ink"
              }`}
            >
              {turn.text}
            </div>
          ))}
        </div>

        {start.isPending && (
          <p className="mb-3 font-body text-sm text-ink-dim">
            Starting the interview…
          </p>
        )}
        {start.isError && (
          <p className="mb-3 font-body text-sm text-reject">
            {formatApiError(start.error)}
          </p>
        )}
        {error && <p className="mb-3 font-body text-sm text-reject">{error}</p>}

        {done ? (
          <p className="border border-border bg-card p-4 font-body text-sm text-ink-dim">
            That's everything for now.{" "}
            <Link to="/applications" className="text-ink underline">
              Back to applications
            </Link>
          </p>
        ) : (
          <form onSubmit={submit} className="flex gap-2">
            <label htmlFor="onboarding-answer" className="sr-only">
              Your answer
            </label>
            <input
              id="onboarding-answer"
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              disabled={sendTurn.isPending || !sessionId}
              className="flex-1 border border-border bg-surface p-3 font-body text-sm text-ink disabled:opacity-70"
              placeholder="Type your answer…"
            />
            <button
              type="submit"
              disabled={sendTurn.isPending || !sessionId || !input.trim()}
              className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface disabled:opacity-50"
            >
              {sendTurn.isPending ? "Sending…" : "Send"}
            </button>
          </form>
        )}

        <div className="mt-8 border-t border-dashed border-rail pt-5">
          <Link
            to="/applications"
            className="font-body text-sm text-ink-dim underline"
          >
            Back to applications
          </Link>
        </div>
      </div>
    </div>
  );
}
