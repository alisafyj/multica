// @vitest-environment node
import { describe, expect, it } from "vitest";
import { builtinDesignSystemHost, builtinDesignSystemLogoURL } from "./design-system-domains";

// The table is ported verbatim from Open Design's OFFICIAL_PRESET_DOMAINS:
// every host must be a bare hostname (the caller builds the URL), and the
// lookups cover the shapes the catalogue actually uses.
describe("builtin design system domains", () => {
  it("resolves curated slugs to bare hosts", () => {
    expect(builtinDesignSystemHost("apple")).toBe("apple.com");
    expect(builtinDesignSystemHost("stripe")).toBe("stripe.com");
    expect(builtinDesignSystemHost("linear-app")).toBe("linear.app");
    expect(builtinDesignSystemHost("shadcn")).toBe("ui.shadcn.com");
  });

  it("builds the favicon URL OD renders, or nothing for an unknown slug", () => {
    expect(builtinDesignSystemLogoURL("apple")).toBe(
      "https://www.google.com/s2/favicons?domain=apple.com&sz=64",
    );
    expect(builtinDesignSystemLogoURL("no-such-system")).toBe("");
    expect(builtinDesignSystemLogoURL("")).toBe("");
  });
});
