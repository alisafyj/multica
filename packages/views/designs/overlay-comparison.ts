export function overlayRevealStyle(percent: number) {
  const revealPercent = Math.min(100, Math.max(0, Math.round(percent)));
  return { clipPath: `inset(0 ${100 - revealPercent}% 0 0)` };
}
