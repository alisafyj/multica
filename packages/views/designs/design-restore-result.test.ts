import { describe, expect, it } from "vitest";
import { readDesignRestoreVisualReview } from "./design-restore-result";

describe("readDesignRestoreVisualReview", () => {
  it("normalizes visual QA evidence from agent result summary", () => {
    const review = readDesignRestoreVisualReview({
      visualFidelityScore: 87,
      visualReview: {
        implementedRoute: "/service-record",
        designScreenshot: "/tmp/design.png",
        implementationScreenshot: "/tmp/impl.png",
        comparisonScreenshot: "/tmp/compare.png",
        remainingDiffs: ["头图裁切仍有轻微差异", "第一位治疗师头像不完全一致"],
        notes: "二轮视觉 QA 后可验收",
      },
    });

    expect(review).toEqual({
      score: 87,
      implementedRoute: "/service-record",
      designScreenshot: "/tmp/design.png",
      implementationScreenshot: "/tmp/impl.png",
      comparisonScreenshot: "/tmp/compare.png",
      remainingDiffs: ["头图裁切仍有轻微差异", "第一位治疗师头像不完全一致"],
      notes: "二轮视觉 QA 后可验收",
    });
  });

  it("falls back to root-level screenshot fields for older agent summaries", () => {
    const review = readDesignRestoreVisualReview({
      visualFidelityScore: "91",
      implementedRoute: "/records",
      designScreenshot: "/tmp/source.png",
      implementationScreenshot: "/tmp/page.png",
      comparisonScreenshot: "/tmp/side-by-side.png",
      remainingDiffs: ["收藏图标略大"],
    });

    expect(review?.score).toBe(91);
    expect(review?.implementedRoute).toBe("/records");
    expect(review?.comparisonScreenshot).toBe("/tmp/side-by-side.png");
    expect(review?.remainingDiffs).toEqual(["收藏图标略大"]);
  });

  it("returns null when no visual QA evidence exists", () => {
    expect(readDesignRestoreVisualReview({ status: "completed" })).toBeNull();
  });
});
