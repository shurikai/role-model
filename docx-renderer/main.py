import io

from fastapi import FastAPI, Response

from models import Resume
from renderer.docx_builder import build_resume_document

app = FastAPI()


@app.post("/render")
def render(resume: Resume) -> Response:
    doc = build_resume_document(resume)

    buffer = io.BytesIO()
    doc.save(buffer)
    content = buffer.getvalue()

    media_type = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    headers = {'Content-Disposition': 'attachment; filename="resume.docx"'}
    response = Response(content=content, status_code=200, headers=headers, media_type=media_type)
    return response
