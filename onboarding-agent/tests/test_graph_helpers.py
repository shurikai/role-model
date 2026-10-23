"""Pure-function tests for agent/graph.py's small parsing/routing helpers --
kept separate from test_graph.py's end-to-end graph behavior, the same split
test_tools.py already draws for agent/tools.py.
"""

from __future__ import annotations

import pytest

from agent.graph import (
    _ends_before,
    _is_bare_date,
    _parse_date,
    _parse_depth_answer,
    _sounds_done,
)


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
        # #163: a stop phrase as its own first clause, then more words. These
        # read as "not done" and were recorded as contributions until the eval
        # hit its turn cap.
        ("Done. We have covered everything.", True),
        ("done, that is everything", True),
        ("No. That's everything from my notes.", True),
        ("That's it, thanks!", True),
        ("Nope -- nothing else", True),
        ("Done \u2014 there's nothing left to add.", True),
        # ...but a clause that carries on with "but" is a contribution.
        ("No, but I also rebuilt the night-shift schedule", False),
        ("Done. But one more: I trained the new hires.", False),
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


def test_parse_depth_answer_does_not_treat_a_calendar_year_as_a_duration():
    """Found by cmd/onboardingeval's first real run: "I got my ACLS in 2012"
    parsed "2012" as 2012.0 years of experience -- unbounded, since nothing
    checked plausibility before this fix -- which skills.years_experience
    (NUMERIC(4,1)) can't even store, so POST /skills 500'd on a numeric field
    overflow and took the whole interview down. A calendar year mentioned
    alongside a real duration must not stop the real duration from being
    found either."""
    levels = [{"value": "proficient"}, {"value": "expert"}]
    proficiency, years = _parse_depth_answer("expert, I got certified in 2012", levels)
    assert proficiency == "expert"
    assert years is None

    proficiency, years = _parse_depth_answer(
        "proficient, since 2012 about 5 years", levels
    )
    assert proficiency == "proficient"
    assert years == 5.0


@pytest.mark.parametrize(
    ("answer", "expected"),
    [
        ("2019-03-15", "2019-03-15"),
        ("2019-03", "2019-03-01"),
        ("2019", "2019-01-01"),
        ("  2019-03  ", "2019-03-01"),
        # #159: a date followed by the end of a sentence.
        ("2019-03.", "2019-03-01"),
        ("2019-03!", "2019-03-01"),
        ("2019.", "2019-01-01"),
        ("March 2019", "2019-03-01"),
        ("mar. 2019", "2019-03-01"),
        ("September, 2021", "2021-09-01"),
        # #158: a date inside a longer answer, including one that goes on to
        # describe another job -- the first date is the one asked about.
        ("I started there in 2015-06, I think", "2015-06-01"),
        ("I was at Acme from 2015, then Globex 2019 to 2022", "2015-01-01"),
        ("from March 2016 until 2018", "2016-03-01"),
        # Numbers that are not years stay numbers.
        ("a 1200-bed hospital", None),
        ("we had 4000 patients", None),
        # An impossible month is skipped, not clamped.
        ("2019-13", None),
        ("sometime last spring", None),
        ("", None),
    ],
)
def test_parse_date(answer, expected):
    assert _parse_date(answer) == expected


@pytest.mark.parametrize(
    ("answer", "expected"),
    [
        ("2019-03", True),
        ("2019-03.", True),
        ("March 2019", True),
        ("I started in 2019-03", False),
    ],
)
def test_is_bare_date(answer, expected):
    assert _is_bare_date(answer) == expected


@pytest.mark.parametrize(
    ("ended_on", "answer", "started_on", "expected"),
    [
        # Compared at the precision given: "2019" ends a job that started in
        # March 2019 rather than reading as January and being refused.
        ("2019-01-01", "2019", "2019-03-15", False),
        ("2019-03-01", "2019-03", "2019-03-15", False),
        ("2018-12-01", "2018-12", "2019-03-15", True),
        ("2019-03-01", "2019-03-01", "2019-03-15", True),
        # A start that already fell back to the placeholder can't refuse
        # anything.
        ("2010-01-01", "2010", "1900-01-01", False),
    ],
)
def test_ends_before(ended_on, answer, started_on, expected):
    assert _ends_before(ended_on, answer, started_on) == expected
