import { describe, expect, it } from "vitest";
import {
  BatchDeleteIssuesResponseSchema,
  BatchUpdateIssuesResponseSchema,
  DashboardAgentRunTimeListSchema,
  DashboardUsageByAgentListSchema,
  DashboardUsageDailyListSchema,
  EMPTY_PROJECT_DESIGN_SYSTEM,
  DesignRestoreTaskSchema,
  ListDesignDeliveriesResponseSchema,
  ListDesignRestoreTasksResponseSchema,
  DuplicateIssueErrorBodySchema,
  EMPTY_BATCH_DELETE_ISSUES_RESPONSE,
  EMPTY_BATCH_UPDATE_ISSUES_RESPONSE,
  EMPTY_USER,
  ListIssuesResponseSchema,
  ProjectDesignSystemSchema,
  RuntimeHourlyActivityListSchema,
  RuntimeUsageByAgentListSchema,
  RuntimeUsageByHourListSchema,
  RuntimeUsageListSchema,
  SquadListSchema,
  SquadSchema,
  UserSchema,
} from "./schemas";
import { parseWithFallback } from "./schema";

const baseIssue = {
  id: "11111111-1111-1111-1111-111111111111",
  workspace_id: "ws-1",
  number: 1,
  identifier: "MUL-1",
  title: "Test",
  description: null,
  status: "todo",
  priority: "medium",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: null,
  position: 0,
  start_date: null,
  due_date: null,
  metadata: {},
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("IssueSchema (via ListIssuesResponseSchema)", () => {
  it("accepts a primitive metadata KV map", () => {
    const payload = {
      issues: [
        {
          ...baseIssue,
          metadata: { pipeline_status: "waiting", pr_number: 3, is_blocked: true },
        },
      ],
      total: 1,
    };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.metadata).toEqual({
      pipeline_status: "waiting",
      pr_number: 3,
      is_blocked: true,
    });
  });

  it("defaults metadata to {} when the server omits it (older backend)", () => {
    const { metadata: _omit, ...issueWithoutMetadata } = baseIssue;
    const payload = { issues: [issueWithoutMetadata], total: 1 };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.metadata).toEqual({});
  });

  it("rejects metadata with non-primitive values (nested object)", () => {
    const payload = {
      issues: [{ ...baseIssue, metadata: { nested: { x: 1 } } }],
      total: 1,
    };
    expect(ListIssuesResponseSchema.safeParse(payload).success).toBe(false);
  });
});

describe("batch issue response schemas", () => {
  it("defaults missing counts to zero", () => {
    expect(BatchUpdateIssuesResponseSchema.parse({}).updated).toBe(0);
    expect(BatchUpdateIssuesResponseSchema.parse({}).skipped).toEqual([]);
    expect(BatchDeleteIssuesResponseSchema.parse({}).deleted).toBe(0);
  });

  it("preserves skipped issue details for partial batch updates", () => {
    const parsed = BatchUpdateIssuesResponseSchema.parse({
      updated: 1,
      skipped: [{
        issue_id: "issue-1",
        identifier: "MUL-1",
        title: "UI设计",
        reason: "UI design issue requires completed UI restore or raw design fallback handoff before completion",
      }],
    });

    expect(parsed.skipped).toEqual([{
      issue_id: "issue-1",
      identifier: "MUL-1",
      title: "UI设计",
      reason: "UI design issue requires completed UI restore or raw design fallback handoff before completion",
    }]);
  });

  it("falls back when counts drift to the wrong type", () => {
    const update = parseWithFallback(
      { updated: "1" },
      BatchUpdateIssuesResponseSchema,
      EMPTY_BATCH_UPDATE_ISSUES_RESPONSE,
      { endpoint: "POST /api/issues/batch-update" },
    );
    const deleted = parseWithFallback(
      { deleted: "1" },
      BatchDeleteIssuesResponseSchema,
      EMPTY_BATCH_DELETE_ISSUES_RESPONSE,
      { endpoint: "POST /api/issues/batch-delete" },
    );

    expect(update.updated).toBe(0);
    expect(update.skipped).toEqual([]);
    expect(deleted.deleted).toBe(0);
  });
});

