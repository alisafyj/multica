import { describe, expect, it, vi } from "vitest";
import type { ApiClient } from "../api/client";
import type { StorageAdapter, User } from "../types";
import { createAuthStore } from "./store";

const fakeUser: User = {
  id: "u1",
  name: "Alice",
  email: "alice@example.com",
  avatar_url: null,
} as User;

function makeStorage(initial: Record<string, string> = {}): StorageAdapter & {
  snapshot: () => Record<string, string>;
} {
  const data = { ...initial };
  return {
    getItem: (k) => data[k] ?? null,
    setItem: (k, v) => {
      data[k] = v;
    },
    removeItem: (k) => {
      delete data[k];
    },
    snapshot: () => ({ ...data }),
  };
}

function makeApi(): ApiClient {
  return {
    setToken: vi.fn(),
  } as unknown as ApiClient;
}

describe("authStore", () => {
  it("publishes a retry request instead of silently ignoring it", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const store = createAuthStore({ api, storage });

    store.setState({ isLoading: true, status: "recovering" });
    store.getState().retryAuthentication();

    expect(store.getState().status).toBe("authenticating");
    expect(store.getState().retryGeneration).toBe(1);
  });

  it("explicit logout still clears credentials and publishes unauthenticated state", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const onLogout = vi.fn();
    const store = createAuthStore({ api, storage, onLogout });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().logout();

    expect(storage.snapshot().multica_token).toBeUndefined();
    expect(api.setToken).toHaveBeenCalledWith(null);
    expect(onLogout).toHaveBeenCalledOnce();
    expect(store.getState().user).toBeNull();
    expect(store.getState().status).toBe("unauthenticated");
    expect(store.getState().expired).toBe(false);
  });

  it("ends the session when the server rejects the credential", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const onLogout = vi.fn();
    const store = createAuthStore({ api, storage, onLogout });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().sessionExpired();

    expect(storage.snapshot().multica_token).toBeUndefined();
    expect(api.setToken).toHaveBeenCalledWith(null);
    expect(onLogout).toHaveBeenCalledOnce();
    expect(store.getState().user).toBeNull();
    expect(store.getState().isLoading).toBe(false);
    expect(store.getState().status).toBe("unauthenticated");
    expect(store.getState().expired).toBe(true);
  });

  // Desktop's deep link writes the token, then verifies it. A rejected token
  // never leaves "unauthenticated", so an idempotence guard placed before the
  // credential teardown would return with the invalid token still in storage,
  // to be replayed at the next launch. The old 401 handler always removed it.
  it("drops a rejected token even when there was no session to end", async () => {
    const storage = makeStorage();
    const api = {
      setToken: vi.fn(),
      getMe: vi.fn().mockRejectedValue(new Error("unauthorized")),
    } as unknown as ApiClient;
    const store = createAuthStore({ api, storage });
    store.setState({ user: null, status: "unauthenticated", isLoading: false });

    await expect(
      store.getState().loginWithToken("stale-deep-link-token"),
    ).rejects.toThrow();
    // Stands in for the api client's 401 hook, which fires inside getMe.
    store.getState().sessionExpired();

    expect(storage.snapshot().multica_token).toBeUndefined();
    expect(api.setToken).toHaveBeenLastCalledWith(null);
  });

  it("does not claim a session expired for a client that never had one", () => {
    const storage = makeStorage();
    const api = makeApi();
    const store = createAuthStore({ api, storage, cookieAuth: true });

    // Boot-time identity probe on a first visit: still "authenticating",
    // no stored credential.
    store.getState().sessionExpired();

    expect(store.getState().status).toBe("unauthenticated");
    expect(store.getState().expired).toBe(false);
  });

  it("flags expiry when a stored token is rejected at boot", () => {
    const storage = makeStorage({ multica_token: "stale" });
    const api = makeApi();
    const store = createAuthStore({ api, storage });

    store.getState().sessionExpired();

    expect(store.getState().expired).toBe(true);
    expect(storage.snapshot().multica_token).toBeUndefined();
  });

  it("runs the expiry handler instead of the logout one when given both", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const onLogout = vi.fn();
    const onSessionExpired = vi.fn();
    const store = createAuthStore({ api, storage, onLogout, onSessionExpired });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().sessionExpired();

    // Desktop's logout teardown stops the local daemon, which is the wrong
    // answer for a session the user did not choose to end (MUL-7028).
    expect(onSessionExpired).toHaveBeenCalledOnce();
    expect(onLogout).not.toHaveBeenCalled();
  });

  it("still runs the logout teardown for an explicit logout", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const onLogout = vi.fn();
    const onSessionExpired = vi.fn();
    const store = createAuthStore({ api, storage, onLogout, onSessionExpired });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().logout();

    expect(onLogout).toHaveBeenCalledOnce();
    expect(onSessionExpired).not.toHaveBeenCalled();
  });

  it("treats a burst of parallel 401s as the one expiry it is", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const onLogout = vi.fn();
    const store = createAuthStore({ api, storage, onLogout });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().sessionExpired();
    store.getState().sessionExpired();
    store.getState().sessionExpired();

    expect(onLogout).toHaveBeenCalledOnce();
  });

  it("clears the expired notice once the user signs back in", async () => {
    const storage = makeStorage();
    const api = {
      setToken: vi.fn(),
      getMe: vi.fn().mockResolvedValue(fakeUser),
    } as unknown as ApiClient;
    const store = createAuthStore({ api, storage });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().sessionExpired();
    expect(store.getState().expired).toBe(true);

    await store.getState().loginWithToken("fresh-token");

    expect(store.getState().status).toBe("authenticated");
    expect(store.getState().expired).toBe(false);
  });
});

