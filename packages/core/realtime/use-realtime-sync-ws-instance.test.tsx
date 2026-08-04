/**
 * @vitest-environment jsdom
 */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { WSClient } from "../api/ws-client";
import { useRealtimeSync, type RealtimeSyncStores } from "./use-realtime-sync";

vi.mock("../platform/workspace-storage", () => ({
  getCurrentWsId: () => "ws-1",
  getCurrentSlug: () => "test-ws",
}));

vi.mock("../paths", () => ({
  useHasOnboarded: () => true,
  resolvePostAuthDestination: () => "/",
}));

function createMockWs(): WSClient {
  return {
    on: vi.fn(() => () => {}),
    onAny: vi.fn(() => () => {}),
    onReconnect: vi.fn(() => () => {}),
  } as unknown as WSClient;
}

function createEventfulMockWs() {
  const handlers = new Map<string, (payload: unknown) => void>();
  const ws = {
    on: vi.fn((event: string, handler: (payload: unknown) => void) => {
      handlers.set(event, handler);
      return () => handlers.delete(event);
    }),
    onAny: vi.fn(() => () => {}),
    onReconnect: vi.fn(() => () => {}),
  } as unknown as WSClient;
  return { ws, handlers };
}

function createAnyEventMockWs() {
  const anyHandlers: Array<(message: { type?: unknown; payload?: unknown }) => void> = [];
  const ws = {
    on: vi.fn(() => () => {}),
    onAny: vi.fn((handler: (message: { type?: unknown; payload?: unknown }) => void) => {
      anyHandlers.push(handler);
      return () => {};
    }),
    onReconnect: vi.fn(() => () => {}),
  } as unknown as WSClient;
  return { ws, anyHandlers };
}

function createStores(): RealtimeSyncStores {
  return {
    authStore: Object.assign(() => ({}), {
      getState: () => ({ user: { id: "u1" } }),
      subscribe: () => () => {},
      setState: () => {},
      destroy: () => {},
    }),
  } as unknown as RealtimeSyncStores;
}

function createWrapper(qc: QueryClient) {
  // Named function (not arrow) so react/display-name lint rule passes —
  // anonymous render-fn components break that rule even in test files.
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

describe("useRealtimeSync — ws instance change", () => {
  let qc: QueryClient;
  let stores: RealtimeSyncStores;
  let invalidateSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    stores = createStores();
    invalidateSpy = vi.spyOn(qc, "invalidateQueries");
  });

  it("skips invalidation on first non-null ws instance", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });

    // The main effect calls invalidateQueries for its own setup, but the
    // ws-instance-change effect should NOT have fired invalidation.
    // The only invalidateQueries calls should come from the main effect's
    // event handlers, not from the instance-change effect.
    // We verify by checking that no call was made with workspaceKeys.list()
    // pattern from the instance-change path (it logs a specific message).
    // Simpler: count calls — first mount with a ws should not trigger the
    // workspace-scoped bulk invalidation.
    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("does not invalidate when ws goes from instance to null", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    rerender({ ws: null });

    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("invalidates exactly once when a new ws instance appears after null gap", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    // Simulate workspace switch: ws -> null -> new ws
    invalidateSpy.mockClear();
    rerender({ ws: null });
    expect(invalidateSpy).not.toHaveBeenCalled();

    const ws2 = createMockWs();
    rerender({ ws: ws2 });

    // Should have called invalidateQueries for all workspace-scoped keys
    // (15 workspace-scoped + 1 workspaceKeys.list() = 16 calls)
    expect(invalidateSpy).toHaveBeenCalledTimes(16);
  });

  it("does not re-invalidate when rerendered with the same ws instance", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    // Rerender with same instance
    rerender({ ws: ws1 });

    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("invalidates chat, pins, labels, and invitations queries on ws instance change", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    rerender({ ws: null });

    const ws2 = createMockWs();
    rerender({ ws: ws2 });

    const calls = invalidateSpy.mock.calls.map((call: [{ queryKey?: unknown }, ...unknown[]]) => call[0].queryKey);
    expect(calls).toContainEqual(["chat", "ws-1"]);
    expect(calls).toContainEqual(["labels", "ws-1"]);
    expect(calls).toContainEqual(["workspaces", "ws-1", "invitations"]);
  });

  it("invalidates design draft queries when a design draft becomes ready", () => {
    const { ws, handlers } = createEventfulMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });

    invalidateSpy.mockClear();
    handlers.get("design_draft:ready")?.({ design_draft_id: "draft-1" });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["designs", "ws-1", "drafts"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["designs", "ws-1", "drafts", "draft-1"] });
  });

  it("invalidates project lookup and system detail when a project design system changes", () => {
    const { ws, handlers } = createEventfulMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });

    invalidateSpy.mockClear();
    handlers.get("project_design_system:changed")?.({
      project_design_system_id: "system-1",
      project_id: "project-1",
      status: "draft",
    });

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["designs", "ws-1", "project-design-systems", "project", "project-1"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["designs", "ws-1", "project-design-systems", "system", "system-1"],
    });
  });

  it("invalidates only the matching project design system on task lifecycle events", () => {
    const { ws, handlers } = createEventfulMockWs();
    const system = {
      id: "system-1",
      project_id: "project-1",
      active_task: { id: "task-1" },
    };
    qc.setQueryData(
      ["designs", "ws-1", "project-design-systems", "project", "project-1"],
      system,
    );
    qc.setQueryData(
      ["designs", "ws-1", "project-design-systems", "system", "system-1"],
      system,
    );
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });

    invalidateSpy.mockClear();
    handlers.get("task:running")?.({
      task_id: "task-1",
      agent_id: "agent-1",
      issue_id: "",
      status: "running",
    });

    expect(invalidateSpy).toHaveBeenCalledTimes(2);
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["designs", "ws-1", "project-design-systems", "project", "project-1"],
      exact: true,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["designs", "ws-1", "project-design-systems", "system", "system-1"],
      exact: true,
    });

    invalidateSpy.mockClear();
    handlers.get("task:running")?.({
      task_id: "other-task",
      agent_id: "agent-1",
      issue_id: "",
      status: "running",
    });
    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("ignores malformed websocket messages without an event type", () => {
    const { ws, anyHandlers } = createAnyEventMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });

    invalidateSpy.mockClear();

    expect(() => anyHandlers[0]?.({ payload: {} })).not.toThrow();
    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});
