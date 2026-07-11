from fastapi import FastAPI, Response
from models import Resume
from docx import Document
import io

app = FastAPI()

@app.post("/render")
def render(resume: Resume) -> Response:
    doc = Document()
    doc.add_paragraph(resume.identity.name)

    buffer = io.BytesIO()
    doc.save(buffer)
    content = buffer.getvalue()

    media_type = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    headers = { 'Content-Disposition': 'attachment; filename="resume.docx"' }
    response = Response(content=content, status_code=200, headers=headers, media_type=media_type)
    return response

