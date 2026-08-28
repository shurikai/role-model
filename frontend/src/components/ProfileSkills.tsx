import { useState } from "react";
import {
  useSkills,
  useProficiencyLevels,
  useCreateSkill,
  useUpdateSkill,
  useDeleteSkill,
} from "../hooks/useProfile";
import { formatApiError } from "../lib/api-client";
import type { Skill } from "../lib/types";

/** NUMERIC arrives as a string over the wire; null means unrecorded. */
function formatYears(v: Skill["years_experience"]): string | null {
  if (v === null || v === undefined) return null;
  const n = typeof v === "string" ? Number(v) : v;
  if (Number.isNaN(n)) return null;
  return n % 1 === 0 ? `${n} yrs` : `${n} yrs`;
}

export function ProfileSkills() {
  const { data: skills, isLoading, error } = useSkills();
  const { data: levels } = useProficiencyLevels();
  const [adding, setAdding] = useState(false);

  const byCategory = new Map<string, Skill[]>();
  for (const s of skills ?? []) {
    byCategory.set(s.category, [...(byCategory.get(s.category) ?? []), s]);
  }
  const categories = [...byCategory.keys()].sort();

  return (
    <section className="mb-10">
      <div className="mb-1 flex flex-wrap items-baseline justify-between gap-3">
        <h2 className="font-display text-lg font-bold text-ink">Skills</h2>
        {!adding && (
          <button
            type="button"
            onClick={() => setAdding(true)}
            className="bg-ink px-4 py-1.5 font-display text-sm font-bold text-surface"
          >
            Add a skill
          </button>
        )}
      </div>
      <p className="mb-4 font-body text-[13px] text-ink-dim">
        What you claim you can do, and how deeply. These are what a posting's
        requirements are matched against, and the only source for the Skills
        section of a generated resume — a tag on a contribution is vocabulary,
        not a claim.
      </p>

      {isLoading && <p className="font-body text-sm text-ink-dim">Loading…</p>}
      {error && (
        <p className="border border-reject bg-card p-4 font-body text-sm text-ink">
          {formatApiError(error)}
        </p>
      )}

      {adding && (
        <SkillForm
          levels={levels ?? []}
          existingCategories={categories}
          onDone={() => setAdding(false)}
          onCancel={() => setAdding(false)}
        />
      )}

      {skills && skills.length === 0 && !adding && (
        <div className="border border-border bg-card p-4">
          <p className="font-body text-sm text-ink-dim">
            No skills claimed yet. A career import proposes these from what you
            described doing; you can also add them here.
          </p>
        </div>
      )}

      {categories.map((category) => (
        <div key={category} className="mb-5">
          <p className="mb-1 font-mono text-[10px] tracking-widest text-rail uppercase">
            {category}
          </p>
          {(byCategory.get(category) ?? []).map((s) => (
            <SkillRow key={s.id} skill={s} levels={levels ?? []} />
          ))}
        </div>
      ))}
    </section>
  );
}

function SkillRow({
  skill,
  levels,
}: {
  skill: Skill;
  levels: { value: string; label: string }[];
}) {
  const update = useUpdateSkill();
  const del = useDeleteSkill();
  const [confirming, setConfirming] = useState(false);
  const years = formatYears(skill.years_experience);

  function setProficiency(value: string) {
    update.mutate({
      id: skill.id,
      proficiency: value,
      years_experience:
        skill.years_experience === null ? null : Number(skill.years_experience),
      is_active: skill.is_active,
    });
  }

  function toggleActive() {
    update.mutate({
      id: skill.id,
      proficiency: skill.proficiency,
      years_experience:
        skill.years_experience === null ? null : Number(skill.years_experience),
      is_active: !skill.is_active,
    });
  }

  return (
    <div
      className={`mb-2 border-y border-r border-l-[6px] border-border bg-card px-4 py-2.5 ${
        skill.is_active ? "border-l-rail" : "border-l-border opacity-60"
      }`}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="font-display text-[15px] font-bold text-ink">
            {skill.name}
            {!skill.is_active && (
              <span className="ml-2 font-body text-xs font-normal text-ink-dim">
                inactive
              </span>
            )}
          </p>
          <p className="font-body text-xs text-ink-dim">
            {skill.proficiency}
            {years && ` · ${years}`}
          </p>
        </div>

        <div className="flex flex-shrink-0 flex-wrap items-center gap-2">
          {/*
            Depth is edited in place rather than behind an edit mode: it is the
            field that actually changes as time passes, and the fit gate scores
            a stated requirement against it.
          */}
          {levels.length > 0 && (
            <select
              aria-label={`Proficiency for ${skill.name}`}
              value={skill.proficiency}
              disabled={update.isPending}
              onChange={(e) => setProficiency(e.target.value)}
              className="border border-border bg-surface px-2 py-1 font-body text-xs text-ink disabled:opacity-50"
            >
              {levels.some((l) => l.value === skill.proficiency) ? null : (
                // A value not on the current scale still has to be selectable,
                // or opening this control would silently rewrite it.
                <option value={skill.proficiency}>{skill.proficiency}</option>
              )}
              {levels.map((l) => (
                <option key={l.value} value={l.value}>
                  {l.label}
                </option>
              ))}
            </select>
          )}
          <button
            type="button"
            onClick={toggleActive}
            disabled={update.isPending}
            className="border border-border bg-surface px-3 py-1 font-body text-xs text-ink-dim disabled:opacity-50"
          >
            {skill.is_active ? "Deactivate" : "Reactivate"}
          </button>
          {confirming ? (
            <>
              <button
                type="button"
                onClick={() => del.mutate(skill.id)}
                disabled={del.isPending}
                className="bg-reject px-3 py-1 font-display text-xs font-bold text-surface disabled:opacity-50"
              >
                {del.isPending ? "Deleting…" : "Really delete"}
              </button>
              <button
                type="button"
                onClick={() => setConfirming(false)}
                className="font-body text-xs text-ink-dim underline"
              >
                Keep
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={() => setConfirming(true)}
              className="border border-border bg-surface px-3 py-1 font-body text-xs text-ink-dim"
            >
              Delete
            </button>
          )}
        </div>
      </div>
      {confirming && (
        <p className="mt-2 font-body text-xs text-ink-dim">
          Deactivating is usually what you want — it keeps the row and its
          history, and only hides it from generation and matching.
        </p>
      )}
      {(update.isError || del.isError) && (
        <p className="mt-2 font-body text-xs text-reject">
          {formatApiError(update.error ?? del.error)}
        </p>
      )}
    </div>
  );
}

