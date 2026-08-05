import { describe, expect, it } from "vitest";
import { testCaseKeys } from "./keys";

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
