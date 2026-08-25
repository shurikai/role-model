import { useState, useRef, useLayoutEffect } from "react";
import { Check, Pencil, X, SkipForward, ChevronDown, ChevronUp } from "lucide-react";

// ---------------------------------------------------------------------------
// Stage 0c — Contribution Draft Review
// Palette: Kanagawa Lotus (light), pulled from rebelot/kanagawa.nvim source.
// paper #e5ddb0, card #f2ecbc, field #dcd5ac, ink #545464, ink-dim #716e61,
// verify-blue #4d699b, flag-orange #cc6d00, stamp-green #6e915f, reject-red
// #c84053, rail-gray #8a8980, border #d5cea3, highlight #f9d791.
// Type: Space Grotesk (display), Inter (body), IBM Plex Mono (metadata).
// ---------------------------------------------------------------------------

const FONT_IMPORT = `
@import url('https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@500;700&family=Inter:wght@400;500&family=IBM+Plex+Mono:wght@400;500&display=swap');
`;

const COLORS = {
  paper: "#d5cea3",
  card: "#e5ddb0",
  surface: "#f2ecbc",
  ink: "#545464",
  inkDim: "#716e61",
  verify: "#4d699b",
  flag: "#cc6d00",
  flagHighlight: "#f9d791",
  stamp: "#6e915f",
  reject: "#c84053",
  rail: "#8a8980",
  border: "#dcd5ac",
};

const STATUS_COLOR = {
  pending: COLORS.rail,
  approved: COLORS.stamp,
  edited: COLORS.verify,
  rejected: COLORS.reject,
  skipped: COLORS.rail,
};

const STATUS_LABEL = {
  approved: "Approved",
  edited: "Edited",
  rejected: "Rejected",
  skipped: "Skipped",
};

const MOCK_DRAFTS = [
  {
    id: "0114",
    employer: "Daugherty Business Solutions",
    title: "Senior Backend Engineer",
    raw_text:
      "Worked on decomposing a monolith into microservices for a financial services client, improved deployment speed.",
    suggested_text:
      "Led decomposition of a monolithic order-management system into ~40% independently deployable microservices for a financial services client, reducing deployment time from hours to minutes.",
    confidence: "medium",
    flagged_inferences: [
      { phrase: "~40% independently deployable", note: "Specific percentage not in original — verify or soften to \"a portion of\"." },
      { phrase: "hours to minutes", note: "Deployment time comparison inferred — confirm actual before/after figures." },
    ],
    missing_fields: ["team size", "timeframe"],
    review_status: "pending",
  },
  {
    id: "0115",
    employer: "Daugherty Business Solutions",
    title: "Senior Backend Engineer",
    raw_text: "Used Splunk and Dynatrace for observability on the Edward Jones engagement.",
    suggested_text:
      "Implemented observability tooling (Splunk, Dynatrace) across the Edward Jones engagement to support production monitoring and incident triage.",
    confidence: "high",
    flagged_inferences: [],
    missing_fields: ["scale (# services monitored)"],
    review_status: "pending",
  },
  {
    id: "0116",
    employer: "Disney Cruise Line",
    title: "Backend Engineer",
    raw_text: "Worked on Navigator app backend, microservices.",
    suggested_text:
      "Built and maintained backend microservices for the Navigator guest-facing application, supporting real-time updates for onboard guests.",
    confidence: "low",
    flagged_inferences: [
      { phrase: "real-time updates", note: "\"Real-time\" not stated in original — confirm this is accurate framing, not assumed." },
    ],
    missing_fields: ["scope", "team", "specific services owned"],
    review_status: "approved",
  },
  {
    id: "0117",
    employer: "MAK Technologies",
    title: "Software Engineer",
    raw_text: "Built C++ plugins integrating Kafka with a real-time simulation platform.",
    suggested_text:
      "Developed C++ plugins to integrate Kafka messaging with a real-time distributed simulation platform, enabling telemetry exchange across simulation nodes.",
    confidence: "high",
    flagged_inferences: [],
    missing_fields: [],
    review_status: "rejected",
  },
];

function FlaggedText({ text, flags }) {
  if (!flags.length) return <span>{text}</span>;

  let remaining = text;
  const segments = [];
  flags.forEach((f, i) => {
    const idx = remaining.indexOf(f.phrase);
    if (idx === -1) return;
    segments.push({ type: "text", content: remaining.slice(0, idx) });
    segments.push({ type: "flag", content: f.phrase, note: f.note, key: i });
    remaining = remaining.slice(idx + f.phrase.length);
  });
  segments.push({ type: "text", content: remaining });

  return (
    <div>
      <div>
        {segments.map((s, i) =>
          s.type === "flag" ? (
            <mark
              key={i}
              style={{ backgroundColor: COLORS.flagHighlight, color: "#6b4a05", fontWeight: 500 }}
              className="px-0.5"
            >
              {s.content}
            </mark>
          ) : (
            <span key={i}>{s.content}</span>
          )
        )}
      </div>
      {flags.map((f, i) => (
        <div key={i} className="mt-2 flex items-start gap-2 pl-2" style={{ borderLeft: `2px dashed ${COLORS.flag}` }}>
          <span
            className="text-[10px] uppercase tracking-wider mt-0.5"
            style={{ fontFamily: "'IBM Plex Mono', monospace", color: COLORS.flag }}
          >
            flag
          </span>
          <span className="text-xs" style={{ color: "#6b5230", fontFamily: "'Inter', sans-serif" }}>
            {f.note}
          </span>
        </div>
      ))}
    </div>
  );
}