function SkillForm({
  levels,
  existingCategories,
  onDone,
  onCancel,
}: {
  levels: { value: string; label: string }[];
  existingCategories: string[];
  onDone: () => void;
  onCancel: () => void;
}) {
  const create = useCreateSkill();
  const [category, setCategory] = useState("");
  const [tag, setTag] = useState("");
  const [proficiency, setProficiency] = useState(levels[0]?.value ?? "");
  const [years, setYears] = useState("");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    create.mutate(
      {
        category: category.trim(),
        tag: tag.trim(),
        proficiency,
        years_experience: years.trim() === "" ? null : Number(years),
      },
      { onSuccess: onDone },
    );
  }

  return (
    <form
      onSubmit={submit}
      className="mb-3 border border-verify bg-card px-4 py-3"
    >
      <div className="mb-3 flex flex-wrap gap-3">
        <div className="min-w-52 flex-1">
          <label
            htmlFor="skill-tag"
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Skill
          </label>
          <input
            id="skill-tag"
            required
            value={tag}
            onChange={(e) => setTag(e.target.value)}
            placeholder="Epic"
            className="w-full border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
          />
        </div>
        <div className="min-w-52 flex-1">
          <label
            htmlFor="skill-category"
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Group it under
          </label>
          <input
            id="skill-category"
            required
            list="skill-categories"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            placeholder="Charting Systems"
            className="w-full border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
          />
          <datalist id="skill-categories">
            {existingCategories.map((c) => (
              <option key={c} value={c} />
            ))}
          </datalist>
        </div>
      </div>

      <div className="mb-3 flex flex-wrap gap-3">
        <div>
          <label
            htmlFor="skill-proficiency"
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            How deeply
          </label>
          {levels.length > 0 ? (
            <select
              id="skill-proficiency"
              value={proficiency}
              onChange={(e) => setProficiency(e.target.value)}
              className="border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
            >
              {levels.map((l) => (
                <option key={l.value} value={l.value}>
                  {l.label}
                </option>
              ))}
            </select>
          ) : (
            // An account with no scale accepts anything, which is how the
            // server behaves too. Guessing a scale here would be worse.
            <input
              id="skill-proficiency"
              required
              value={proficiency}
              onChange={(e) => setProficiency(e.target.value)}
              className="border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
            />
          )}
        </div>
        <div>
          <label
            htmlFor="skill-years"
            className="mb-1 block font-mono text-[10px] tracking-widest text-rail uppercase"
          >
            Years <span className="text-ink-dim">(optional)</span>
          </label>
          <input
            id="skill-years"
            type="number"
            min={0}
            step={0.5}
            value={years}
            onChange={(e) => setYears(e.target.value)}
            className="w-28 border border-border bg-surface px-3 py-2 font-body text-sm text-ink"
          />
        </div>
      </div>

      <p className="mb-3 font-body text-xs text-ink-dim">
        Leave years blank if you are not sure. Blank means unrecorded, which is
        read differently from a small number — nothing compares against it, and
        an unrecorded duration is not evidence of a short one.
      </p>

      {create.isError && (
        <p className="mb-2 font-body text-sm text-reject">
          {formatApiError(create.error)}
        </p>
      )}

      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={create.isPending}
          className="bg-ink px-5 py-2 font-display text-sm font-bold text-surface disabled:opacity-50"
        >
          {create.isPending ? "Saving…" : "Add skill"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="font-body text-sm text-ink-dim underline"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
