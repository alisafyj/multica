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
