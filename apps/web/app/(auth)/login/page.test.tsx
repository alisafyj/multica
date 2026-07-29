import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const state = vi.hoisted(() => ({
  params: new URLSearchParams(),
  config: {
    useSySso: null as boolean | null,
    authConfigError: null as string | null,
    googleClientId: "",
    loadConfig: vi.fn(),
  },
  auth: {
    user: null as null | { id: string; email: string; onboarded_at: string | null },
    isLoading: false,
    loginWithSSO: vi.fn(),
  },
}));
const mockPush = vi.hoisted(() => vi.fn());
const mockReplace = vi.hoisted(() => vi.fn());
const mockListWorkspaces = vi.hoisted(() => vi.fn());
const mockListMyInvitations = vi.hoisted(() => vi.fn());
const mockGetConfig = vi.hoisted(() => vi.fn());
const mockIssueCliToken = vi.hoisted(() => vi.fn());
const mockTranslate = vi.hoisted(() => vi.fn(() => "translated"));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useSearchParams: () => state.params,
}));

vi.mock("@multica/core/config", () => ({
  useConfigStore: (selector: (value: typeof state.config) => unknown) =>
    selector(state.config),
}));

vi.mock("@multica/core/auth", async () => {
  const actual = await vi.importActual<typeof import("@multica/core/auth")>(
    "@multica/core/auth",
  );
  return {
    ...actual,
    useAuthStore: Object.assign(
      (selector: (value: typeof state.auth) => unknown) => selector(state.auth),
      { getState: () => state.auth },
    ),
  };
});

vi.mock("@multica/core/api", () => ({
  api: {
    getConfig: mockGetConfig,
    listWorkspaces: mockListWorkspaces,
    listMyInvitations: mockListMyInvitations,
    issueCliToken: mockIssueCliToken,
  },
}));
vi.mock("@multica/views/auth", () => ({
  LoginPage: (props: { google?: unknown }) => (
    <div>
      Legacy login
      {props.google ? <span>Google enabled</span> : null}
    </div>
  ),
  validateCliCallback: () => true,
}));
vi.mock("@multica/views/i18n", () => ({
  useT: () => ({ t: mockTranslate }),
}));
vi.mock("@/features/auth/auth-cookie", () => ({
  setLoggedInCookie: vi.fn(),
}));

import LoginPage from "./page";

function wrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={new QueryClient()}>
      {children}
    </QueryClientProvider>
  );
}

describe("Web login auth mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.params = new URLSearchParams();
    state.config.useSySso = null;
    state.config.authConfigError = null;
    state.config.googleClientId = "";
    state.auth.user = null;
    state.auth.isLoading = false;
    state.auth.loginWithSSO.mockResolvedValue({
      id: "u1",
      email: "alice@example.com",
      onboarded_at: "2026-01-01T00:00:00Z",
    });
    mockListWorkspaces.mockResolvedValue([{ id: "w1", slug: "platform" }]);
    mockListMyInvitations.mockResolvedValue([]);
    mockGetConfig.mockResolvedValue({ use_sy_sso: false });
    state.config.loadConfig.mockImplementation((request) => request());
  });

  it("shows a stable loading state until config resolves", () => {
    render(<LoginPage />, { wrapper });

    expect(screen.getByText("Loading sign-in configuration")).toBeInTheDocument();
    expect(state.auth.loginWithSSO).not.toHaveBeenCalled();
    expect(screen.queryByText("Legacy login")).not.toBeInTheDocument();
  });

  it("shows config failure and retries the shared loader without choosing a mode", () => {
    state.config.authConfigError = "Config unavailable";
    render(<LoginPage />, { wrapper });

    expect(screen.getByText("Config unavailable")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(state.config.loadConfig).toHaveBeenCalledOnce();
    expect(mockGetConfig).toHaveBeenCalledOnce();
    expect(state.auth.loginWithSSO).not.toHaveBeenCalled();
    expect(screen.queryByText("Legacy login")).not.toBeInTheDocument();
  });

  it("renders legacy email and Google login only when use_sy_sso is false", () => {
    state.config.useSySso = false;
    state.config.googleClientId = "google-client";
    render(<LoginPage />, { wrapper });

    expect(screen.getByText("Legacy login")).toBeInTheDocument();
    expect(screen.getByText("Google enabled")).toBeInTheDocument();
    expect(state.auth.loginWithSSO).not.toHaveBeenCalled();
  });

  it("exchanges SSO and preserves a safe next destination only in SSO mode", async () => {
    state.config.useSySso = true;
    state.params = new URLSearchParams({ next: "/invite/inv-1" });
    render(<LoginPage />, { wrapper });

    await waitFor(() => expect(state.auth.loginWithSSO).toHaveBeenCalledOnce());
    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith("/invite/inv-1"));
    expect(screen.queryByText("Legacy login")).not.toBeInTheDocument();
  });

  it("drops an unsafe next destination in SSO mode", async () => {
    state.config.useSySso = true;
    state.params = new URLSearchParams({ next: "https://evil.example" });
    render(<LoginPage />, { wrapper });

    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith("/platform/issues"));
    expect(mockReplace).not.toHaveBeenCalledWith("https://evil.example");
  });

  it("hands an existing legacy session back to Desktop", async () => {
    state.config.useSySso = false;
    state.params = new URLSearchParams({ platform: "desktop" });
    state.auth.user = {
      id: "u1",
      email: "alice@example.com",
      onboarded_at: "2026-01-01T00:00:00Z",
    };
    mockIssueCliToken.mockResolvedValue({ token: "desktop-token" });
    const hrefSetter = vi.fn();
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...originalLocation,
        set href(value: string) {
          hrefSetter(value);
        },
      },
    });

    try {
      render(<LoginPage />, { wrapper });
      await waitFor(() => expect(mockIssueCliToken).toHaveBeenCalledOnce());
      await waitFor(() =>
        expect(hrefSetter).toHaveBeenCalledWith(
          "multica://auth/callback?token=desktop-token",
        ),
      );
    } finally {
      Object.defineProperty(window, "location", {
        configurable: true,
        value: originalLocation,
      });
    }
  });
});
