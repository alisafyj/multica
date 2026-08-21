// @vitest-environment node

// Canonical matrix for the placeholder typewriter state machine. The composer
// suite only covers the wiring (typing/freezing through the textarea), not
// this matrix.
import { describe, expect, it } from "vitest";
import {
  advanceTypewriter,
  DEFAULT_TYPEWRITER_TIMING,
  initialTypewriterState,
  PLACEHOLDER_BRIEF_EXAMPLES,
} from "./typewriter-placeholder";

const T = DEFAULT_TYPEWRITER_TIMING;

describe("advanceTypewriter", () => {
  it("types one character per step and rolls into holding at full length", () => {
    let state = initialTypewriterState();

    const first = advanceTypewriter(state, 5, 3, T, false);
    expect(first.state).toEqual({ index: 0, charCount: 1, phase: "typing" });
    expect(first.delayMs).toBe(T.typeMs);

    state = { index: 0, charCount: 4, phase: "typing" };
    expect(advanceTypewriter(state, 5, 3, T, false)).toEqual({
      state: { index: 0, charCount: 5, phase: "typing" },
      delayMs: T.typeMs,
    });
    // Full length: the next step holds instead of over-typing.
    state = { index: 0, charCount: 5, phase: "typing" };
    expect(advanceTypewriter(state, 5, 3, T, false)).toEqual({
      state: { index: 0, charCount: 5, phase: "holding" },
      delayMs: T.holdMs,
    });
  });

  it("holds, then deletes faster than typing, then pauses before the next line", () => {
    expect(advanceTypewriter({ index: 0, charCount: 5, phase: "holding" }, 5, 3, T, false)).toEqual({
      state: { index: 0, charCount: 5, phase: "deleting" },
      delayMs: T.deleteMs,
    });
    expect(advanceTypewriter({ index: 0, charCount: 3, phase: "deleting" }, 5, 3, T, false)).toEqual({
      state: { index: 0, charCount: 2, phase: "deleting" },
      delayMs: T.deleteMs,
    });
    // Fully deleted: pause, do not underflow.
    expect(advanceTypewriter({ index: 0, charCount: 0, phase: "deleting" }, 5, 3, T, false)).toEqual({
      state: { index: 0, charCount: 0, phase: "pausing" },
      delayMs: T.pauseMs,
    });
  });

  it("starts the next line after the pause and wraps around the list", () => {
    expect(advanceTypewriter({ index: 0, charCount: 0, phase: "pausing" }, 5, 3, T, false)).toEqual({
      state: { index: 1, charCount: 0, phase: "typing" },
      delayMs: T.typeMs,
    });
    expect(advanceTypewriter({ index: 2, charCount: 0, phase: "pausing" }, 5, 3, T, false)).toEqual({
      state: { index: 0, charCount: 0, phase: "typing" },
      delayMs: T.typeMs,
    });
  });

  it("collapses to whole-line swaps under reduced motion", () => {
    expect(advanceTypewriter(initialTypewriterState(), 5, 3, T, true)).toEqual({
      state: { index: 1, charCount: 5, phase: "holding" },
      delayMs: T.holdMs,
    });
    expect(advanceTypewriter({ index: 2, charCount: 5, phase: "holding" }, 5, 3, T, true)).toEqual({
      state: { index: 0, charCount: 5, phase: "holding" },
      delayMs: T.holdMs,
    });
  });

  it("leaves the state untouched when there is nothing to rotate", () => {
    const state = initialTypewriterState();
    expect(advanceTypewriter(state, 0, 0, T, false)).toEqual({ state, delayMs: T.holdMs });
    expect(advanceTypewriter(state, 0, 0, T, true)).toEqual({ state, delayMs: T.holdMs });
  });
});

describe("PLACEHOLDER_BRIEF_EXAMPLES", () => {
  it("keeps every example a single short line in the composer voice", () => {
    expect(PLACEHOLDER_BRIEF_EXAMPLES.length).toBeGreaterThan(1);
    for (const text of PLACEHOLDER_BRIEF_EXAMPLES) {
      expect(text.startsWith("例如：")).toBe(true);
      // One textarea row: the rotation must read as a hint, not lost text.
      expect(text.length).toBeLessThanOrEqual(30);
    }
    expect(new Set(PLACEHOLDER_BRIEF_EXAMPLES).size).toBe(PLACEHOLDER_BRIEF_EXAMPLES.length);
  });
});
