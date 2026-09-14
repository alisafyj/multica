// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import type { Agent, Member } from "../types";

const state = vi.hoisted(() => ({
  userId: "member-1",
  agents: [] as Agent[] | undefined,
  agentsFetched: true,
  agentsError: false,
  members: [] as Member[] | undefined,
  membersFetched: true,
  membersError: false,
}));

vi.mock("../hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("../auth", () => ({
  useAuthStore: (selector: (value: { user: { id: string } }) => unknown) =>
    selector({ user: { id: state.userId } }),
}));
vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: (options: { queryKey: readonly unknown[] }) =>
      options.queryKey.includes("agents")
        ? {
            data: state.agents,
            isFetched: state.agentsFetched,
            isError: state.agentsError,
            isSuccess: state.agentsFetched && !state.agentsError,
          }
        : {
            data: state.members,
            isFetched: state.membersFetched,
            isError: state.membersError,
            isSuccess: state.membersFetched && !state.membersError,
          },
  };
});

import { useWorkspaceAgentAvailability } from "./use-workspace-agent-availability";

const sharedAgent = {
  id: "agent-1",
  runtime_id: "runtime-private",
  runtime_bound: true,
  archived_at: null,
  owner_id: "owner-1",
  permission_mode: "public_to",
  invocation_targets: [{ target_type: "workspace", target_id: null }],
} as Agent;

describe("useWorkspaceAgentAvailability", () => {
  beforeEach(() => {
    state.userId = "member-1";
    state.agents = [sharedAgent];
    state.agentsFetched = true;
    state.agentsError = false;
    state.members = [
      { user_id: "member-1", role: "member" } as Member,
    ];
    state.membersFetched = true;
    state.membersError = false;
  });

  it("allows a workspace member to execute a shared bound agent without runtime visibility", () => {
    const { result } = renderHook(() => useWorkspaceAgentAvailability());
    expect(result.current).toBe("available");
  });

  it("denies a viewer who is not a workspace member", () => {
    state.members = [];
    const { result } = renderHook(() => useWorkspaceAgentAvailability());
    expect(result.current).toBe("none");
  });

  it("returns none after successful empty queries", () => {
    state.agents = [];
    const { result } = renderHook(() => useWorkspaceAgentAvailability());
    expect(result.current).toBe("none");
  });

  it.each(["agents", "members"] as const)(
    "stays neutral when the %s query failed without data",
    (query) => {
      state.agents = query === "agents" ? undefined : [sharedAgent];
      state.members =
        query === "members"
          ? undefined
          : [{ user_id: "member-1", role: "member" } as Member];
      state[`${query}Error`] = true;

      const { result } = renderHook(() => useWorkspaceAgentAvailability());
      expect(result.current).toBe("loading");
    },
  );

  it.each(["agents", "members"] as const)(
    "preserves cached positive availability when the %s query has a transient error",
    (query) => {
      state[`${query}Error`] = true;

      const { result } = renderHook(() => useWorkspaceAgentAvailability());
      expect(result.current).toBe("available");
    },
  );

  it("stays loading while either authority input is unresolved", () => {
    state.membersFetched = false;
    state.members = undefined;
    const { result } = renderHook(() => useWorkspaceAgentAvailability());
    expect(result.current).toBe("loading");
  });
});
