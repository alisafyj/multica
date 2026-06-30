import { describe, expect, it } from "vitest";
import { overlayRevealStyle } from "./overlay-comparison";

describe("overlayRevealStyle", () => {
  it("clips the reference image instead of making it translucent", () => {
    expect(overlayRevealStyle(42)).toEqual({
      clipPath: "inset(0 58% 0 0)",
    });
  });

  it("clamps reveal percent to a valid range", () => {
    expect(overlayRevealStyle(-10).clipPath).toBe("inset(0 100% 0 0)");
    expect(overlayRevealStyle(120).clipPath).toBe("inset(0 0% 0 0)");
  });
});
