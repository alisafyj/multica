// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Agent, Member } from "@multica/core/types";

const state = vi.hoisted(() => ({
  userId: "member-1",
  agents: [] as Agent[] | undefined,
  agentsFetched: true,
  agentsError: false,
  members: [] as Member[] | undefined,
  membersFetched: true,
  membersError: false,
}));

vi.mock("@tanstack/react-query", () => ({
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
}));
vi.mock("@/data/auth-store", () => ({
  useAuthStore: (selector: (value: { user: { id: string } }) => unknown) =>
    selector({ user: { id: state.userId } }),
}));
vi.mock("@/data/workspace-store", () => ({
  useWorkspaceStore: (
    selector: (value: { currentWorkspaceId: string }) => unknown,
  ) => selector({ currentWorkspaceId: "workspace-1" }),
}));
vi.mock("@/data/queries/agents", () => ({
  agentListOptions: () => ({ queryKey: ["agents"] }),
}));
vi.mock("@/data/queries/members", () => ({
  memberListOptions: () => ({ queryKey: ["members"] }),
}));

// Mock declarations must run before the module under test is imported.
// eslint-disable-next-line import/first
import { useWorkspaceAgentAvailability } from "./workspace-agent-availability";

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
    state.members = [{ user_id: "member-1", role: "member" } as Member];
    state.membersFetched = true;
    state.membersError = false;
  });

  it("returns none after successful empty queries", () => {
    state.agents = [];
    expect(useWorkspaceAgentAvailability()).toBe("none");
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

      expect(useWorkspaceAgentAvailability()).toBe("loading");
    },
  );

  it.each(["agents", "members"] as const)(
    "preserves cached positive availability when the %s query has a transient error",
    (query) => {
      state[`${query}Error`] = true;

      expect(useWorkspaceAgentAvailability()).toBe("available");
    },
  );
});
