import { ProfileSkills } from "../components/ProfileSkills";
import { ProfilePreferences } from "../components/ProfilePreferences";

/**
 * The two things the fit gate scores a posting against.
 *
 * One page rather than two routes, because they answer halves of one
 * question — what you can do, and what you want — and the gate reports them
 * on separate axes on purpose: a role you could do and would hate should read
 * as high capability and poor preference, not as one muddled number. Seeing
 * both in one place is what makes that legible.
 *
 * Both had a full API and nothing in front of it, so a preference could be
 * created exactly once, by an import, and never corrected.
 */
export function Profile() {
  return (
    <div className="mx-auto max-w-4xl px-6 py-10">
      <p className="mb-2 font-mono text-[11px] tracking-widest text-verify uppercase">
        Profile
      </p>
      <h1 className="mb-1 font-display text-2xl font-bold text-ink">
        What you can do, and what you want
      </h1>
      <p className="mb-8 font-body text-[13px] text-ink-dim">
        Every fit report is built from these two lists. Skills answer whether a
        posting's requirements are covered; preferences answer whether the job
        is one you would take. Neither is inferred from your career history —
        they are claims you make, and only you can make them.
      </p>

      <ProfileSkills />
      <ProfilePreferences />
    </div>
  );
}
