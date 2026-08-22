import json
from pathlib import Path

import pytest

from models import Resume

# The renderer's fixtures are the repo's shared pipeline fixtures, not a
# private copy -- the same documents the Go side generates against.
FIXTURE_DIR = Path(__file__).resolve().parents[2] / "tests" / "fixtures"

# sample_resume.json has no projects and no credentials; sample_formatted.json
# carries two of each. Between them the optional-section branches are covered in
# both directions, so tests that care about structure parametrize over both.
#
# The credentials on sample_formatted.json were added when _render_credentials
# was written: neither fixture had carried one, so the section could be modelled
# end to end and never rendered without a single test going red. A fixture pair
# whose job is covering both directions has to actually cover both.
# minimal_resume_data below is the all-empty case, and test_docx_builder feeds
# it to the builder -- for a long time nothing did.
FIXTURE_NAMES = ["sample_resume.json", "sample_formatted.json"]


def load_fixture(name: str) -> dict:
    with open(FIXTURE_DIR / name) as f:
        return json.load(f)


@pytest.fixture(params=FIXTURE_NAMES)
def resume_data(request: pytest.FixtureRequest) -> dict:
    """Each tracked fixture, as raw JSON."""
    return load_fixture(request.param)


@pytest.fixture(params=FIXTURE_NAMES)
def resume(request: pytest.FixtureRequest) -> Resume:
    """Each tracked fixture, validated."""
    return Resume.model_validate(load_fixture(request.param))


@pytest.fixture
def minimal_resume_data() -> dict:
    """The smallest document that validates.

    Written out rather than derived from a fixture so that a test which
    mutates one field is unambiguous about what it is removing.
    """
    return {
        "schema_version": "2.0",
        "generated_at": "2026-08-17T00:00:00Z",
        "application_id": "11111111-1111-1111-1111-111111111111",
        "resume_version_id": "22222222-2222-2222-2222-222222222222",
        "summary": "A summary.",
        "sections": [
            {"key": "summary", "heading": "SUMMARY"},
            {"key": "skills", "heading": "SKILLS"},
            {"key": "experience", "heading": "EXPERIENCE"},
            {"key": "projects", "heading": "PROJECTS"},
            {"key": "education", "heading": "EDUCATION"},
            {"key": "credentials", "heading": "CREDENTIALS"},
        ],
        "identity": {"name": "Test Person", "email": "test@example.com"},
        "experience": [
            {
                "employer": "Acme",
                "positions": [
                    {
                        "position_id": "33333333-3333-3333-3333-333333333333",
                        "title": "Engineer",
                        "started_on": "2020-01",
                        "bullets": [
                            {
                                "text": "Did the thing.",
                                "contribution_ids": [
                                    "44444444-4444-4444-4444-444444444444"
                                ],
                            }
                        ],
                    }
                ],
            }
        ],
        "skills": {"Languages": ["Go"]},
        "education": [],
        "credentials": [],
        "projects": [],
        "meta": {"generation_model": "claude-sonnet-5", "prompt_version": "2a/2b"},
    }
