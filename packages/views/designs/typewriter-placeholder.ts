"use client";

import { useEffect, useState } from "react";

// Design-home composer placeholder typewriter. The empty design-mode composer
// rotates example briefs through its placeholder with a type → hold → delete
// → next cycle, following Open Design's home-hero carousel (their
// apps/web/src/components/home-hero/placeholderScenarios.ts). Two deliberate
// differences from upstream: OD overlays a div on a Lexical editor so it can
// draw a blinking caret span, while we drive the textarea's native
// `placeholder` attribute (nothing to pixel-align, nothing intercepting
// pointer events); and OD's scenarios bind a create template to
// submit-on-empty, which our composer does not offer, so the rotation is
// purely suggestive — submitting still requires a typed brief.

/** Example briefs the empty design composer types through. One line each —
 *  anything taller than the textarea's first row reads as lost text, not a
 *  hint. The canonical matrix for the machine lives in
 *  typewriter-placeholder.test.ts. */
export const PLACEHOLDER_BRIEF_EXAMPLES: ReadonlyArray<string> = [
  "例如：做一个 CRM 客户列表页，支持筛选和批量操作",
  "例如：设计一个销售数据看板，展示漏斗与转化率",
  "例如：画一个移动端订单流程的三屏原型",
  "例如：做一个产品落地页，突出核心功能与定价",
  "例如：设计一个团队设置页，含成员与权限管理",
  "例如：做一个任务看板，支持拖拽切换状态",
  "例如：画一个注册登录流程的线框图",
];

export type TypewriterPhase = "typing" | "holding" | "deleting" | "pausing";

export interface TypewriterState {
  /** Index into the scenario list. */
  index: number;
  /** Number of visible characters of the current scenario's text. */
  charCount: number;
  phase: TypewriterPhase;
}

export interface TypewriterTiming {
  /** Per-character delay while typing. */
  typeMs: number;
  /** Per-character delay while deleting (faster than typing reads as decisive). */
  deleteMs: number;
  /** Dwell once a line is fully typed. */
  holdMs: number;
  /** Gap after a line is fully deleted, before the next line starts. */
  pauseMs: number;
}

export const DEFAULT_TYPEWRITER_TIMING: TypewriterTiming = {
  typeMs: 42,
  deleteMs: 22,
  holdMs: 1900,
  pauseMs: 320,
};

export function initialTypewriterState(): TypewriterState {
  return { index: 0, charCount: 0, phase: "typing" };
}

/** Advance the machine one step and report how long to wait before applying
 *  it. `length` is the current scenario's text length; `count` is the
 *  scenario count (for wraparound). With `reducedMotion`, the per-character
 *  animation collapses to whole-line swaps held for `holdMs` (the caller
 *  renders the full text rather than a slice). */
export function advanceTypewriter(
  state: TypewriterState,
  length: number,
  count: number,
  timing: TypewriterTiming,
  reducedMotion: boolean,
): { state: TypewriterState; delayMs: number } {
  if (count <= 0) return { state, delayMs: timing.holdMs };
  if (reducedMotion) {
    // Hold the current line, then jump straight to the next one.
    return {
      state: { index: (state.index + 1) % count, charCount: length, phase: "holding" },
      delayMs: timing.holdMs,
    };
  }
  switch (state.phase) {
    case "typing":
      if (state.charCount < length) {
        return { state: { ...state, charCount: state.charCount + 1 }, delayMs: timing.typeMs };
      }
      return { state: { ...state, phase: "holding" }, delayMs: timing.holdMs };
    case "holding":
      return { state: { ...state, phase: "deleting" }, delayMs: timing.deleteMs };
    case "deleting":
      if (state.charCount > 0) {
        return { state: { ...state, charCount: state.charCount - 1 }, delayMs: timing.deleteMs };
      }
      return { state: { ...state, phase: "pausing" }, delayMs: timing.pauseMs };
    case "pausing":
    default:
      return {
        state: { index: (state.index + 1) % count, charCount: 0, phase: "typing" },
        delayMs: timing.typeMs,
      };
  }
}

// Reports the OS "reduce motion" preference, live. SSR/jsdom without
// matchMedia falls back to false (animate) — the animation is purely
// decorative, so a missing matcher must not crash the composer.
function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(query.matches);
    const onChange = (event: MediaQueryListEvent) => setReduced(event.matches);
    // addEventListener is the modern API; addListener is the Safari<14 fallback.
    if (typeof query.addEventListener === "function") {
      query.addEventListener("change", onChange);
      return () => query.removeEventListener("change", onChange);
    }
    query.addListener(onChange);
    return () => query.removeListener(onChange);
  }, []);
  return reduced;
}

/** Type the rotating examples into a placeholder string. `enabled` is the
 *  composer's "design mode and the brief is empty"; `paused` freezes the
 *  rotation (a focused composer) and shows the current example in full, so a
 *  caret about to type never sits over moving or half-deleted text. Returns
 *  "" while disabled — the caller falls back to its static placeholder. */
export function useTypewriterPlaceholder(
  scenarios: ReadonlyArray<string>,
  { enabled, paused = false }: { enabled: boolean; paused?: boolean },
): string {
  const reducedMotion = usePrefersReducedMotion();
  const [state, setState] = useState(initialTypewriterState);

  // Restart from the first example whenever the rotation switches off, so a
  // user who types then clears the brief sees the cycle start fresh instead
  // of resuming mid-delete.
  useEffect(() => {
    if (!enabled) setState(initialTypewriterState());
  }, [enabled]);

  // Drive the machine: each committed state schedules the next step. Changing
  // state re-runs this effect, whose cleanup clears the prior timer — so the
  // chain self-sustains without overlapping timers (StrictMode-safe).
  useEffect(() => {
    if (!enabled || paused || scenarios.length === 0) return;
    const length = scenarios[state.index % scenarios.length]?.length ?? 0;
    const { state: nextState, delayMs } = advanceTypewriter(
      state,
      length,
      scenarios.length,
      DEFAULT_TYPEWRITER_TIMING,
      reducedMotion,
    );
    const timer = window.setTimeout(() => setState(nextState), Math.max(16, delayMs));
    return () => window.clearTimeout(timer);
  }, [enabled, paused, state, scenarios, reducedMotion]);

  if (!enabled || scenarios.length === 0) return "";
  const text = scenarios[state.index % scenarios.length] ?? "";
  // Paused and reduced motion both render the whole line: a frozen
  // half-deleted fragment reads like broken copy.
  return paused || reducedMotion ? text : text.slice(0, state.charCount);
}
