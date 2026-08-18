"""Document construction.

These tests assert two kinds of thing:

1. Nothing is silently dropped. The failure mode that matters here is a
   position or a bullet disappearing from the output while the render still
   returns 200 -- exactly the shape of the early-career dropout regression
   (#32) on the Go side, which was invisible until someone read the document.

2. The layout invariants CLAUDE.md calls load-bearing hold: no Word heading
   styles, keep_with_next on the header chains, and bullets deliberately left
   free to break across a page.

They do not assert on .docx bytes. Binary comparison against the tracked
fixtures would fail on every incidental spacing change and teach us nothing.
"""

from docx.document import Document as DocumentObject

from models import Resume
from renderer.docx_builder import build_resume_document


def paragraph_texts(doc: DocumentObject) -> list[str]:
    return [p.text for p in doc.paragraphs]


def document_text(doc: DocumentObject) -> str:
    return "\n".join(paragraph_texts(doc))


SECTION_HEADINGS = ("SUMMARY", "SKILLS", "EXPERIENCE", "PROJECTS", "EDUCATION")


def section_text(doc: DocumentObject, heading: str) -> str:
    """Text of one section only, heading exclusive.

    Whole-document substring checks are too loose for anything whose values
    also occur in prose: dropping the entire "Frameworks & Libraries" skill
    row still passed a document-wide search, because "Spring Boot" and "React"
    appear in bullet text elsewhere. Assertions about a section's contents have
    to be scoped to that section.
    """
    texts = paragraph_texts(doc)
    assert heading in texts, f"section not found: {heading}"
    start = texts.index(heading) + 1
    end = start
    while end < len(texts) and texts[end] not in SECTION_HEADINGS:
        end += 1
    return "\n".join(texts[start:end])


def test_builds_without_error(resume: Resume) -> None:
    doc = build_resume_document(resume)
    assert isinstance(doc, DocumentObject)
    assert len(doc.paragraphs) > 0


def test_identity_and_summary_are_rendered(resume: Resume) -> None:
    text = document_text(build_resume_document(resume))
    assert resume.identity.name in text
    assert resume.summary in text


def test_every_employer_appears(resume: Resume) -> None:
    text = document_text(build_resume_document(resume))
    for employer in resume.experience:
        assert employer.employer in text, f"employer dropped: {employer.employer}"


def test_every_position_title_appears(resume: Resume) -> None:
    text = document_text(build_resume_document(resume))
    for employer in resume.experience:
        for position in employer.positions:
            assert position.title in text, f"position dropped: {position.title}"


def test_every_bullet_appears(resume: Resume) -> None:
    """The one that would have caught a dropped role."""
    text = document_text(build_resume_document(resume))
    for employer in resume.experience:
        for position in employer.positions:
            for bullet in position.bullets:
                assert bullet.text in text, f"bullet dropped: {bullet.text[:60]}"


def test_every_project_appears(resume: Resume) -> None:
    text = document_text(build_resume_document(resume))
    for project in resume.projects:
        assert project.name in text, f"project dropped: {project.name}"
        for bullet in project.bullets:
            assert bullet.text in text, f"project bullet dropped: {bullet.text[:60]}"


def test_every_skill_appears(resume: Resume) -> None:
    """Scoped to the SKILLS section on purpose -- see section_text."""
    skills_text = section_text(build_resume_document(resume), "SKILLS")
    for category, skills in resume.skills.items():
        assert category in skills_text, f"skill category dropped: {category}"
        for skill in skills:
            assert skill in skills_text, f"skill dropped: {category}/{skill}"


def test_education_appears(resume: Resume) -> None:
    if not resume.education:
        return
    education_text = section_text(build_resume_document(resume), "EDUCATION")
    for entry in resume.education:
        assert entry.institution in education_text


def test_expected_section_headings_present(resume: Resume) -> None:
    texts = paragraph_texts(build_resume_document(resume))
    assert "SUMMARY" in texts
    assert "SKILLS" in texts
    assert "EXPERIENCE" in texts


def test_projects_heading_tracks_whether_there_are_projects(resume: Resume) -> None:
    """An empty section must not leave a bare heading behind."""
    texts = paragraph_texts(build_resume_document(resume))
    assert ("PROJECTS" in texts) == bool(resume.projects)


def test_education_heading_tracks_whether_there_is_education(resume: Resume) -> None:
    texts = paragraph_texts(build_resume_document(resume))
    assert ("EDUCATION" in texts) == bool(resume.education)


def test_no_word_heading_styles_are_used(resume: Resume) -> None:
    """Explicit formatting only.

    Word's built-in heading styles carry theme-dependent formatting that
    renders inconsistently across Word versions and platforms, so the builder
    styles headings itself. Reintroducing them is called out in CLAUDE.md as a
    thing not to do.
    """
    doc = build_resume_document(resume)
    used = {p.style.name for p in doc.paragraphs if p.style is not None}
    assert not any(name.startswith("Heading") for name in used), used
    assert "Title" not in used


def test_section_headings_keep_with_next(resume: Resume) -> None:
    """A heading must never strand at the foot of a page."""
    doc = build_resume_document(resume)
    headings = {"SUMMARY", "SKILLS", "EXPERIENCE", "PROJECTS", "EDUCATION"}
    found = [p for p in doc.paragraphs if p.text in headings]
    assert found, "no section headings found"
    for p in found:
        assert p.paragraph_format.keep_with_next is True, (
            f"heading not bound to its content: {p.text}"
        )


def test_bullets_are_free_to_break(resume: Resume) -> None:
    """The deliberate non-invariant.

    Widow/orphan protection covers the header chain only. A bullet list that
    refuses to split wastes more page than the orphan it prevents, so bullets
    must not carry keep_with_next.
    """
    doc = build_resume_document(resume)
    bullets = [
        p
        for p in doc.paragraphs
        if p.style is not None and p.style.name == "List Bullet"
    ]
    assert bullets, "no bullets rendered"
    for p in bullets:
        assert p.paragraph_format.keep_with_next is not True, (
            f"bullet pinned to next paragraph: {p.text[:60]}"
        )


def test_page_margins_are_configured(resume: Resume) -> None:
    doc = build_resume_document(resume)
    section = doc.sections[0]
    assert section.top_margin == section.bottom_margin
    assert section.left_margin == section.right_margin
    assert section.left_margin > 0


def test_render_is_deterministic(resume: Resume) -> None:
    """Same input, same document.

    Not a byte comparison -- python-docx embeds timestamps -- but the visible
    content must not vary between runs.
    """
    first = document_text(build_resume_document(resume))
    second = document_text(build_resume_document(resume))
    assert first == second
