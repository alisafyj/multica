import type { DesignRestoreTaskItemInput, GalleryNativeJson } from "@multica/core/types";

export type GroupedFrame = GalleryNativeJson["frames"][number] & { board?: { x?: number; y?: number; order?: number } };

export type FrameTreeNode =
  | { kind: "frame"; frame: GroupedFrame }
  | { kind: "group"; id: string; name: string; path: string[]; frames: GroupedFrame[] };

function stringValue(value: unknown) {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function stringPath(value: unknown) {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && item.trim().length > 0) : [];
}

export function frameGroupPath(frame: GroupedFrame) {
  const source = frame.source ?? {};
  const path = stringPath(source.groupPath);
  const name = stringValue(source.groupName);
  if (path.length > 0) return path;
  return name ? [name] : [];
}

export function frameGroupLabel(frame: GroupedFrame) {
  const path = frameGroupPath(frame);
  return path[path.length - 1] ?? null;
}

function frameGroupId(frame: GroupedFrame) {
  const source = frame.source ?? {};
  const sourceId = stringValue(source.groupId);
  if (sourceId) return sourceId;
  const path = frameGroupPath(frame);
  return path.length > 0 ? path.join("/") : null;
}

function frameMatches(frame: GroupedFrame, query: string) {
  if (!query) return true;
  return frame.name.toLowerCase().includes(query);
}

function groupMatches(name: string, path: string[], query: string) {
  if (!query) return true;
  const haystack = [name, ...path].join(" ").toLowerCase();
  return haystack.includes(query);
}

export function buildFrameTree(frames: GroupedFrame[], query: string): FrameTreeNode[] {
  const normalizedQuery = query.trim().toLowerCase();
  const nodes: FrameTreeNode[] = [];
  const groups = new Map<string, Extract<FrameTreeNode, { kind: "group" }>>();

  for (const frame of frames) {
    const groupId = frameGroupId(frame);
    const groupLabel = frameGroupLabel(frame);
    if (!groupId || !groupLabel) {
      if (frameMatches(frame, normalizedQuery)) nodes.push({ kind: "frame", frame });
      continue;
    }

    let group = groups.get(groupId);
    if (!group) {
      group = { kind: "group", id: groupId, name: groupLabel, path: frameGroupPath(frame), frames: [] };
      groups.set(groupId, group);
      nodes.push(group);
    }
    if (frameMatches(frame, normalizedQuery) || groupMatches(group.name, group.path, normalizedQuery)) {
      group.frames.push(frame);
    }
  }

  return nodes.filter((node) => node.kind === "frame" || node.frames.length > 0);
}

export function groupedFrames(frames: GroupedFrame[]) {
  return buildFrameTree(frames, "").filter((node): node is Extract<FrameTreeNode, { kind: "group" }> => node.kind === "group");
}

export function restoreTaskItemsForFrames(
  frames: GroupedFrame[],
  options: { designFileId: string; revisionId: string; notePrefix: string },
): DesignRestoreTaskItemInput[] {
  return frames.map((frame, index) => {
    const groupLabel = frameGroupLabel(frame);
    return {
      itemId: `frame-${index + 1}`,
      order: index + 1,
      designFileId: options.designFileId,
      revisionId: options.revisionId,
      frameId: frame.id,
      frameName: frame.name,
      source: "frame",
      note: groupLabel
        ? `${options.notePrefix}：来自 Figma 分组 ${groupLabel}，请作为同一组页面/状态一起理解。`
        : `${options.notePrefix}：按 frame 提供给前端工程师或 Agent 获取上下文。`,
    };
  });
}
