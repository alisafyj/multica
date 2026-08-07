import { describe, expect, it } from "vitest";
import type { IssuePriority, IssueStatus } from "@multica/core/types";
// Relative import, not the "@/" tsconfig alias: vitest.config.ts resolves
// "@" but lib/i18n/parity.test.ts documents that the alias is unreliable
// here, so every lib test uses relative paths.
import { RESOURCES } from "../locales";
import {
  BOARD_STATUSES,
  type IssueTranslate,
  issuePriorityLabel,
  issueStatusLabel,
} from "./issue-status";

/**
 * Enum coverage guard. Replaces the exhaustiveness that the old
 * `Record<IssueStatus, string>` maps gave TypeScript: now that labels resolve
 * by string key, only a test can catch "web added a status and mobile's
 * locale JSON never got it". Stronger than the type check ever was — it
 * also holds zh-Hans to the same bar.
 *
 * lib/i18n/parity.test.ts is the complementary half: it proves en and
 * zh-Hans agree with each other, but not that either covers the enum.
 */
const ALL_STATUSES: IssueStatus[] = [...BOARD_STATUSES, "cancelled"];

const ALL_PRIORITIES: IssuePriority[] = [
  "urgent",
  "high",
  "medium",
  "low",
  "none",
];

const LOCALES = ["en", "zh-Hans"] as const;

function issuesBundle(locale: (typeof LOCALES)[number]) {
  return RESOURCES[locale].issues as unknown as {
    status?: Record<string, string>;
    priority?: Record<string, string>;
  };
}

/** Minimal stand-in for i18next: dictionary hit, else `defaultValue`. */
function makeT(dict: Record<string, string> = {}): IssueTranslate {
  return (key, opts) =>
    dict[key] ?? ((opts?.defaultValue as string | undefined) ?? key);
}

describe("issue status / priority label coverage", () => {
  for (const locale of LOCALES) {
    it(`${locale}: every issue status has a label`, () => {
      const status = issuesBundle(locale).status ?? {};
      const missing = ALL_STATUSES.filter((s) => !status[s]);
      expect(missing).toEqual([]);
    });

    it(`${locale}: every issue priority has a label`, () => {
      const priority = issuesBundle(locale).priority ?? {};
      const missing = ALL_PRIORITIES.filter((p) => !priority[p]);
      expect(missing).toEqual([]);
    });
  }

  it("BOARD_STATUSES excludes cancelled, matching web's board", () => {
    expect(BOARD_STATUSES).not.toContain("cancelled");
  });
});

describe("issueStatusLabel / issuePriorityLabel", () => {
  it("resolves a known status through the issues namespace", () => {
    const t = makeT({ "status.in_progress": "进行中" });
    expect(issueStatusLabel(t, "in_progress")).toBe("进行中");
  });

  it("resolves a known priority through the issues namespace", () => {
    const t = makeT({ "priority.none": "No priority" });
    expect(issuePriorityLabel(t, "none")).toBe("No priority");
  });

  // Enum drift downgrades, not crashes (root CLAUDE.md "API Compatibility"):
  // a status this build has never heard of must still render readably.
  it("falls back to the raw value for an unknown status", () => {
    expect(issueStatusLabel(makeT(), "awaiting_triage")).toBe(
      "awaiting_triage",
    );
  });

  it("falls back to the raw value for an unknown priority", () => {
    expect(issuePriorityLabel(makeT(), "catastrophic")).toBe("catastrophic");
  });
});
