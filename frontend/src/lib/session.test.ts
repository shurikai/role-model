import { describe, it, expect } from "vitest";
import { getSession, setSession, clearSession } from "./session";

describe("session", () => {
  it("returns null when localStorage is empty", () => {
    expect(getSession()).toBeNull();
  });

  it("round-trips the exact object through setSession/getSession", () => {
    const session = {
      token: "abc123",
      user: { id: "user-1", email: "a@example.com" },
    };
    setSession(session);
    expect(getSession()).toEqual(session);
  });

  it("removes the key on clearSession so subsequent getSession is null", () => {
    setSession({
      token: "abc123",
      user: { id: "user-1", email: "a@example.com" },
    });
    clearSession();
    expect(getSession()).toBeNull();
  });

  it("returns null without throwing when the stored value is malformed JSON", () => {
    localStorage.setItem("role_model_session", "not json");
    expect(() => getSession()).not.toThrow();
    expect(getSession()).toBeNull();
  });
});
