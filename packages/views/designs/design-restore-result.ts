export interface DesignRestoreVisualReview {
  score: number | null;
  implementedRoute: string;
  designScreenshot: string;
  implementationScreenshot: string;
  comparisonScreenshot: string;
  remainingDiffs: string[];
  notes: string;
}

function readRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function readString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function readStringList(value: unknown): string[] {
  return Array.isArray(value) ? value.map(readString).filter(Boolean) : [];
}

function readScore(value: unknown): number | null {
  const score = typeof value === "number" ? value : typeof value === "string" ? Number(value) : Number.NaN;
  if (!Number.isFinite(score)) return null;
  return Math.max(0, Math.min(100, Math.round(score)));
}

function firstString(primary: Record<string, unknown>, fallback: Record<string, unknown>, key: string): string {
  return readString(primary[key]) || readString(fallback[key]);
}

export function readDesignRestoreVisualReview(summary: unknown): DesignRestoreVisualReview | null {
  const root = readRecord(summary);
  if (!root) return null;
  const visual = readRecord(root.visualReview) ?? {};
  const review: DesignRestoreVisualReview = {
    score: readScore(visual.visualFidelityScore ?? root.visualFidelityScore),
    implementedRoute: firstString(visual, root, "implementedRoute"),
    designScreenshot: firstString(visual, root, "designScreenshot"),
    implementationScreenshot: firstString(visual, root, "implementationScreenshot"),
    comparisonScreenshot: firstString(visual, root, "comparisonScreenshot"),
    remainingDiffs: readStringList(visual.remainingDiffs ?? root.remainingDiffs),
    notes: firstString(visual, root, "notes"),
  };

  if (
    review.score === null &&
    !review.implementedRoute &&
    !review.designScreenshot &&
    !review.implementationScreenshot &&
    !review.comparisonScreenshot &&
    !review.remainingDiffs.length &&
    !review.notes
  ) {
    return null;
  }
  return review;
}
