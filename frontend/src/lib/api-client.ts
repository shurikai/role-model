import { getSession } from "./session";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL;

/**
 * The API origin plus its path prefix, e.g. http://localhost:8080/api/v1.
 *
 * Endpoint paths are concatenated onto this verbatim, so an unset value would
 * otherwise produce a request to `undefined/applications` — a 404 that reads
 * as a broken backend rather than missing configuration. Checked here rather
 * than at module load so importing the client never throws, which would take
 * the whole app down instead of the request that needs it.
 */
function apiBaseUrl(): string {
  if (!API_BASE_URL) {
    throw new Error(
      "VITE_API_BASE_URL is not set. Copy .env.example to .env in frontend/. " +
        "The value must include the API path prefix, e.g. " +
        "http://localhost:8080/api/v1.",
    );
  }
  return API_BASE_URL;
}

interface ErrorBody {
  error: string;
  code: string;
}

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(message: string, status: number, code: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

const GENERIC_ERROR_MESSAGE = "Something went wrong. Please try again.";

export function formatApiError(err: unknown): string {
  if (err instanceof ApiError) {
    return err.code === "internal_error" ? GENERIC_ERROR_MESSAGE : err.message;
  }
  return GENERIC_ERROR_MESSAGE;
}

let unauthorizedHandler: (() => void) | null = null;

export function setUnauthorizedHandler(fn: () => void): void {
  unauthorizedHandler = fn;
}

function handleUnauthorized(): void {
  unauthorizedHandler?.();
}

async function rawFetch(path: string, init: RequestInit): Promise<Response> {
  const session = getSession();
  const headers = new Headers(init.headers);

  if (session) {
    headers.set("Authorization", `Bearer ${session.token}`);
  }
  if (init.body) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${apiBaseUrl()}${path}`, {
    ...init,
    headers,
  });

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    let code = "unknown_error";

    try {
      const body = (await response.json()) as ErrorBody;
      message = body.error ?? message;
      code = body.code ?? code;
    } catch {
      // Response body wasn't JSON — fall back to the generic message above.
    }

    if (response.status === 401) {
      handleUnauthorized();
    }

    throw new ApiError(message, response.status, code);
  }

  return response;
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const response = await rawFetch(path, init);

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

// apiFetchBlob is for binary responses (e.g. the rendered .docx download) that
// apiFetch's response.json() parsing can't handle.
export async function apiFetchBlob(
  path: string,
  init: RequestInit = {},
): Promise<Blob> {
  const response = await rawFetch(path, init);
  return response.blob();
}
