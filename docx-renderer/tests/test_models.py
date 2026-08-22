"""The schema contract.

models.py mirrors schema/resume.v1.json. Nothing enforces that the two agree,
so these tests pin the invariants the Go pipeline relies on. If the JSON schema
changes and models.py does not, this is where it should surface -- a renderer
that silently accepts a document the schema forbids is how a broken .docx gets
produced instead of a 422.
"""

import pytest
from pydantic import ValidationError

from models import Bullet, PositionBlock, ProjectEntry, Resume


def test_tracked_fixtures_validate(resume_data: dict) -> None:
    Resume.model_validate(resume_data)


def test_minimal_document_validates(minimal_resume_data: dict) -> None:
    Resume.model_validate(minimal_resume_data)


@pytest.mark.parametrize(
    "field",
    ["schema_version", "summary", "identity", "experience", "skills", "meta"],
)
def test_missing_required_field_is_rejected(
    minimal_resume_data: dict, field: str
) -> None:
    del minimal_resume_data[field]
    with pytest.raises(ValidationError):
        Resume.model_validate(minimal_resume_data)


def test_bullet_requires_at_least_one_contribution_id() -> None:
    """Provenance is the point of the whole system.

    Every bullet traces back to the contributions it was generated from. A
    bullet with an empty contribution_ids array is an untraceable claim, and
    the schema's min_length=1 is what keeps it out.
    """
    with pytest.raises(ValidationError):
        Bullet(text="Unsourced claim.", contribution_ids=[])


def test_bullet_accepts_a_contribution_id() -> None:
    bullet = Bullet(text="Sourced claim.", contribution_ids=["abc"])
    assert bullet.contribution_ids == ["abc"]
    assert bullet.feedback_signal is None


@pytest.mark.parametrize("bad_date", ["2020", "2020-1", "2020-01-15", "January 2020"])
def test_position_dates_must_be_year_month(bad_date: str) -> None:
    """Dates are YYYY-MM, not full dates.

    The renderer formats tenure ranges from these directly, so a full date or a
    free-text month would render literally.
    """
    with pytest.raises(ValidationError):
        PositionBlock(
            position_id="p1",
            title="Engineer",
            started_on=bad_date,
            bullets=[Bullet(text="x", contribution_ids=["c1"])],
        )


def test_position_ended_on_may_be_omitted_for_current_role() -> None:
    position = PositionBlock(
        position_id="p1",
        title="Engineer",
        started_on="2020-01",
        bullets=[Bullet(text="x", contribution_ids=["c1"])],
    )
    assert position.ended_on is None


def test_industry_level_accepts_any_ladder_rung() -> None:
    """industry_level is free text, and that is deliberate.

    It used to be a Literal of ten software rungs, which was one industry's
    ladder applied to every other: a chef's "sous" and a clinician's
    "attending" were rejected by the renderer for not being "staff". The
    ladder is user-owned vocabulary (career_levels) on the Go side now, so the
    document contract cannot enumerate it. Nothing in the renderer branches on
    the value -- it is carried for provenance alongside title.
    """
    for rung in ("senior", "sous", "attending", "journeyman"):
        position = PositionBlock(
            position_id="p1",
            title="Engineer",
            industry_level=rung,
            started_on="2020-01",
            bullets=[Bullet(text="x", contribution_ids=["c1"])],
        )
        assert position.industry_level == rung


def test_project_role_and_status_accept_any_vocabulary() -> None:
    """role and status are free text in schema v2, and that is deliberate.

    v1 enumerated open-source repository vocabulary --
    author/maintainer/contributor/lead and active/dormant/archived -- which a
    portfolio of buildings, cases, recipes, or performances has none of.
    Nothing in the renderer branches on either value.
    """
    for role, status in (
        ("author", "active"),
        ("principal investigator", "published"),
        ("head chef", "seasonal"),
    ):
        project = ProjectEntry(
            project_id="proj1",
            name="Thing",
            role=role,
            status=status,
            bullets=[Bullet(text="x", contribution_ids=["c1"])],
        )
        assert project.role == role
        assert project.status == status


def test_employer_must_have_at_least_one_position(minimal_resume_data: dict) -> None:
    minimal_resume_data["experience"][0]["positions"] = []
    with pytest.raises(ValidationError):
        Resume.model_validate(minimal_resume_data)


def test_optional_sections_may_be_empty(minimal_resume_data: dict) -> None:
    """Empty is not the same as missing.

    A resume with no projects or credentials is ordinary; the fixture with no
    projects is a real generated document.
    """
    resume = Resume.model_validate(minimal_resume_data)
    assert resume.projects == []
    assert resume.credentials == []
    assert resume.education == []


def test_industry_role_is_free_text_not_an_enum() -> None:
    """Real normalized roles are compound: the seed data carries "Senior
    Software Engineer / Architect" and "Senior Software Engineer / Team Lead".
    industry_level is the enum; industry_role is prose."""
    position = PositionBlock(
        position_id="p1",
        title="Programmer VII",
        industry_role="Senior Software Engineer / Architect",
        started_on="2020-01",
        bullets=[Bullet(text="x", contribution_ids=["c1"])],
    )
    assert position.display_title == "Senior Software Engineer / Architect"


def test_display_title_falls_back_to_the_verbatim_title() -> None:
    """Documents generated before industry_role existed carry only title."""
    position = PositionBlock(
        position_id="p1",
        title="Programmer VII",
        started_on="2020-01",
        bullets=[Bullet(text="x", contribution_ids=["c1"])],
    )
    assert position.industry_role is None
    assert position.display_title == "Programmer VII"
