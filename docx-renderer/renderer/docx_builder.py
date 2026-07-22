from datetime import datetime

from docx import Document

from models import EducationEntry, EmployerBlock, Identity, ProjectEntry, Resume


def build_resume_document(resume: Resume) -> Document:
    doc = Document()

    _render_identity(doc, resume.identity)
    _render_summary(doc, resume.summary)
    _render_skills(doc, resume.skills)
    _render_experience(doc, resume.experience)
    _render_projects(doc, resume.projects)
    _render_education(doc, resume.education)

    return doc


def _render_identity(doc: Document, identity: Identity) -> None:
    doc.add_heading(identity.name, level=0)

    if identity.headline:
        doc.add_paragraph(identity.headline)

    if identity.location:
        doc.add_paragraph(identity.location)

    contact_parts = [identity.email]
    if identity.phone:
        contact_parts.append(identity.phone)
    if identity.linkedin_url:
        contact_parts.append(identity.linkedin_url)
    if identity.github_url:
        contact_parts.append(identity.github_url)
    if identity.site_url:
        contact_parts.append(identity.site_url)
    doc.add_paragraph(" · ".join(contact_parts))


def _render_summary(doc: Document, summary: str) -> None:
    doc.add_heading("SUMMARY", level=1)
    doc.add_paragraph(summary)


def _render_skills(doc: Document, skills: dict[str, list[str]]) -> None:
    doc.add_heading("SKILLS", level=1)
    for category, skills_list in skills.items():
        if skills_list:
            line = f"{category}: {', '.join(skills_list)}"
            doc.add_paragraph(line, style="List Bullet")


def _render_experience(doc: Document, experience: list[EmployerBlock]) -> None:
    doc.add_heading("EXPERIENCE", level=1)
    for employer_block in experience:
        doc.add_heading(employer_block.employer, level=2)

        for position in employer_block.positions:
            doc.add_heading(position.title, level=3)
            started = format_date(position.started_on)
            ended = format_date(position.ended_on) if position.ended_on else "Present"
            doc.add_paragraph(f"{started} – {ended}")

            for bullet in position.bullets:
                doc.add_paragraph(bullet.text, style="List Bullet")


def _render_projects(doc: Document, projects: list[ProjectEntry]) -> None:
    if not projects:
        return

    doc.add_heading("PROJECTS", level=1)
    for project in projects:
        p = doc.add_paragraph()
        run = p.add_run(project.name)
        run.bold = True

        if project.tagline:
            p.add_run(f" - {project.tagline}")

        if project.started_on:
            ended = format_date(project.ended_on) if project.ended_on else "Present"
            doc.add_paragraph(f"{format_date(project.started_on)} – {ended}")

        for bullet in project.bullets:
            doc.add_paragraph(bullet.text, style="List Bullet")

        if project.writeup_url:
            doc.add_paragraph(project.writeup_url)
        if project.repo_url:
            doc.add_paragraph(project.repo_url)
        if project.live_url:
            doc.add_paragraph(project.live_url)


def _render_education(doc: Document, education: list[EducationEntry]) -> None:
    doc.add_heading("EDUCATION", level=1)
    for education_block in education:
        parts = [education_block.institution]
        if education_block.degree:
            parts.append(education_block.degree)
        if education_block.field_of_study:
            parts.append(education_block.field_of_study)
        if education_block.graduated:
            parts.append(education_block.graduated)
        doc.add_paragraph(" - ".join(parts))


def format_date(value: str) -> str:
    return datetime.strptime(value, "%Y-%m").strftime("%b %Y")
