import { Page } from "../components/Page";
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
    <Page
      eyebrow="Skills & preferences"
      title="What you can do, and what you want"
      lede="Every fit report is built from these two lists. Skills answer whether a posting's requirements are covered; preferences answer whether the job is one you would take. Neither is inferred from your career history — they are claims you make, and only you can make them."
    >
      <ProfileSkills />
      <ProfilePreferences />
    </Page>
  );
}
