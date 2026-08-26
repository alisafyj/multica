import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { configStore } from "@multica/core/config";
import { desktopAuthStorage } from "./desktop-auth-storage";

const getAuthToken = vi.fn<() => string>();
const clearAuthToken = vi.fn();

describe("desktopAuthStorage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getAuthToken.mockReturnValue("");
    (window as unknown as { desktopAPI: { getAuthToken: typeof getAuthToken; clearAuthToken: typeof clearAuthToken } }).desktopAPI = {
      getAuthToken,
      clearAuthToken,
    };
    window.localStorage.clear();
    configStore.setState({ useSySso: null });
  });

  afterEach(() => {
    configStore.setState({ useSySso: null });
  });

  // The auth boot check runs before /api/config resolves on every cold
  // start. With the mode unknown, the adapter must surface an existing
  // token instead of reporting "logged out" — the relaunch-relogin bug.
  it("falls back to whichever store holds a token while the SSO mode is unknown", () => {
    window.localStorage.setItem("multica_token", "legacy-jwt");
    expect(desktopAuthStorage.getItem("multica_token")).toBe("legacy-jwt");

    getAuthToken.mockReturnValue("sso-jwt");
    expect(desktopAuthStorage.getItem("multica_token")).toBe("sso-jwt");

    window.localStorage.removeItem("multica_token");
    getAuthToken.mockReturnValue("");
    expect(desktopAuthStorage.getItem("multica_token")).toBeNull();
  });

  it("reads only the mode's own store once the config resolved", () => {
    window.localStorage.setItem("multica_token", "legacy-jwt");
    getAuthToken.mockReturnValue("sso-jwt");

    configStore.setState({ useSySso: false });
    expect(desktopAuthStorage.getItem("multica_token")).toBe("legacy-jwt");

    configStore.setState({ useSySso: true });
    expect(desktopAuthStorage.getItem("multica_token")).toBe("sso-jwt");
  });

  it("keeps non-token keys and the mode-gated writes unchanged", () => {
    desktopAuthStorage.setItem("other_key", "value");
    expect(window.localStorage.getItem("other_key")).toBe("value");

    // Unknown mode: the token write stays parked (login cannot happen
    // before the config resolved, so nothing is lost).
    desktopAuthStorage.setItem("multica_token", "jwt");
    expect(window.localStorage.getItem("multica_token")).toBeNull();

    configStore.setState({ useSySso: false });
    desktopAuthStorage.setItem("multica_token", "jwt");
    expect(window.localStorage.getItem("multica_token")).toBe("jwt");

    desktopAuthStorage.removeItem("multica_token");
    expect(window.localStorage.getItem("multica_token")).toBeNull();
    expect(clearAuthToken).not.toHaveBeenCalled();

    configStore.setState({ useSySso: true });
    desktopAuthStorage.removeItem("multica_token");
    expect(clearAuthToken).toHaveBeenCalledTimes(1);
  });
});
