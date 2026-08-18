/**
 * The unset-configuration guard.
 *
 * Lives in its own file because it has to reset the module registry to change
 * import.meta.env, which would leak into the other api-client tests.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

describe("VITE_API_BASE_URL validation", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("throws a configuration error when unset", async () => {
    vi.stubEnv("VITE_API_BASE_URL", "");
    const { apiFetch } = await import("./api-client");

    await expect(apiFetch("/applications")).rejects.toThrow(
      /VITE_API_BASE_URL is not set/,
    );
  });

  it("names the file to copy and shows the path prefix", async () => {
    vi.stubEnv("VITE_API_BASE_URL", "");
    const { apiFetch } = await import("./api-client");

    await expect(apiFetch("/applications")).rejects.toThrow(
      /\.env\.example.*http:\/\/localhost:8080\/api\/v1/s,
    );
  });

  it("does not throw when set", async () => {
    vi.stubEnv("VITE_API_BASE_URL", "http://example.test/api/v1");
    const { apiFetch } = await import("./api-client");

    await expect(apiFetch("/applications")).rejects.not.toThrow(
      /VITE_API_BASE_URL/,
    );
  });
});
