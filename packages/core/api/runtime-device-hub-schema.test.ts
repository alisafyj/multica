// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseWithFallback } from "./schema";
import { EMPTY_RUNTIME_DEVICE_HUB, RuntimeDeviceHubSchema, TestRunSchema } from "./schemas";

// Boundary tests for the M4 shapes: a device-hub report from an older or
// newer daemon, and a run row with or without the parallelism cap.

describe("RuntimeDeviceHubSchema", () => {
  it("fills defaults for a minimal report and passes unknown fields through", () => {
    const parsed = RuntimeDeviceHubSchema.parse({ reachable: true, phones: 2, future_field: "x" });
    expect(parsed.reachable).toBe(true);
    expect(parsed.phones).toBe(2);
    expect(parsed.pairing_url).toBeNull();
    expect(parsed.reported_at).toBeNull();
    expect((parsed as Record<string, unknown>).future_field).toBe("x");
  });

  it("falls back to the empty hub on a malformed response", () => {
    const parsed = parseWithFallback({ reachable: "yes", phones: -1 }, RuntimeDeviceHubSchema, EMPTY_RUNTIME_DEVICE_HUB, {
      endpoint: "GET /api/runtimes/{id}/device-hub",
    });
    expect(parsed).toEqual(EMPTY_RUNTIME_DEVICE_HUB);
  });
});

describe("TestRunSchema.parallelism", () => {
  it("defaults to null when the backend predates the cap and keeps a positive cap", () => {
    expect(TestRunSchema.parse({ id: "r1" }).parallelism).toBeNull();
    expect(TestRunSchema.parse({ id: "r1", parallelism: 3 }).parallelism).toBe(3);
  });

  it("rejects a non-positive cap so the run page never shows a nonsense value", () => {
    expect(TestRunSchema.safeParse({ id: "r1", parallelism: 0 }).success).toBe(false);
  });
});
