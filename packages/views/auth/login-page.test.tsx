import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement, ReactNode } from "react";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../locales/en/common.json";
import enAuth from "../locales/en/auth.json";
import enSettings from "../locales/en/settings.json";

const sendCode = vi.hoisted(() => vi.fn());
const verifyCode = vi.hoisted(() => vi.fn());
const listWorkspaces = vi.hoisted(() => vi.fn());
const apiVerifyCode = vi.hoisted(() => vi.fn());
const setToken = vi.hoisted(() => vi.fn());
const getMe = vi.hoisted(() => vi.fn());
const issueCliToken = vi.hoisted(() => vi.fn());
const setQueryData = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-query", async () => {
  const actual = await vi.importActual<typeof import("@tanstack/react-query")>(
    "@tanstack/react-query",
  );
  return { ...actual, useQueryClient: () => ({ setQueryData }) };
});

vi.mock("@multica/core/auth", () => ({
  useAuthStore: {
    getState: () => ({ sendCode, verifyCode }),
  },
}));

vi.mock("@multica/core/api", () => ({
  api: { listWorkspaces, verifyCode: apiVerifyCode, setToken, getMe, issueCliToken },
}));

import { LoginPage, validateCliCallback } from "./login-page";

const resources = {
  en: { common: enCommon, auth: enAuth, settings: enSettings },
};

function Wrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={resources}>
      {children}
    </I18nProvider>
  );
}

function renderPage(ui: ReactElement) {
  return render(ui, { wrapper: Wrapper });
}

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    getMe.mockRejectedValue(new Error("unauthorized"));
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { href: "http://localhost:3000" },
    });
  });

  it("completes legacy email verification and seeds workspaces", async () => {
    sendCode.mockResolvedValue(undefined);
    verifyCode.mockResolvedValue(undefined);
    listWorkspaces.mockResolvedValue([{ id: "ws-1" }]);
    const onSuccess = vi.fn();
    const user = userEvent.setup();
    renderPage(<LoginPage onSuccess={onSuccess} />);

    await user.type(screen.getByLabelText(/email/i), "alice@example.com");
    await user.click(screen.getByRole("button", { name: /continue/i }));
    await user.type(await screen.findByRole("textbox", { hidden: true }), "123456");

    await waitFor(() => {
      expect(sendCode).toHaveBeenCalledWith("alice@example.com");
      expect(verifyCode).toHaveBeenCalledWith("alice@example.com", "123456");
      expect(setQueryData).toHaveBeenCalledWith(
        expect.arrayContaining(["workspaces", "list"]),
        [{ id: "ws-1" }],
      );
      expect(onSuccess).toHaveBeenCalledOnce();
    });
  });

  it("shows Google only when a Google handler is configured", () => {
    const { rerender } = renderPage(<LoginPage onSuccess={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /google/i })).not.toBeInTheDocument();

    rerender(
      <Wrapper>
        <LoginPage onSuccess={vi.fn()} onGoogleLogin={vi.fn()} />
      </Wrapper>,
    );
    expect(screen.getByRole("button", { name: /google/i })).toBeInTheDocument();
  });

  it("issues a CLI token for an existing cookie session", async () => {
    getMe.mockResolvedValueOnce({
      id: "u-1",
      email: "alice@example.com",
      name: "Alice",
    });
    issueCliToken.mockResolvedValueOnce({ token: "fresh-token" });
    const user = userEvent.setup();
    renderPage(
      <LoginPage
        onSuccess={vi.fn()}
        cliCallback={{ url: "http://localhost:9876/callback", state: "cli-state" }}
      />,
    );

    await user.click(await screen.findByRole("button", { name: /^authorize$/i }));

    await waitFor(() => {
      expect(issueCliToken).toHaveBeenCalledOnce();
      expect(window.location.href).toContain("token=fresh-token&state=cli-state");
    });
  });
});

describe("validateCliCallback", () => {
  it("allows local/private HTTP callbacks and rejects public or HTTPS callbacks", () => {
    expect(validateCliCallback("http://localhost:9876/callback")).toBe(true);
    expect(validateCliCallback("http://10.0.0.5:9876/callback")).toBe(true);
    expect(validateCliCallback("http://172.31.0.5:9876/callback")).toBe(true);
    expect(validateCliCallback("http://192.168.1.5:9876/callback")).toBe(true);
    expect(validateCliCallback("http://8.8.8.8:9876/callback")).toBe(false);
    expect(validateCliCallback("https://localhost:9876/callback")).toBe(false);
  });
});
