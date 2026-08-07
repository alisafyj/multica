/**
 * The SSO redirect target is the one string that `(auth)/login.tsx` and
 * `app/+native-intent.ts` MUST agree on. If they drift, Android SSO breaks
 * silently: expo-router consumes the redirect as a navigation, unmounts the
 * login screen, and the code-for-token exchange never runs — no error, no
 * request, just a dead "Unmatched Route" screen.
 *
 * These tests pin the exact URL shapes the flow produces on device:
 *   - the redirect Chrome Custom Tabs delivers (query string attached)
 *   - the dev-client's own deep links, which must NOT be swallowed
 */
import { describe, expect, it } from "vitest";
import {
  SSO_CALLBACK_PATH,
  SSO_CALLBACK_URL,
  isSsoCallbackUrl,
} from "./sso-callback";

describe("SSO callback constants", () => {
  it("builds the native redirect URL from the shared path", () => {
    expect(SSO_CALLBACK_URL).toBe(`multica://${SSO_CALLBACK_PATH}`);
  });
});

describe("isSsoCallbackUrl", () => {
  it("matches the real Android redirect, code and state attached", () => {
    // Verbatim shape observed on device (values shortened).
    expect(
      isSsoCallbackUrl(
        "multica://auth/mobile-callback?code=bPbtsHq_ugqK&state=wb3ZjdE4EG",
      ),
    ).toBe(true);
  });

  it("matches the bare redirect with no query string", () => {
    expect(isSsoCallbackUrl(SSO_CALLBACK_URL)).toBe(true);
  });

  it("matches regardless of scheme so a dev-client scheme still resolves", () => {
    expect(
      isSsoCallbackUrl("exp+multica-mobile://auth/mobile-callback?code=x"),
    ).toBe(true);
  });

  it("ignores a trailing slash", () => {
    expect(isSsoCallbackUrl("multica://auth/mobile-callback/?code=x")).toBe(
      true,
    );
  });

  it("does not swallow the dev-client bundle deep link", () => {
    expect(
      isSsoCallbackUrl(
        "exp+multica-mobile://expo-development-client/?url=http%3A%2F%2F192.168.1.2%3A8081",
      ),
    ).toBe(false);
  });

  it("does not swallow ordinary in-app deep links", () => {
    expect(isSsoCallbackUrl("multica://acme/issues/MUL-42")).toBe(false);
    expect(isSsoCallbackUrl("multica://")).toBe(false);
  });

  it("does not match a path that merely contains the callback segment", () => {
    expect(isSsoCallbackUrl("multica://auth/mobile-callback/extra")).toBe(
      false,
    );
  });
});
