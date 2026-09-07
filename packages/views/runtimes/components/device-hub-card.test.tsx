// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AgentRuntime, RuntimeDeviceHub } from "@multica/core/types";
import enCommon from "../../locales/en/common.json";
import enRuntimes from "../../locales/en/runtimes.json";
import { DeviceHubCard } from "./device-hub-card";

// Canonical parsing for the hub shape lives in
// packages/core/api/runtime-device-hub-schema.test.ts; this suite keeps the
// wiring: what the owner sees, what a reader sees, and the switch.

const TEST_RESOURCES = { en: { common: enCommon, runtimes: enRuntimes } };

const mocks = vi.hoisted(() => ({
  hub: null as unknown,
  mutate: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: mocks.hub, isLoading: false }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/runtimes", () => ({
  runtimeDeviceHubOptions: (id: string) => ({ queryKey: ["runtimes", "device-hub", id] }),
  useUpdateRuntime: () => ({ mutate: mocks.mutate, isPending: false }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const RUNTIME = {
  id: "rt-1",
  daemon_id: "daemon-1",
  name: "mac-mini",
  runtime_mode: "local",
  provider: "claude",
  status: "online",
  test_host_enabled: false,
} as unknown as AgentRuntime;

function hub(over: Partial<RuntimeDeviceHub> = {}): RuntimeDeviceHub {
  return {
    reachable: true,
    url: "http://127.0.0.1:18801",
    version: "0.1.0",
    adb: true,
    devices: 2,
    phones: 1,
    leases: 0,
    pairing_url: "ws://10.0.0.5:18800/phone?code=ABCD2345",
    pairing_code: "ABCD2345",
    reported_at: new Date().toISOString(),
    ...over,
  };
}

function renderCard(canEdit: boolean, runtime: AgentRuntime = RUNTIME) {
  return render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <DeviceHubCard runtime={runtime} canEdit={canEdit} />
    </I18nProvider>,
  );
}

afterEach(() => {
  cleanup();
  mocks.hub = null;
  mocks.mutate.mockReset();
});

describe("DeviceHubCard", () => {
  it("shows the hub summary and lets an editor reveal the pairing code", () => {
    mocks.hub = hub();
    renderCard(true);
    expect(screen.getByText("Hub 0.1.0 online")).toBeTruthy();
    expect(screen.getByText("2 devices · 1 apps · 0 leases")).toBeTruthy();
    expect(screen.queryByText("ABCD2345")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /show pairing code/i }));
    expect(screen.getByText("ABCD2345")).toBeTruthy();
    expect(screen.getByText("ws://10.0.0.5:18800/phone?code=ABCD2345")).toBeTruthy();
  });

  it("offers no pairing to a reader and disables the test-host switch", () => {
    mocks.hub = hub({ pairing_url: null, pairing_code: null });
    renderCard(false);
    expect(screen.queryByRole("button", { name: /pairing code/i })).toBeNull();
    const toggle = screen.getByRole("switch", { name: "Test host" });
    // Base UI marks a disabled switch with aria-disabled / data-disabled rather
    // than the native attribute on every render path; accept any of them.
    const disabled =
      toggle.hasAttribute("disabled") ||
      toggle.getAttribute("aria-disabled") === "true" ||
      toggle.hasAttribute("data-disabled");
    expect(disabled).toBe(true);
  });

  it("explains a missing hub", () => {
    mocks.hub = hub({ reachable: false, pairing_url: null, pairing_code: null, reported_at: null });
    renderCard(true);
    expect(screen.getByText("No device hub on this machine")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /pairing code/i })).toBeNull();
  });

  it("patches test_host_enabled when the owner flips the switch", () => {
    mocks.hub = hub();
    renderCard(true);
    fireEvent.click(screen.getByRole("switch", { name: "Test host" }));
    expect(mocks.mutate).toHaveBeenCalledWith(
      { runtimeId: "rt-1", patch: { test_host_enabled: true } },
      expect.anything(),
    );
  });
});
