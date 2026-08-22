from typing import Literal

from pydantic import BaseModel, Field


class Bullet(BaseModel):
    text: str
    contribution_ids: list[str] = Field(min_length=1)
    feedback_signal: Literal["accepted", "rejected", "edited"] | None = None


class PositionBlock(BaseModel):
    position_id: str
    #: The verbatim title the employer used, e.g. "Programmer VII". Carried for
    #: provenance; see display_title for what actually reaches the page.
    title: str
    #: The industry-normalized name for the same job, e.g. "Senior Software
    #: Engineer". Free text rather than an enum, because real roles are
    #: compound: "Senior Software Engineer / Architect".
    industry_role: str | None = None
    #: The rung of the candidate's own career ladder this position sat on:
    #: "senior", "sous", "attending". Free text rather than a Literal, because
    #: the ladder is user-owned vocabulary (career_levels) and an enum here was
    #: one industry's rungs applied to every other. The renderer does not read
    #: it; it is carried for provenance alongside title.
    industry_level: str | None = None
    started_on: str = Field(pattern=r"^\d{4}-\d{2}$")
    ended_on: str | None = Field(pattern=r"^\d{4}-\d{2}$", default=None)
    bullets: list[Bullet]

    @property
    def display_title(self) -> str:
        """What the page shows for this role.

        The normalized role wins. A verbatim title carries the employer's
        internal grade ladder rather than the job -- "Programmer VII" tells a
        reader nothing about the work and reads as junior to anyone outside
        that company -- while the database has carried the normalization since
        the schema was written. It survived every stage of the pipeline and was
        discarded on the last line.

        title remains the fallback, so documents generated before
        industry_role existed still render.
        """
        return self.industry_role or self.title


class EmployerBlock(BaseModel):
    employer: str
    positions: list[PositionBlock] = Field(min_length=1)


class EducationEntry(BaseModel):
    institution: str
    degree: str | None = None
    field_of_study: str | None = None
    graduated: str | None = None


class CredentialEntry(BaseModel):
    name: str
    issuer: str | None = None
    issued_on: str | None = Field(pattern=r"^\d{4}-\d{2}$", default=None)
    expires_on: str | None = Field(pattern=r"^\d{4}-\d{2}$", default=None)


class ProjectEntry(BaseModel):
    project_id: str
    name: str
    tagline: str | None = None
    #: Free text in schema v2, both of these. v1 enumerated open-source
    #: repository vocabulary -- author/maintainer/contributor/lead and
    #: active/dormant/archived -- which a portfolio of buildings, cases,
    #: recipes, or performances has none of. Nothing branches on either; the
    #: renderer prints role and uses status for nothing at all.
    role: str
    status: str
    started_on: str | None = Field(pattern=r"^\d{4}-\d{2}$", default=None)
    ended_on: str | None = Field(pattern=r"^\d{4}-\d{2}$", default=None)
    #: Where the work itself lives: a repository, a catalogue entry, a case
    #: number, a recording. Named repo_url in schema v1.
    source_url: str | None = None
    live_url: str | None = None
    writeup_url: str | None = None
    force_include: bool = False
    force_exclude: bool = False
    tags: list[str] = Field(default_factory=list)
    bullets: list[Bullet]


class Identity(BaseModel):
    name: str
    email: str
    phone: str | None = None
    location: str | None = None
    linkedin_url: str | None = None
    github_url: str | None = None
    site_url: str | None = None
    headline: str | None = None


# jd_signals is deliberately absent from MetaBlock.
#
# There used to be a JDSignals model here describing priority_skills,
# seniority, and domain_vocabulary — the pre-split shape, long superseded by
# required_skills/preferred_skills/domain/work_type/culture_signals. Because
# Pydantic ignores unknown fields by default, it silently parsed every current
# document into an empty object and no one noticed for as long as it took the
# Go-side schema to catch the same drift from the other direction.
#
# Nothing in the renderer read it. Rather than resync a model with no consumer
# — which would simply rot again — the block is dropped. The document still
# carries meta.jd_signals for provenance; the renderer just does not model it,
# and Pydantic discards it as it discards any other unmodelled field.
class MetaBlock(BaseModel):
    generation_model: str
    prompt_version: str
    schema_version: str | None = None
    generated_at: str | None = None
    application_id: str | None = None
    target_role: str | None = None
    target_company: str | None = None


class Resume(BaseModel):
    schema_version: str
    generated_at: str
    application_id: str
    resume_version_id: str
    summary: str
    identity: Identity
    experience: list[EmployerBlock]
    skills: dict[str, list[str]]
    education: list[EducationEntry]
    credentials: list[CredentialEntry]
    projects: list[ProjectEntry]
    meta: MetaBlock