function ConfidenceTag({ level }) {
  const map = {
    high: { label: "HIGH", bg: COLORS.stamp },
    medium: { label: "MED", bg: COLORS.flag },
    low: { label: "LOW", bg: COLORS.rail },
  };
  const { label, bg } = map[level];
  return (
    <span
      className="text-[10px] px-2 py-0.5 tracking-wider"
      style={{ fontFamily: "'IBM Plex Mono', monospace", backgroundColor: bg, color: COLORS.surface }}
    >
      {label}
    </span>
  );
}

const STATUS_ICON = {
  approved: Check,
  edited: Pencil,
  rejected: X,
  skipped: SkipForward,
};

function StatusLabel({ status }) {
  if (status === "pending") return null;
  const Icon = STATUS_ICON[status];
  return (
    <span
      className="flex items-center gap-1 text-[10px] uppercase tracking-wider px-2 py-1"
      style={{
        fontFamily: "'IBM Plex Mono', monospace",
        fontWeight: 500,
        color: COLORS.surface,
        backgroundColor: STATUS_COLOR[status],
      }}
    >
      <Icon size={11} strokeWidth={2.5} />
      {STATUS_LABEL[status]}
    </span>
  );
}

function DraftCard({ draft, onAction }) {
  const [expanded, setExpanded] = useState(true);
  const decided = draft.review_status !== "pending";
  const dimmed = draft.review_status === "rejected" || draft.review_status === "skipped";

  return (
    <div
      className="relative mb-5"
      style={{
        backgroundColor: COLORS.card,
        borderTop: `1px solid ${COLORS.border}`,
        borderRight: `1px solid ${COLORS.border}`,
        borderBottom: `1px solid ${COLORS.border}`,
        borderLeft: `6px solid ${decided ? STATUS_COLOR[draft.review_status] : COLORS.border}`,
        opacity: dimmed ? 0.6 : 1,
      }}
    >
      {/* Header */}
      <div
        className="px-5 py-3"
        style={{ borderBottom: `1px solid ${COLORS.border}` }}
      >
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-3">
            <div className="flex flex-col items-start gap-1.5 flex-shrink-0" style={{ minWidth: "34px" }}>
              <span style={{ fontFamily: "'IBM Plex Mono', monospace", fontSize: "11px", color: COLORS.rail }}>
                #{draft.id}
              </span>
              <ConfidenceTag level={draft.confidence} />
            </div>
            <div className="flex flex-col gap-0.5 items-start">
              <span style={{ fontFamily: "'Space Grotesk', sans-serif", fontWeight: 700, fontSize: "15px", color: COLORS.ink }}>
                {draft.employer}
              </span>
              <span style={{ fontFamily: "'Inter', sans-serif", fontSize: "13px", color: COLORS.inkDim }}>
                {draft.title}
              </span>
            </div>
          </div>
          <button onClick={() => setExpanded((e) => !e)} style={{ color: COLORS.rail }}>
            {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </button>
        </div>
        {draft.review_status !== "pending" && (
          <div className="mt-2 pl-[46px]">
            <StatusLabel status={draft.review_status} />
          </div>
        )}
      </div>

      {expanded && (
        <div className="px-5 py-4">
          <div className="grid grid-cols-2 gap-6 mb-4">
            <div>
              <div
                className="text-[10px] uppercase tracking-widest mb-2"
                style={{ fontFamily: "'IBM Plex Mono', monospace", color: COLORS.rail }}
              >
                Original
              </div>
              <div
                className="text-sm p-3"
                style={{
                  fontFamily: "'Inter', sans-serif",
                  color: COLORS.inkDim,
                  backgroundColor: COLORS.surface,
                  border: `1px dashed ${COLORS.border}`,
                  minHeight: "88px",
                }}
              >
                {draft.raw_text}
              </div>
            </div>
            <div>
              <div
                className="text-[10px] uppercase tracking-widest mb-2"
                style={{ fontFamily: "'IBM Plex Mono', monospace", color: COLORS.verify }}
              >
                Suggested
              </div>
              <div
                className="text-sm p-3"
                style={{
                  fontFamily: "'Inter', sans-serif",
                  color: COLORS.ink,
                  border: `1px solid ${COLORS.verify}`,
                  backgroundColor: COLORS.surface,
                  minHeight: "88px",
                }}
              >
                <FlaggedText text={draft.suggested_text} flags={draft.flagged_inferences} />
              </div>
            </div>
          </div>

          {draft.missing_fields.length > 0 && (
            <div className="flex items-center gap-2 mb-4 flex-wrap">
              <span
                className="text-[10px] uppercase tracking-widest"
                style={{ fontFamily: "'IBM Plex Mono', monospace", color: COLORS.rail }}
              >
                Would strengthen:
              </span>
              {draft.missing_fields.map((f, i) => (
                <span
                  key={i}
                  className="text-xs px-2 py-0.5"
                  style={{ fontFamily: "'Inter', sans-serif", color: COLORS.inkDim, border: `1px dashed ${COLORS.rail}` }}
                >
                  {f}
                </span>
              ))}
            </div>
          )}

          <div className="flex gap-2 pt-3" style={{ borderTop: `1px solid ${COLORS.border}` }}>
            <button
              onClick={() => onAction(draft.id, "approved")}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{ fontFamily: "'Inter', sans-serif", fontWeight: 500, backgroundColor: COLORS.stamp, color: COLORS.surface }}
            >
              <Check size={13} /> Approve
            </button>
            <button
              onClick={() => onAction(draft.id, "edited")}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{ fontFamily: "'Inter', sans-serif", fontWeight: 500, border: `1px solid ${COLORS.ink}`, color: COLORS.ink }}
            >
              <Pencil size={13} /> Edit
            </button>
            <button
              onClick={() => onAction(draft.id, "rejected")}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{ fontFamily: "'Inter', sans-serif", fontWeight: 500, border: `1px solid ${COLORS.border}`, color: COLORS.reject }}
            >
              <X size={13} /> Reject
            </button>
            <button
              onClick={() => onAction(draft.id, "skipped")}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5 ml-auto"
              style={{ fontFamily: "'Inter', sans-serif", color: COLORS.rail }}
            >
              <SkipForward size={13} /> Skip for now
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export default function Stage0cReview() {
  const [drafts, setDrafts] = useState(MOCK_DRAFTS);
  const handleAction = (id, status) =>
    setDrafts((prev) => prev.map((d) => (d.id === id ? { ...d, review_status: status } : d)));

  const reviewedCount = drafts.filter((d) => d.review_status !== "pending").length;

  return (
    <div style={{ backgroundColor: COLORS.paper, minHeight: "100vh" }}>
      <style>{FONT_IMPORT}</style>
      <div className="flex max-w-5xl mx-auto">
        {/* Ledger rail — tick color reflects what happened to each item */}
        <div className="hidden md:flex flex-col items-center pt-[152px] pr-4 flex-shrink-0" style={{ width: "32px" }}>
          {drafts.map((d) => (
            <div key={d.id} className="flex flex-col items-center" style={{ marginBottom: "1px" }}>
              <div
                style={{
                  width: "10px",
                  height: "10px",
                  backgroundColor: d.review_status === "pending" ? "transparent" : STATUS_COLOR[d.review_status],
                  border: `1.5px solid ${STATUS_COLOR[d.review_status]}`,
                }}
              />
              <div style={{ width: "1px", height: "132px", backgroundColor: COLORS.border }} />
            </div>
          ))}
        </div>

        {/* Main column */}
        <div className="flex-1 py-10 px-6">
          <div className="mb-8">
            <div
              className="text-[11px] uppercase tracking-widest mb-2"
              style={{ fontFamily: "'IBM Plex Mono', monospace", color: COLORS.verify }}
            >
              Stage 0c · Import Review
            </div>
            <div className="flex items-baseline justify-between mb-1">
              <h1 style={{ fontFamily: "'Space Grotesk', sans-serif", fontWeight: 700, fontSize: "26px", color: COLORS.ink }}>
                Review imported entries
              </h1>
              <span style={{ fontFamily: "'IBM Plex Mono', monospace", fontSize: "13px", color: COLORS.inkDim }}>
                {reviewedCount} / {drafts.length} reviewed
              </span>
            </div>
            <p style={{ fontFamily: "'Inter', sans-serif", fontSize: "13px", color: COLORS.inkDim }}>
              Imported from resume_2026.pdf — parsed {drafts.length} entries, 2 minutes ago
            </p>
          </div>

          {drafts.map((d) => (
            <DraftCard key={d.id} draft={d} onAction={handleAction} />
          ))}

          <div className="flex justify-end mt-6 pt-5" style={{ borderTop: `1px dashed ${COLORS.rail}` }}>
            <button
              className="text-sm px-5 py-2.5"
              style={{ fontFamily: "'Space Grotesk', sans-serif", fontWeight: 700, backgroundColor: COLORS.ink, color: COLORS.surface }}
            >
              Write approved entries to career record
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
