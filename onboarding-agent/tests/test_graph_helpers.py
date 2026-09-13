"""Pure-function tests for agent/graph.py's small parsing/routing helpers --
kept separate from test_graph.py's end-to-end graph behavior, the same split
test_tools.py already draws for agent/tools.py.
"""

from __future__ import annotations

import pytest

from agent.graph import _parse_depth_answer, _parse_started_on, _sounds_done


@pytest.mark.parametrize(
    ("answer", "expected"),
    [
        ("done", True),
        ("Done.", True),
        ("no", True),
        ("No!", True),
        ("nope", True),
        ("nothing", True),
        ("that's it", True),
        ("thats it", True),
        ("no more", True),
        ("next job please", True),
        # The regression this test file exists for: the target word appearing
        # mid-answer used to truncate a real contribution description.
        (
            "I made sure the migration was done before the cutover, then improved caching",
            False,
        ),
        ("Wrote a script that does nothing but log errors", False),
        ("Rebuilt the onboarding flow end to end", False),
    ],
)
def test_sounds_done(answer, expected):
    assert _sounds_done(answer) is expected


@pytest.mark.parametrize(
    ("answer", "expected_proficiency", "expected_years"),
    [
        ("expert, 5 years", "expert", 5.0),
        ("proficient, 3 yrs", "proficient", 3.0),
        ("novice, 1 year", "novice", 1.0),
        ("expert, 2yr", "expert", 2.0),
        ("expert, about 4 years I think", "expert", 4.0),
        ("not sure", None, None),
    ],
)
def test_parse_depth_answer(answer, expected_proficiency, expected_years):
    levels = [
        {"value": "novice"},
        {"value": "proficient"},
        {"value": "expert"},
    ]
    proficiency, years = _parse_depth_answer(answer, levels)
    assert proficiency == expected_proficiency
    assert years == expected_years


def test_parse_depth_answer_does_not_misparse_a_non_year_token():
    """The case that actually distinguishes the two approaches. rstrip("years")
    strips any trailing RUN of the characters y/e/a/r/s, not the substring
    "years" -- so a nonsense token like "5ass" (all of a/s/s happen to be in
    that character set) used to parse as years=5.0 even though it names no
    unit at all. removesuffix only strips the literal word, so it correctly
    leaves this token unparsed and the loop moves on to nothing -- no years
    recorded, which is the right answer for an answer that names none."""
    levels = [{"value": "expert"}]
    proficiency, years = _parse_depth_answer("expert, 5ass", levels)
    assert proficiency == "expert"
    assert years is None


@pytest.mark.parametrize(
    ("answer", "expected"),
    [
        ("2019-03-15", "2019-03-15"),
        ("2019-03", "2019-03-01"),
        ("2019", "2019-01-01"),
        ("  2019-03  ", "2019-03-01"),
        ("sometime last spring", None),
        ("March 2019", None),
        ("", None),
    ],
)
def test_parse_started_on(answer, expected):
    assert _parse_started_on(answer) == expected
