// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import type { Agent, AgentRuntime, AgentTask } from "../types";

const queryState = vi.hoisted(() => ({
  agents: undefined as Agent[] | undefined,
  runtimes: undefined as AgentRuntime[] | undefined,
  snapshot: undefined as AgentTask[] | undefined,
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: (options: { queryKey: readonly unknown[] }) => {
      const key = options.queryKey;
      if (key.includes("agent-task-snapshot")) {
        return { data: queryState.snapshot, isError: false };
      }
      if (key.includes("runtimes")) {
        return { data: queryState.runtimes, isError: false };
      }
      return { data: queryState.agents, isError: false };
    },
  };
});

import { useAgentPresenceDetail } from "./use-agent-presence";

const agent = {
  id: "agent-1",
  runtime_id: "runtime-1",
  runtime_bound: true,
  archived_at: null,
  max_concurrent_tasks: 1,
} as Agent;

const runtime = {
  id: "runtime-1",
  status: "online",
  runtime_mode: "local",
  last_seen_at: new Date().toISOString(),
} as AgentRuntime;

describe("useAgentPresenceDetail runtime visibility", () => {
  beforeEach(() => {
    queryState.agents = [agent];
    queryState.runtimes = [];
    queryState.snapshot = [];
  });

  it("reports unknown when the agent is bound but its private runtime is hidden", () => {
    const { result } = renderHook(() =>
      useAgentPresenceDetail("workspace-1", "agent-1"),
    );

    expect(result.current).toMatchObject({ availability: "unknown" });
  });

  it("uses visible runtime health for the owner", () => {
    queryState.runtimes = [runtime];
    const { result, rerender } = renderHook(() =>
      useAgentPresenceDetail("workspace-1", "agent-1"),
    );
    expect(result.current).toMatchObject({ availability: "online" });

    queryState.runtimes = [
      {
        ...runtime,
        status: "offline",
        last_seen_at: "2020-01-01T00:00:00Z",
      },
    ];
    rerender();
    expect(result.current).toMatchObject({ availability: "offline" });
  });

  it("keeps an explicitly unbound agent offline", () => {
    queryState.agents = [{ ...agent, runtime_id: "", runtime_bound: false }];
    const { result } = renderHook(() =>
      useAgentPresenceDetail("workspace-1", "agent-1"),
    );

    expect(result.current).toMatchObject({ availability: "offline" });
  });

  it("stays loading until every presence input has resolved", () => {
    queryState.runtimes = undefined;
    const { result } = renderHook(() =>
      useAgentPresenceDetail("workspace-1", "agent-1"),
    );

    expect(result.current).toBe("loading");
  });
});
