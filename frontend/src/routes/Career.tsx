import { Link } from "react-router-dom";
import { Page } from "../components/Page";

/**
 * Placeholder. The career view itself — employers, positions, contributions —
 * is #118 and lands in the next PR. This exists now because `/` and the Career
 * tab both point here, and it says nothing about whether the record is empty:
 * telling an account with ten years of history that it has none would be worse
 * than saying nothing.
 */
export function Career() {
  return (
    <Page
      eyebrow="Career"
      title="Your career"
      actions={
        <Link
          to="/career/add"
          className="bg-ink px-5 py-2.5 font-display text-sm font-bold text-surface"
        >
          + Add to career
        </Link>
      }
      lede="Where you have worked, the roles you held, and what you did in each. Resumes are built from what is here."
    />
  );
}