describe("ListDesignDeliveriesResponseSchema", () => {
  const delivery = {
    id: "delivery-1",
    workspace_id: "ws-1",
    project_id: null,
    source_issue_id: "issue-ui",
    target_issue_id: "issue-fe",
    file_id: "file-1",
    revision_id: "revision-1",
    scope: {
      version: "1.0",
      items: [{ frameId: "frame-1", frameName: "Main" }],
    },
    status: "active",
    delivered_by: null,
    delivered_at: "2026-06-30T00:00:00Z",
    cancelled_by: null,
    cancelled_at: null,
    cancel_reason: null,
    audit_metadata: {},
    created_at: "2026-06-30T00:00:00Z",
    updated_at: "2026-06-30T00:00:00Z",
  };

  it("defaults deliveries to [] when an older backend omits the field", () => {
    const parsed = ListDesignDeliveriesResponseSchema.parse({});
    expect(parsed.deliveries).toEqual([]);
  });

  it("preserves unknown delivery fields for forward compatibility", () => {
    const parsed = ListDesignDeliveriesResponseSchema.parse({
      deliveries: [{ ...delivery, handoff_summary: "Ready for frontend" }],
    });
    expect(parsed.deliveries[0]?.handoff_summary).toBe("Ready for frontend");
  });

  it("defaults nullable fields that older backends may omit", () => {
    const {
      project_id: _projectId,
      delivered_by: _deliveredBy,
      cancelled_by: _cancelledBy,
      cancelled_at: _cancelledAt,
      cancel_reason: _cancelReason,
      audit_metadata: _auditMetadata,
      ...legacyDelivery
    } = delivery;
    const parsed = ListDesignDeliveriesResponseSchema.parse({ deliveries: [legacyDelivery] });
    expect(parsed.deliveries[0]?.project_id).toBe(null);
    expect(parsed.deliveries[0]?.delivered_by).toBe(null);
    expect(parsed.deliveries[0]?.cancelled_by).toBe(null);
    expect(parsed.deliveries[0]?.cancelled_at).toBe(null);
    expect(parsed.deliveries[0]?.cancel_reason).toBe(null);
    expect(parsed.deliveries[0]?.audit_metadata).toEqual({});
  });

  it("accepts cancellation audit fields", () => {
    const parsed = ListDesignDeliveriesResponseSchema.parse({
      deliveries: [{
        ...delivery,
        status: "cancelled",
        cancelled_by: "user-1",
        cancelled_at: "2026-06-30T01:00:00Z",
        cancel_reason: "设计稿需要重新确认",
        audit_metadata: { cancel_reason: "设计稿需要重新确认" },
      }],
    });
    expect(parsed.deliveries[0]?.cancelled_by).toBe("user-1");
    expect(parsed.deliveries[0]?.cancel_reason).toBe("设计稿需要重新确认");
    expect(parsed.deliveries[0]?.audit_metadata.cancel_reason).toBe("设计稿需要重新确认");
  });
});

describe("ProjectDesignSystemSchema", () => {
  it("downgrades unknown status and null collections without throwing", () => {
    const parsed = ProjectDesignSystemSchema.parse({
      workspace_id: "ws-1",
      project_id: "project-1",
      status: "future_server_status",
      content: null,
      activity: null,
    });

    expect(parsed.status).toBe("unestablished");
    expect(parsed.content).toEqual({
      sections: [],
      token_groups: [],
      locators: [],
      preview_html: "",
      integrity_sha256: "",
    });
    expect(parsed.activity).toEqual([]);
  });

  it("defaults missing content arrays and discards malformed locators", () => {
    const parsed = ProjectDesignSystemSchema.parse({
      id: "system-1",
      workspace_id: "ws-1",
      project_id: "project-1",
      status: "draft",
      content: {
        preview_html: "<main>CRM</main>",
        integrity_sha256: "sha-1",
        locators: [{ id: 42, kind: "component", label: "Button" }],
      },
    });

    expect(parsed.content.sections).toEqual([]);
    expect(parsed.content.token_groups).toEqual([]);
    expect(parsed.content.locators).toEqual([]);
    expect(parsed.content.preview_html).toBe("<main>CRM</main>");
  });

  it("falls back to an empty unestablished response for malformed top-level data", () => {
    const parsed = parseWithFallback(
      null,
      ProjectDesignSystemSchema,
      EMPTY_PROJECT_DESIGN_SYSTEM,
      { endpoint: "GET /api/project-design-systems/{id}" },
    );

    expect(parsed).toEqual(EMPTY_PROJECT_DESIGN_SYSTEM);
  });
});

