import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DesignRestoreVisualReviewPanel } from "./design-restore-visual-review-panel";

describe("DesignRestoreVisualReviewPanel", () => {
  it("renders visual QA score, screenshots, route, and remaining diffs", () => {
    render(
      <DesignRestoreVisualReviewPanel
        review={{
          score: 87,
          implementedRoute: "/service-record",
          designScreenshot: "/tmp/design.png",
          implementationScreenshot: "/tmp/impl.png",
          comparisonScreenshot: "/tmp/compare.png",
          remainingDiffs: ["头图裁切仍有轻微差异", "第一位治疗师头像不完全一致"],
          notes: "二轮视觉 QA 后可验收",
        }}
      />,
    );

    expect(screen.getByText("视觉验收")).toBeInTheDocument();
    expect(screen.getByText("87/100")).toBeInTheDocument();
    expect(screen.getByText("/service-record")).toBeInTheDocument();
    expect(screen.getByText("/tmp/design.png")).toBeInTheDocument();
    expect(screen.getByText("/tmp/impl.png")).toBeInTheDocument();
    expect(screen.getByText("/tmp/compare.png")).toBeInTheDocument();
    expect(screen.getByText("头图裁切仍有轻微差异")).toBeInTheDocument();
    expect(screen.getByText("第一位治疗师头像不完全一致")).toBeInTheDocument();
    expect(screen.getByText("二轮视觉 QA 后可验收")).toBeInTheDocument();
  });
});
