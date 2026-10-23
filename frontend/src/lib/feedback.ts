/**
 * Where "Feedback" in the shell points, or null when the deployment has not
 * configured one (the link is then not shown at all, rather than pointing at
 * nothing).
 *
 * Set at build time from VITE_FEEDBACK_URL (FEEDBACK_URL in the compose
 * file). Either a form URL, used as-is, or a mailto: address, which gets a
 * subject and a body naming the page the report came from -- testers report
 * from where they hit a problem, and "which screen?" is the first thing a
 * report otherwise lacks (#167).
 */
export function feedbackHref(
  configured: string | undefined,
  pathname: string,
): string | null {
  const url = configured?.trim();
  if (!url) return null;
  if (!url.toLowerCase().startsWith("mailto:")) return url;

  const [address, existingQuery] = url.split("?", 2);
  const params = new URLSearchParams(existingQuery ?? "");
  if (!params.has("subject")) params.set("subject", "Role Model feedback");
  if (!params.has("body")) {
    params.set("body", `Page: ${pathname}\n\nWhat happened:\n\n`);
  }
  // URLSearchParams encodes spaces as "+", which mail clients show literally.
  return `${address}?${params.toString().replace(/\+/g, "%20")}`;
}