describe("DesignRestoreTaskSchema", () => {
  const task = {
    id: "task-1",
    workspace_id: "ws-1",
    file_id: "file-1",
    revision_id: "revision-1",
    issue_id: "issue-fe",
    agent_task_id: null,
    status: "queued",
    input: { version: "1.0" },
    result: {},
    error: null,
    created_by: "user-1",
    created_at: "2026-06-30T00:00:00Z",
    updated_at: "2026-06-30T00:00:00Z",
  };

  it("defaults delivery_id to null for older restore task responses", () => {
    const parsed = DesignRestoreTaskSchema.parse(task);
    expect(parsed.delivery_id).toBe(null);
  });

  it("preserves delivery_id on task list responses", () => {
    const parsed = ListDesignRestoreTasksResponseSchema.parse({
      tasks: [{ ...task, delivery_id: "delivery-1" }],
    });
    expect(parsed.tasks[0]?.delivery_id).toBe("delivery-1");
  });

  it("defaults execution_status to null for older restore task responses", () => {
    const parsed = DesignRestoreTaskSchema.parse(task);
    expect(parsed.execution_status).toBe(null);
  });

  it("preserves execution_status diagnostics on task responses", () => {
    const parsed = DesignRestoreTaskSchema.parse({
      ...task,
      execution_status: {
        agent_task_id: "agent-task-1",
        agent_task_status: "queued",
        agent_task_created_at: "2026-07-03T01:00:00Z",
        agent_task_dispatched_at: null,
        agent_task_started_at: null,
        agent_task_completed_at: null,
        agent_task_error: null,
        agent_task_wait_reason: null,
        runtime_id: "runtime-1",
        runtime_status: "offline",
        runtime_last_seen_at: "2026-07-03T00:50:00Z",
        last_message_seq: null,
        last_message_at: null,
        phase: "waiting_runtime",
        reason: "runtime_offline",
        severity: "warning",
      },
    });

    expect(parsed.execution_status?.phase).toBe("waiting_runtime");
    expect(parsed.execution_status?.reason).toBe("runtime_offline");
    expect(parsed.execution_status?.runtime_status).toBe("offline");
  });
});

// The duplicate-issue branch in create-issue.tsx feeds ApiError.body
// (typed as `unknown`) through this schema. Any future server drift that
// loses the contract MUST fail the parse so the UI falls back to a normal
// error toast instead of rendering an empty / partial duplicate card.
describe("DuplicateIssueErrorBodySchema", () => {
  const valid = {
    code: "active_duplicate_issue",
    error: "An active issue with this title already exists: MUL-12 – Login bug",
    issue: {
      id: "11111111-1111-1111-1111-111111111111",
      identifier: "MUL-12",
      title: "Login bug",
    },
  };

  it("accepts a well-formed body", () => {
    expect(DuplicateIssueErrorBodySchema.safeParse(valid).success).toBe(true);
  });

  it("accepts unknown extra fields via .loose()", () => {
    const forwardCompat = {
      ...valid,
      hint: "Try a different title",
      issue: { ...valid.issue, workspace_id: "ws-1", status: "todo" },
    };
    expect(DuplicateIssueErrorBodySchema.safeParse(forwardCompat).success).toBe(true);
  });

  it("rejects a renamed code (so renames degrade to the generic toast)", () => {
    const renamed = { ...valid, code: "duplicate_issue" };
    expect(DuplicateIssueErrorBodySchema.safeParse(renamed).success).toBe(false);
  });

  it("rejects a missing issue object", () => {
    const { issue: _omit, ...without } = valid;
    expect(DuplicateIssueErrorBodySchema.safeParse(without).success).toBe(false);
  });

  it("rejects a non-string issue.id", () => {
    const broken = { ...valid, issue: { ...valid.issue, id: 42 } };
    expect(DuplicateIssueErrorBodySchema.safeParse(broken).success).toBe(false);
  });

  it("accepts a missing error field (it is optional)", () => {
    const { error: _omit, ...without } = valid;
    expect(DuplicateIssueErrorBodySchema.safeParse(without).success).toBe(true);
  });
});

// `user.timezone` (Viewing tz) was added in the timezone-architecture RFC.
// A desktop build older than the server — or a server predating the
// `user.timezone` migration — will return a `/api/me` body with no
// `timezone` key. The schema must not fail closed on that: the field
// defaults to `null`, which the frontend resolves to the browser-detected
// tz at render time.
describe("UserSchema timezone drift", () => {
  const base = {
    id: "11111111-1111-1111-1111-111111111111",
    name: "Ada",
    email: "ada@example.com",
  };

  it("defaults timezone to null when the field is absent", () => {
    const parsed = UserSchema.parse(base);
    expect(parsed.timezone).toBe(null);
  });

  it("preserves an explicit IANA timezone", () => {
    const parsed = UserSchema.parse({ ...base, timezone: "Asia/Tokyo" });
    expect(parsed.timezone).toBe("Asia/Tokyo");
  });

  it("accepts an explicit null timezone", () => {
    const parsed = UserSchema.parse({ ...base, timezone: null });
    expect(parsed.timezone).toBe(null);
  });

  // Wrong-type drift: a future server bug sending `timezone` as a number
  // must not throw into the UI. parseWithFallback degrades the whole user
  // object to the explicit fallback (EMPTY_USER) so /api/me callers keep a
  // valid shape instead of white-screening.
  it("falls back to EMPTY_USER when timezone is the wrong type", () => {
    const parsed = parseWithFallback(
      { ...base, timezone: 42 },
      UserSchema,
      EMPTY_USER,
      { endpoint: "GET /api/me" },
    );
    expect(parsed).toBe(EMPTY_USER);
  });
});

