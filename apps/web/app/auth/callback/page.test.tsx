import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

const state = vi.hoisted(() => ({
  params: new URLSearchParams(),
  config: {
    useSySso: null as boolean | null,
    authConfigError: null as string | null,
    loadConfig: vi.fn(),
  },
}));
const mockPush = vi.hoisted(() => vi.fn());
const mockReplace = vi.hoisted(() => vi.fn());
const mockLoginWithGoogle = vi.hoisted(() => vi.fn());
const mockListWorkspaces = vi.hoisted(() => vi.fn());
const mockListMyInvitations = vi.hoisted(() => vi.fn());
const mockGetConfig = vi.hoisted(() => vi.fn());
const mockSetQueryData = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useSearchParams: () => state.params,
}));
vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => ({ setQueryData: mockSetQueryData }),
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
    useAuthStore: (selector: (value: { loginWithGoogle: typeof mockLoginWithGoogle }) => unknown) =>
      selector({ loginWithGoogle: mockLoginWithGoogle }),
  };
});
vi.mock("@multica/core/api", () => ({
  api: {
    getConfig: mockGetConfig,
    listWorkspaces: mockListWorkspaces,
    listMyInvitations: mockListMyInvitations,
    googleLogin: vi.fn(),
  },
}));

import CallbackPage from "./page";

describe("Google callback auth mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.params = new URLSearchParams({ code: "google-code" });
    state.config.useSySso = null;
    state.config.authConfigError = null;
    mockGetConfig.mockResolvedValue({ use_sy_sso: false });
    state.config.loadConfig.mockImplementation((request) => request());
    mockLoginWithGoogle.mockResolvedValue({
      id: "u1",
      email: "alice@example.com",
      onboarded_at: "2026-01-01T00:00:00Z",
    });
    mockListWorkspaces.mockResolvedValue([{ id: "w1", slug: "platform" }]);
    mockListMyInvitations.mockResolvedValue([]);
  });

  it("does not exchange Google credentials while config is unknown", () => {
    render(<CallbackPage />);

    expect(screen.getByText("Loading sign-in configuration")).toBeInTheDocument();
    expect(mockLoginWithGoogle).not.toHaveBeenCalled();
  });

  it("retries config failure without falling back to Google", () => {
    state.config.authConfigError = "Config unavailable";
    render(<CallbackPage />);

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(state.config.loadConfig).toHaveBeenCalledOnce();
    expect(mockGetConfig).toHaveBeenCalledOnce();
    expect(mockLoginWithGoogle).not.toHaveBeenCalled();
  });

  it("redirects away without exchanging Google credentials in SSO mode", async () => {
    state.config.useSySso = true;
    render(<CallbackPage />);

    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith("/login"));
    expect(mockLoginWithGoogle).not.toHaveBeenCalled();
  });

  it("exchanges Google credentials and preserves a safe next only in legacy mode", async () => {
    state.config.useSySso = false;
    state.params.set("state", "next:/invite/inv-1");
    render(<CallbackPage />);

    await waitFor(() => expect(mockLoginWithGoogle).toHaveBeenCalledOnce());
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/invite/inv-1"));
  });
});
