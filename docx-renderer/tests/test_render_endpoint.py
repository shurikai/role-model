"""The HTTP contract.

internal/renderer.Client on the Go side depends on all of this: the status
code, the media type, the attachment filename, and a 4xx rather than a 5xx
when the document is malformed. Changing any of it breaks a caller that lives
in another language and another repo directory, with nothing but these tests
connecting the two.
"""

import io
import zipfile

import pytest
from docx import Document
from fastapi.testclient import TestClient

from main import app

DOCX_MEDIA_TYPE = (
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)


@pytest.fixture
def client() -> TestClient:
    return TestClient(app)


def test_render_returns_a_docx(client: TestClient, resume_data: dict) -> None:
    response = client.post("/render", json=resume_data)
    assert response.status_code == 200
    assert response.headers["content-type"] == DOCX_MEDIA_TYPE


def test_render_sets_attachment_filename(client: TestClient, resume_data: dict) -> None:
    response = client.post("/render", json=resume_data)
    assert (
        response.headers["content-disposition"] == 'attachment; filename="resume.docx"'
    )


def test_rendered_bytes_are_a_readable_document(
    client: TestClient, resume_data: dict
) -> None:
    """Not just non-empty -- actually openable.

    A .docx is a zip archive; a truncated or half-written response would still
    have a length and a content type.
    """
    response = client.post("/render", json=resume_data)
    payload = io.BytesIO(response.content)

    assert zipfile.is_zipfile(payload), "response is not a zip archive"

    payload.seek(0)
    doc = Document(payload)
    text = "\n".join(p.text for p in doc.paragraphs)
    assert resume_data["identity"]["name"] in text
    assert resume_data["summary"] in text


def test_malformed_document_is_rejected(client: TestClient) -> None:
    response = client.post("/render", json={"nope": 1})
    assert response.status_code == 422


def test_missing_required_field_is_rejected(
    client: TestClient, minimal_resume_data: dict
) -> None:
    """A 422, not a 500.

    Validation happens at the boundary, so a bad document is the caller's
    error and says so -- it must not surface as a renderer crash.
    """
    del minimal_resume_data["identity"]
    response = client.post("/render", json=minimal_resume_data)
    assert response.status_code == 422


def test_bullet_without_provenance_is_rejected(
    client: TestClient, minimal_resume_data: dict
) -> None:
    """The provenance invariant, enforced over HTTP."""
    position = minimal_resume_data["experience"][0]["positions"][0]
    position["bullets"][0]["contribution_ids"] = []
    response = client.post("/render", json=minimal_resume_data)
    assert response.status_code == 422


def test_minimal_document_renders(
    client: TestClient, minimal_resume_data: dict
) -> None:
    response = client.post("/render", json=minimal_resume_data)
    assert response.status_code == 200
    assert zipfile.is_zipfile(io.BytesIO(response.content))


def test_empty_body_is_rejected(client: TestClient) -> None:
    response = client.post(
        "/render", content=b"", headers={"content-type": "application/json"}
    )
    assert response.status_code == 422
