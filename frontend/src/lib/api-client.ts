import { getSession } from "./session";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL;

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

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const session = getSession();
  const headers = new Headers(init.headers);

  if (session) {
    headers.set("Authorization", `Bearer ${session.token}`);
  }
  if (init.body) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
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

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}
