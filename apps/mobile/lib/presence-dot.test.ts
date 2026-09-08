// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import { PresenceDot } from "@/components/ui/presence-dot";

vi.mock("react-native", () => ({ View: "View" }));

describe("PresenceDot", () => {
  it("renders unknown availability with the neutral dot treatment", () => {
    const dot = PresenceDot({ availability: "unknown" });

    expect(dot.props.className).toContain("bg-muted-foreground/40");
    expect(dot.props.className).not.toContain("bg-success");
    expect(dot.props.className).not.toContain("bg-warning");
  });
});
