// @vitest-environment node
import { describe, expect, it } from "vitest";
import { isFlakyHistory } from "./flaky";

// Newest first, as the timeline endpoint returns it.
describe("isFlakyHistory", () => {
  it("flags a history that flips between pass and fail at least twice", () => {
    expect(isFlakyHistory(["passed", "failed", "passed", "passed", "failed"])).toBe(true);
    expect(isFlakyHistory(["failed", "passed", "failed"])).toBe(true);
  });

  it("does not flag a single regression or a steady history", () => {
    expect(isFlakyHistory(["failed", "passed", "passed", "passed", "passed"])).toBe(false);
    expect(isFlakyHistory(["passed", "passed", "passed"])).toBe(false);
    expect(isFlakyHistory(["failed", "failed"])).toBe(false);
  });

  it("ignores blocked, skipped and pending entries and needs three real results", () => {
    expect(isFlakyHistory(["passed", "blocked", "failed", "skipped", "passed"])).toBe(true);
    expect(isFlakyHistory(["passed", "failed"])).toBe(false);
    expect(isFlakyHistory([])).toBe(false);
  });

  it("only looks at the newest window", () => {
    // Old flips outside the window do not count.
    expect(isFlakyHistory(["passed", "passed", "passed", "passed", "passed", "failed", "passed", "failed"])).toBe(false);
  });
});