describe("authStore.loginWithSSO", () => {
  it("uses the cookie session without persisting a bearer token", async () => {
    const storage = makeStorage();
    const api = {
      ssoSession: vi.fn().mockResolvedValue({ user: fakeUser }),
      setToken: vi.fn(),
    } as unknown as ApiClient;
    const onLogin = vi.fn();
    const store = createAuthStore({ api, storage, cookieAuth: true, onLogin });

    await store.getState().loginWithSSO();

    expect(store.getState().user).toEqual(fakeUser);
    expect(storage.snapshot()).toEqual({});
    expect(api.setToken).not.toHaveBeenCalled();
    expect(onLogin).toHaveBeenCalledOnce();
  });
});

describe("authStore legacy login", () => {
  it("sends email codes and persists verified tokens in token mode", async () => {
    const storage = makeStorage();
    const api = {
      sendCode: vi.fn().mockResolvedValue(undefined),
      verifyCode: vi.fn().mockResolvedValue({ token: "legacy-token", user: fakeUser }),
      setToken: vi.fn(),
    } as unknown as ApiClient;
    const store = createAuthStore({ api, storage });

    await store.getState().sendCode("alice@example.com");
    await store.getState().verifyCode("alice@example.com", "123456");

    expect(api.sendCode).toHaveBeenCalledWith("alice@example.com");
    expect(storage.snapshot().multica_token).toBe("legacy-token");
    expect(api.setToken).toHaveBeenCalledWith("legacy-token");
    expect(store.getState().user).toEqual(fakeUser);
  });

  it("uses the Google response without persisting a token in cookie mode", async () => {
    const storage = makeStorage();
    const api = {
      googleLogin: vi.fn().mockResolvedValue({ token: "cookie-token", user: fakeUser }),
      setToken: vi.fn(),
    } as unknown as ApiClient;
    const store = createAuthStore({ api, storage, cookieAuth: true });

    await store.getState().loginWithGoogle("google-code", "https://app.example.test/auth/callback");

    expect(storage.snapshot()).toEqual({});
    expect(api.setToken).not.toHaveBeenCalled();
    expect(store.getState().user).toEqual(fakeUser);
  });
});
