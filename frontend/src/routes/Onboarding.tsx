import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useSendOnboardingTurn } from "../hooks/useOnboarding";
import { formatApiError } from "../lib/api-client";

interface Turn {
  role: "agent" | "user";
  text: string;
}

/**
 * The conversational on-ramp (#117), parallel to Stage 0's paste-a-document
 * path (ImportStart.tsx) for someone with nothing to paste.
 *
 * Unlike ImportReview's batch-status polling, there is no background job to
 * wait on here — a turn is a direct request/response, just a slower one. The
 * session id and the visible transcript both live in local component state;
 * refreshing the page loses the transcript on screen but not the interview
 * itself, which the onboarding agent has already checkpointed server-side.
 */
export function Onboarding() {
  const [transcript, setTranscript] = useState<Turn[]>([]);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [input, setInput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const sendTurn = useSendOnboardingTurn();
  const started = useRef(false);

  // Starts itself: the first question is the agent's, not something the
  // person has to ask for by submitting anything.
  useEffect(() => {
    if (started.current) return;
    started.current = true;
    sendTurn.mutate(
      {},
      {
        onSuccess: (turn) => {
          setSessionId(turn.session_id);
          setDone(turn.done);
          setTranscript([{ role: "agent", text: turn.reply }]);
        },
        onError: (err) => setError(formatApiError(err)),
      },
    );
  }, []);

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
