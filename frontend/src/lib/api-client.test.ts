import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { apiFetch, ApiError, setUnauthorizedHandler } from "./api-client";
import { setSession } from "./session";

function mockResponse(overrides: {
  ok: boolean;
  status: number;
  json?: () => Promise<unknown>;
}): Response {
  return {
    ok: overrides.ok,
    status: overrides.status,
    json: overrides.json ?? vi.fn().mockResolvedValue({}),
  } as unknown as Response;
}

describe("apiFetch", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    setUnauthorizedHandler(() => {});
  });

  it("sends an Authorization header when a session exists", async () => {
    setSession({ token: "tok-123", user: { id: "u1", email: "a@example.com" } });
    fetchMock.mockResolvedValueOnce(mockResponse({ ok: true, status: 200, json: vi.fn().mockResolvedValue({}) }));

    await apiFetch("/whoami");

    const [, init] = fetchMock.mock.calls[0];
    expect((init.headers as Headers).get("Authorization")).toBe("Bearer tok-123");
  });

  it("sends no Authorization header when no session exists", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse({ ok: true, status: 200, json: vi.fn().mockResolvedValue({}) }));

    await apiFetch("/whoami");

    const [, init] = fetchMock.mock.calls[0];
    expect((init.headers as Headers).get("Authorization")).toBeNull();
  });

  it("sets Content-Type only when a body is present", async () => {
    fetchMock.mockResolvedValue(mockResponse({ ok: true, status: 200, json: vi.fn().mockResolvedValue({}) }));

    await apiFetch("/with-body", { method: "POST", body: JSON.stringify({ a: 1 }) });
    await apiFetch("/no-body");

    const [, withBodyInit] = fetchMock.mock.calls[0];
    const [, noBodyInit] = fetchMock.mock.calls[1];
    expect((withBodyInit.headers as Headers).get("Content-Type")).toBe("application/json");
    expect((noBodyInit.headers as Headers).get("Content-Type")).toBeNull();
  });

  it("throws an ApiError matching the JSON error body on non-2xx", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse({
        ok: false,
        status: 422,
        json: vi.fn().mockResolvedValue({ error: "email is required", code: "validation_error" }),
      }),
    );

    await expect(apiFetch("/signup")).rejects.toMatchObject({
      message: "email is required",
      status: 422,
      code: "validation_error",
    });
  });

  it("falls back to a generic message and unknown_error code when the error body isn't JSON", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse({
        ok: false,
        status: 500,
        json: vi.fn().mockRejectedValue(new Error("not json")),
      }),
    );

    let caught: unknown;
    try {
      await apiFetch("/broken");
    } catch (err) {
      caught = err;
    }

    expect(caught).toBeInstanceOf(ApiError);
    expect((caught as ApiError).message).toBe("Request failed with status 500");
    expect((caught as ApiError).code).toBe("unknown_error");
  });

  it("calls the unauthorized handler before the caller's catch observes the rejection", async () => {
    const order: string[] = [];
    setUnauthorizedHandler(() => {
      order.push("handler");
    });
    fetchMock.mockResolvedValueOnce(
      mockResponse({
        ok: false,
        status: 401,
        json: vi.fn().mockResolvedValue({ error: "unauthorized", code: "unauthorized" }),
      }),
    );

    await apiFetch("/me").catch(() => {
      order.push("catch");
    });

    expect(order).toEqual(["handler", "catch"]);
  });

  it("resolves undefined on a 204 and never calls .json()", async () => {
    const jsonSpy = vi.fn().mockResolvedValue({});
    fetchMock.mockResolvedValueOnce(mockResponse({ ok: true, status: 204, json: jsonSpy }));

    const result = await apiFetch("/deleted");

    expect(result).toBeUndefined();
    expect(jsonSpy).not.toHaveBeenCalled();
  });
});
