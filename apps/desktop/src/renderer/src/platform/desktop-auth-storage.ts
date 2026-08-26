import { configStore } from "@multica/core/config";
import type { StorageAdapter } from "@multica/core/types";

/**
 * Desktop auth-token storage. Two stores exist because the two login modes
 * hand the token over differently: SSO keeps it with the main process
 * (window.desktopAPI), the legacy email flow keeps it in localStorage.
 * `useSySso` from /api/config decides which one is authoritative.
 *
 * The config is fetched asynchronously, and the auth boot check reads this
 * adapter synchronously on mount — always before /api/config resolves on a
 * cold start. Reporting "no token" for that window made every desktop
 * relaunch land on the login page with a perfectly valid token still in
 * localStorage. While the mode is unknown, read whichever store holds a
 * token (SSO bridge first — it only ever holds SSO-issued tokens); a token
 * from the wrong store simply fails getMe with a 401, which is exactly the
 * login page the old behaviour forced unconditionally.
 *
 * Writes stay gated: a login can only happen after /api/config resolved
 * (the login page needs the mode to render), so `useSySso` is known by then.
 */
export const desktopAuthStorage: StorageAdapter = {
  getItem: (key) => {
    if (key !== "multica_token") return window.localStorage.getItem(key);
    const useSySso = configStore.getState().useSySso;
    if (useSySso === null) {
      return window.desktopAPI.getAuthToken() || window.localStorage.getItem(key);
    }
    return useSySso
      ? window.desktopAPI.getAuthToken()
      : window.localStorage.getItem(key);
  },
  setItem: (key, value) => {
    if (
      key !== "multica_token" ||
      configStore.getState().useSySso === false
    ) {
      window.localStorage.setItem(key, value);
    }
  },
  removeItem: (key) => {
    if (
      key === "multica_token" &&
      configStore.getState().useSySso === true
    ) {
      void window.desktopAPI.clearAuthToken();
    } else {
      window.localStorage.removeItem(key);
    }
  },
};
