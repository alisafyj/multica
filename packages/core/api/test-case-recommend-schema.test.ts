// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseWithFallback } from "./schema";
import { EMPTY_RECOMMEND_TEST_CASES_RESPONSE, RecommendTestCasesResponseSchema } from "./schemas";

describe("RecommendTestCasesResponseSchema", () => {
  it("parses a recommendation with its matches and keeps unknown fields", () => {
    const parsed = RecommendTestCasesResponseSchema.parse({
      cases: [
        {
          test_case: { id: "c1", key: "TC-1", title: "Checkout", repos: [{ alias: "web", role: "under_test", path_globs: ["src/cart/**"] }] },
          matches: [{ alias: "web", role: "under_test", glob: "src/cart/**", paths: ["src/cart/total.ts"] }],
          path_count: 1,
          extra: true,
        },
      ],
      unmatched_paths: ["docs/readme.md"],
      total: 1,
    });
    expect(parsed.cases[0]?.test_case.key).toBe("TC-1");
    expect(parsed.cases[0]?.matches[0]?.paths).toEqual(["src/cart/total.ts"]);
    expect((parsed.cases[0] as Record<string, unknown>).extra).toBe(true);
    expect(parsed.unmatched_paths).toEqual(["docs/readme.md"]);
  });

  it("falls back to an empty recommendation on a malformed response", () => {
    const parsed = parseWithFallback({ cases: "many", total: "1" }, RecommendTestCasesResponseSchema, EMPTY_RECOMMEND_TEST_CASES_RESPONSE, {
      endpoint: "POST /api/test-cases/recommend",
    });
    expect(parsed).toEqual(EMPTY_RECOMMEND_TEST_CASES_RESPONSE);
    expect(parsed.cases).toEqual([]);
  });
});
