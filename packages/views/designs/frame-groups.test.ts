import { describe, expect, it } from "vitest";
import { buildFrameTree, frameGroupLabel, restoreTaskItemsForFrames } from "./frame-groups";
import type { GalleryNativeJson } from "@multica/core/types";

type TestFrame = GalleryNativeJson["frames"][number];

function frame(input: Partial<TestFrame> & Pick<TestFrame, "id" | "name">): TestFrame {
  return {
    rootLayerId: `${input.id}-root`,
    width: 390,
    height: 844,
    ...input,
  };
}

describe("buildFrameTree", () => {
  it("groups frames by Figma source group while keeping ungrouped frames flat", () => {
    const frames = [
      frame({ id: "frame-1", name: "列表", source: { groupId: "group-43", groupName: "Group 43", groupPath: ["Group 43"] } }),
      frame({ id: "frame-2", name: "详情", source: { groupId: "group-43", groupName: "Group 43", groupPath: ["Group 43"] } }),
      frame({ id: "frame-3", name: "空状态" }),
    ];

    const tree = buildFrameTree(frames, "");

    expect(tree).toMatchObject([
      { kind: "group", id: "group-43", name: "Group 43", frames: [{ id: "frame-1" }, { id: "frame-2" }] },
      { kind: "frame", frame: { id: "frame-3" } },
    ]);
  });

  it("keeps the group visible when a query matches only one child frame", () => {
    const frames = [
      frame({ id: "frame-1", name: "服务记录列表", source: { groupId: "group-43", groupName: "服务记录", groupPath: ["服务记录"] } }),
      frame({ id: "frame-2", name: "服务记录详情", source: { groupId: "group-43", groupName: "服务记录", groupPath: ["服务记录"] } }),
      frame({ id: "frame-3", name: "客户列表" }),
    ];

    const tree = buildFrameTree(frames, "详情");

    expect(tree).toMatchObject([
      { kind: "group", name: "服务记录", frames: [{ id: "frame-2" }] },
    ]);
  });

  it("uses frame-level group name when grouped restore hints are unavailable", () => {
    const testFrame = frame({ id: "frame-1", name: "列表", source: { groupName: "Group 43" } });

    expect(frameGroupLabel(testFrame)).toBe("Group 43");
  });
});

describe("restoreTaskItemsForFrames", () => {
  it("adds Figma group context to grouped frame restore items", () => {
    const frames = [
      frame({ id: "frame-1", name: "列表", source: { groupName: "Group 43", groupPath: ["Group 43"] } }),
      frame({ id: "frame-2", name: "详情", source: { groupName: "Group 43", groupPath: ["Group 43"] } }),
    ];

    const items = restoreTaskItemsForFrames(frames, {
      designFileId: "design-1",
      revisionId: "revision-1",
      notePrefix: "完整设计稿就绪任务",
    });

    expect(items).toEqual([
      expect.objectContaining({
        frameId: "frame-1",
        frameName: "列表",
        note: "完整设计稿就绪任务：来自 Figma 分组 Group 43，请作为同一组页面/状态一起理解。",
      }),
      expect.objectContaining({
        frameId: "frame-2",
        frameName: "详情",
        note: "完整设计稿就绪任务：来自 Figma 分组 Group 43，请作为同一组页面/状态一起理解。",
      }),
    ]);
  });
});
