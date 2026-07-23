Context: docx-renderer is a stateless FastAPI service that accepts resume JSON
(validated against models.py's Resume schema) via POST /render and returns a
.docx file built with python-docx. It currently works end-to-end but was built
as a learning exercise, with all document-generation logic inline in the route
handler in main.py.

Goals for this refactor:

1. Extract document generation out of the route handler into a separate
   rendering module (e.g. renderer/docx_builder.py), so main.py only handles
   HTTP concerns (receiving the request, returning the Response) and the
   renderer module contains no FastAPI imports.

2. Break the monolithic render function into one function per resume section
   (header/identity, summary, experience, education, skills, projects), each
   taking the shared `doc` object and the relevant slice of the Resume model.

3. Keep renderer functions accepting the existing Pydantic models directly
   (Resume, ProjectEntry, etc.) — no new DTO/view-model layer for now. When
   writing the projects-rendering function specifically, only read the
   fields actually used for display (name, tagline, started_on, ended_on,
   repo_url, live_url, writeup_url, bullets) and leave tags/role/status/
   force_include/force_exclude/project_id untouched, even though they're
   present on the model. This is a deliberate deferral, not an oversight —
   a DTO boundary can be introduced later if backend schema churn actually
   causes breakage.

4. Fix known rough edges called out in code comments / TODOs:
   - [education] guard optional fields (degree, field_of_study, graduated)
     individually instead of unguarded string interpolation
   - [skills] guard against empty skill lists per category
   - [identity] join contact fields (email, phone, links) into a single
     compact line instead of one paragraph per field
   - [projects] credentials section is still unimplemented, currently a TODO

Do NOT change: the Pydantic models in models.py, the overall visual structure
we've settled on (name as level-0 heading, sections at level-1, employers at
level-2, titles at level-3), or the existing bullet/date-formatting helpers
unless a genuine bug is found.

Read main.py and models.py fully before proposing a plan. Confirm the plan
with me before making changes.
