import { describe, expect, it } from "vitest";
import type { DesignFrame } from "@multica/core/types";
import {
  createDesignRestoreMCPPrompt,
  createFigmaGroupRestoreScope,
  createFrameRestoreScope,
  createSelectionRestoreScope,
} from "./design-restore-scope";
import type { FrameTreeNode } from "./frame-groups";

function frame(input: Partial<DesignFrame> & Pick<DesignFrame, "id" | "name">): DesignFrame {
  return {
    rootLayerId: `${input.id}-root`,
    width: 390,
    height: 844,
    ...input,
  };
}

describe("design restore MCP scopes", () => {
  it("creates a frame scope for a single frame", () => {
    const scope = createFrameRestoreScope({
      designFileId: "design-1",
      revisionId: "revision-1",
      frame: frame({ id: "frame-1", name: "钱包首页-已绑支付宝" }),
      sourcePageUrl: "http://localhost:3031/amc/designs/design-1/frames/frame-1",
    });

    expect(scope).toEqual({
      version: "1.0",
      kind: "frame",
      designFileId: "design-1",
      revisionId: "revision-1",
      frameId: "frame-1",
      label: "钱包首页-已绑支付宝",
      sourcePageUrl: "http://localhost:3031/amc/designs/design-1/frames/frame-1",
    });
  });

  it("creates a Figma group scope with ordered child frames", () => {
    const group = {
      kind: "group",
      id: "group-wallet",
      name: "钱包首页",
      path: ["钱包首页"],
      frames: [
        frame({ id: "frame-1", name: "钱包首页-已绑支付宝" }),
        frame({ id: "frame-2", name: "钱包首页-未绑支付宝" }),
      ],
    } satisfies Extract<FrameTreeNode, { kind: "group" }>;

    const scope = createFigmaGroupRestoreScope({
      designFileId: "design-1",
      revisionId: "revision-1",
      group,
      sourcePageUrl: "http://localhost:3031/amc/designs/design-1",
    });

    expect(scope).toEqual({
      version: "1.0",
      kind: "figma_group",
      designFileId: "design-1",
      revisionId: "revision-1",
      groupId: "group-wallet",
      groupName: "钱包首页",
      groupPath: ["钱包首页"],
      frameIds: ["frame-1", "frame-2"],
      label: "钱包首页",
      sourcePageUrl: "http://localhost:3031/amc/designs/design-1",
    });
  });

  it("creates a selected layer scope without root layer noise", () => {
    const scope = createSelectionRestoreScope({
      designFileId: "design-1",
      revisionId: "revision-1",
      frame: frame({ id: "frame-1", name: "提现-弹窗:确认提现", rootLayerId: "root" }),
      layerIds: ["root", "amount-input", "amount-input", "submit-button"],
    });

    expect(scope).toMatchObject({
      version: "1.0",
      kind: "selected_layers",
      designFileId: "design-1",
      revisionId: "revision-1",
      frameId: "frame-1",
      layerIds: ["amount-input", "submit-button"],
      label: "提现-弹窗:确认提现 · 选中图层",
    });
  });

  it("creates a selection bounds scope for marquee restore", () => {
    const scope = createSelectionRestoreScope({
      designFileId: "design-1",
      revisionId: "revision-1",
      frame: frame({ id: "frame-1", name: "提现-弹窗:确认提现" }),
      layerIds: [],
      selectionBounds: { x: 24, y: 120, width: 320, height: 180 },
      sourcePageUrl: "http://localhost:3031/amc/designs/design-1/frames/frame-1",
    });

    expect(scope).toEqual({
      version: "1.0",
      kind: "selection_bounds",
      designFileId: "design-1",
      revisionId: "revision-1",
      frameId: "frame-1",
      selectionBounds: { x: 24, y: 120, width: 320, height: 180 },
      includeIntersectingLayers: true,
      label: "提现-弹窗:确认提现 · 框选区域",
      sourcePageUrl: "http://localhost:3031/amc/designs/design-1/frames/frame-1",
    });
  });

  it("creates a concise ready-to-paste MCP restore prompt with the scope embedded", () => {
    const scope = createFigmaGroupRestoreScope({
      designFileId: "design-1",
      revisionId: "revision-1",
      group: {
        kind: "group",
        id: "group-wallet",
        name: "钱包首页",
        path: ["钱包首页"],
        frames: [
          frame({ id: "frame-1", name: "钱包首页-已绑支付宝" }),
          frame({ id: "frame-2", name: "钱包首页-未绑支付宝" }),
        ],
      },
    });

    const prompt = createDesignRestoreMCPPrompt(scope);

    expect(prompt).toContain("multica_design_get_restore_pack");
    expect(prompt).toContain('"detailLevel":"normal"');
    expect(prompt).toContain('"kind":"figma_group"');
    expect(prompt).toContain('"groupName":"钱包首页"');
    expect(prompt).toContain("分组=同一业务页面多状态/弹窗");
    expect(prompt).toContain("按 Restore Pack 实现");
    expect(prompt).not.toContain("要求：");
    expect(prompt.length).toBeLessThan(900);
  });
});