describe("SquadListSchema member preview drift", () => {
  const baseSquad = {
    id: "squad-1",
    workspace_id: "ws-1",
    name: "Frontend Squad",
    description: "",
    instructions: "",
    avatar_url: null,
    leader_id: "agent-1",
    creator_id: "user-1",
    created_at: "2026-05-01T00:00:00Z",
    updated_at: "2026-05-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
  };

  it("defaults preview fields when an older backend omits them", () => {
    const parsed = SquadListSchema.parse([baseSquad]);
    expect(parsed[0]?.member_count).toBe(0);
    expect(parsed[0]?.member_preview).toEqual([]);
  });

  it("defaults preview fields on a single squad response", () => {
    const parsed = SquadSchema.parse(baseSquad);
    expect(parsed.member_count).toBe(0);
    expect(parsed.member_preview).toEqual([]);
  });

  it("preserves lightweight member preview rows", () => {
    const parsed = SquadListSchema.parse([
      {
        ...baseSquad,
        member_count: 2,
        member_preview: [
          { member_type: "agent", member_id: "agent-1", role: "leader" },
          { member_type: "member", member_id: "user-2", role: "member" },
        ],
      },
    ]);
    expect(parsed[0]?.member_count).toBe(2);
    expect(parsed[0]?.member_preview).toHaveLength(2);
    expect(parsed[0]?.member_preview?.[0]?.role).toBe("leader");
  });
});

// The workspace dashboard and runtime-detail pages were re-pointed at the
// unified `task_usage_hourly` rollup. Every numeric field drives chart /
// KPI math, and string keys (date / agent_id / model) bucket the series.
// The contract these schemas must hold: a row missing a field degrades
// that field to a sane default rather than dropping the WHOLE array to
// the `[]` fallback — one drifted row must not blank the entire chart.
describe("dashboard + runtime usage schema drift", () => {
  it("coerces a missing numeric field to 0 instead of dropping the array", () => {
    const parsed = DashboardUsageDailyListSchema.parse([
      { date: "2026-05-19", model: "claude-opus-4-7", input_tokens: 100 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.output_tokens).toBe(0);
    expect(parsed[0]?.cache_read_tokens).toBe(0);
    expect(parsed[0]?.cache_write_tokens).toBe(0);
  });

  it("coerces a missing date key to \"\" so the rest of the series survives", () => {
    const parsed = DashboardUsageDailyListSchema.parse([
      { model: "claude-opus-4-7", input_tokens: 5 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.date).toBe("");
  });

  it("coerces a missing agent_id key to \"\" for the agent-runtime panel", () => {
    const parsed = DashboardAgentRunTimeListSchema.parse([
      { total_seconds: 42, task_count: 3, failed_count: 0 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.agent_id).toBe("");
  });

  it("coerces a missing agent_id key to \"\" for the usage-by-agent panel", () => {
    const parsed = DashboardUsageByAgentListSchema.parse([
      { model: "claude-opus-4-7", input_tokens: 7 },
    ]);
    expect(parsed[0]?.agent_id).toBe("");
  });

  it("coerces missing fields on every runtime usage schema", () => {
    expect(RuntimeUsageListSchema.parse([{ date: "2026-05-19" }])[0]?.input_tokens).toBe(0);
    expect(RuntimeHourlyActivityListSchema.parse([{ hour: 9 }])[0]?.count).toBe(0);
    expect(RuntimeUsageByAgentListSchema.parse([{ model: "x" }])[0]?.agent_id).toBe("");
    expect(RuntimeUsageByHourListSchema.parse([{ hour: 9 }])[0]?.model).toBe("");
  });

  it("rejects a non-array body so parseWithFallback can return its fallback", () => {
    expect(DashboardUsageDailyListSchema.safeParse(null).success).toBe(false);
    expect(RuntimeUsageListSchema.safeParse({ rows: [] }).success).toBe(false);
  });

  it("keeps unknown server-side fields via .loose()", () => {
    const parsed = RuntimeUsageListSchema.parse([
      { date: "2026-05-19", region: "us-east" },
    ]);
    expect((parsed[0] as Record<string, unknown>).region).toBe("us-east");
  });
});
