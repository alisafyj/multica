import { describe, expect, it } from "vitest";
import { testCaseKeys, testPlanKeys, testRunKeys, testCapabilityKeys, testCaseTimelineKeys } from "./keys";

describe("testCaseKeys", () => {
  it("scopes every key by workspace id", () => {
    expect(testCaseKeys.all("ws-1")[1]).toBe("ws-1");
    expect(testCaseKeys.list("ws-1")[1]).toBe("ws-1");
    expect(testCaseKeys.detail("ws-1", "TC-2")[1]).toBe("ws-1");
    expect(testCaseKeys.modules("ws-1", "p1")[1]).toBe("ws-1");
    expect(testCaseKeys.revisions("ws-1", "TC-2")[1]).toBe("ws-1");
  });

  it("derives child keys from the parent key", () => {
    const parent = testCaseKeys.all("ws-1");
    for (const child of [
      testCaseKeys.list("ws-1"),
      testCaseKeys.detail("ws-1", "TC-2"),
      testCaseKeys.modules("ws-1", "p1"),
      testCaseKeys.revisions("ws-1", "TC-2"),
    ]) {
      expect(child.slice(0, 2)).toEqual([...parent]);
    }
  });

  it("separates lists with different filters", () => {
    expect(testCaseKeys.list("ws-1", { projectId: "a" })).not.toEqual(
      testCaseKeys.list("ws-1", { projectId: "b" }),
    );
  });

  it("separates the same case across workspaces", () => {
    expect(testCaseKeys.detail("ws-1", "TC-2")).not.toEqual(
      testCaseKeys.detail("ws-2", "TC-2"),
    );
  });
});

describe("testPlanKeys", () => {
  it("scopes every key by workspace id at index 1", () => {
    expect(testPlanKeys.all("ws-1")[1]).toBe("ws-1");
    expect(testPlanKeys.list("ws-1")[1]).toBe("ws-1");
    expect(testPlanKeys.detail("ws-1", "p-1")[1]).toBe("ws-1");
    expect(testPlanKeys.cases("ws-1", "p-1")[1]).toBe("ws-1");
  });

  it("derives child keys from the parent key", () => {
    const parent = testPlanKeys.all("ws-1");
    for (const child of [
      testPlanKeys.list("ws-1"),
      testPlanKeys.detail("ws-1", "p-1"),
      testPlanKeys.cases("ws-1", "p-1"),
    ]) {
      expect(child.slice(0, 2)).toEqual([...parent]);
    }
  });

  it("separates plans across workspaces", () => {
    expect(testPlanKeys.detail("ws-1", "p-1")).not.toEqual(
      testPlanKeys.detail("ws-2", "p-1"),
    );
  });

  it("separates lists with different filters", () => {
    expect(testPlanKeys.list("ws-1", { projectId: "a" })).not.toEqual(
      testPlanKeys.list("ws-1", { projectId: "b" }),
    );
  });
});

describe("testRunKeys", () => {
  it("scopes every key by workspace id at index 1", () => {
    expect(testRunKeys.all("ws-1")[1]).toBe("ws-1");
    expect(testRunKeys.list("ws-1")[1]).toBe("ws-1");
    expect(testRunKeys.detail("ws-1", "r-1")[1]).toBe("ws-1");
    expect(testRunKeys.cases("ws-1", "r-1")[1]).toBe("ws-1");
  });

  it("derives child keys from the parent key", () => {
    const parent = testRunKeys.all("ws-1");
    for (const child of [
      testRunKeys.list("ws-1"),
      testRunKeys.detail("ws-1", "r-1"),
      testRunKeys.cases("ws-1", "r-1"),
    ]) {
      expect(child.slice(0, 2)).toEqual([...parent]);
    }
  });
});

describe("testCapabilityKeys", () => {
  it("scopes every key by workspace id at index 1", () => {
    expect(testCapabilityKeys.all("ws-1")[1]).toBe("ws-1");
    expect(testCapabilityKeys.list("ws-1")[1]).toBe("ws-1");
  });
});

describe("testCaseTimelineKeys", () => {
  it("scopes by workspace id at index 1", () => {
    expect(testCaseTimelineKeys.timeline("ws-1", "TC-1")[1]).toBe("ws-1");
  });

  it("separates different cases", () => {
    expect(testCaseTimelineKeys.timeline("ws-1", "TC-1")).not.toEqual(
      testCaseTimelineKeys.timeline("ws-1", "TC-2"),
    );
  });
});
