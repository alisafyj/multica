import { describe, expect, it } from "vitest";
import {
  createDesktopAuthorization,
  readDesktopCallback,
} from "./desktop-auth";

describe("desktop SSO", () => {
  it("builds a PKCE authorization URL without a token", () => {
    const auth = createDesktopAuthorization(
      "https://api.example.test",
      "multica://auth/callback",
    );
    const url = new URL(auth.url);

    expect(url.pathname).toBe("/auth/sso/authorize");
    expect(url.searchParams.get("client_id")).toBe("desktop");
    expect(url.searchParams.get("code_challenge_method")).toBe("S256");
    expect(url.searchParams.get("code_challenge")).toBeTruthy();
    expect(url.searchParams.has("token")).toBe(false);
    expect(auth.verifier.length).toBeGreaterThanOrEqual(43);
  });

  it("accepts only a code with the matching state", () => {
    const pending = createDesktopAuthorization(
      "https://api.example.test",
      "multica://auth/callback",
    );
    expect(
      readDesktopCallback(
        `multica://auth/callback?code=code-1&state=${pending.state}`,
        pending,
      ),
    ).toBe("code-1");
    expect(() =>
      readDesktopCallback(
        "multica://auth/callback?code=code-1&state=wrong",
        pending,
      ),
    ).toThrow("state");
  });
});
