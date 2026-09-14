/**
 * "Unstable" marker for a case's cross-round history (09-02 §7.5): the last
 * `window` executed results contain both a pass and a fail and flip between
 * them at least twice. One regression is a bug; a case that keeps changing
 * its mind without the code changing is a test problem, and that is the
 * signal worth a badge.
 */
export const FLAKY_WINDOW = 5;

export function isFlakyHistory(results: readonly string[], window = FLAKY_WINDOW): boolean {
  const recent = results
    .filter((r) => r === "passed" || r === "failed")
    .slice(0, window);
  if (recent.length < 3) return false;
  let flips = 0;
  for (let i = 1; i < recent.length; i++) {
    if (recent[i] !== recent[i - 1]) flips++;
  }
  return flips >= 2;
}
