import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const { mockReplace, mockLoginWithSSO, mockListWorkspaces } = vi.hoisted(() => ({
  mockReplace: vi.fn(),
  mockLoginWithSSO: vi.fn(),
  mockListWorkspaces: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("@multica/core/auth", async () => {
  const actual = await vi.importActual<typeof import("@multica/core/auth")>("@multica/core/auth");
  return {
    ...actual,
    useAuthStore: (selector: (state: unknown) => unknown) => selector({ loginWithSSO: mockLoginWithSSO }),
  };
});

vi.mock("@multica/core/api", () => ({ api: { listWorkspaces: mockListWorkspaces } }));

import LoginPage from "./page";

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>;
}

describe("SSO login page", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLoginWithSSO.mockResolvedValue({ id: "u1", onboarded_at: "2026-01-01T00:00:00Z" });
    mockListWorkspaces.mockResolvedValue([{ id: "w1", slug: "platform" }]);
  });

  it("exchanges the SSO cookie and redirects into the workspace", async () => {
    render(<LoginPage />, { wrapper });
    await waitFor(() => expect(mockLoginWithSSO).toHaveBeenCalledOnce());
    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith("/platform/issues"));
  });

  it("shows a retry action when SSO exchange fails", async () => {
    mockLoginWithSSO.mockRejectedValue(new Error("SSO unavailable"));
    render(<LoginPage />, { wrapper });
    expect(await screen.findByRole("button", { name: "Retry" })).toBeInTheDocument();
  });
});
