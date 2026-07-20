import type { DesignFrame } from "@multica/core/types";
import type { FrameTreeNode } from "./frame-groups";

export type DesignRestoreScopeKind = "frame" | "figma_group" | "selected_layers" | "selection_bounds";

export interface DesignSelectionBoundsScope {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface DesignRestoreScopeV1 {
  version: "1.0";
  kind: DesignRestoreScopeKind;
  designFileId: string;
  revisionId: string;
  frameId?: string;
  groupId?: string;
  groupName?: string;
  groupPath?: string[];
  frameIds?: string[];
  layerIds?: string[];
  selectionBounds?: DesignSelectionBoundsScope;
  includeIntersectingLayers?: boolean;
  label?: string;
  sourcePageUrl?: string;
}

type GroupNode = Extract<FrameTreeNode, { kind: "group" }>;

function withSourcePageUrl<T extends DesignRestoreScopeV1>(scope: T, sourcePageUrl?: string | null): T {
  const value = sourcePageUrl?.trim();
  return value ? { ...scope, sourcePageUrl: value } : scope;
}

function uniqueLayerIds(layerIds: string[], rootLayerId?: string) {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const rawId of layerIds) {
    const id = rawId.trim();
    if (!id || id === rootLayerId || seen.has(id)) continue;
    seen.add(id);
    out.push(id);
  }
  return out;
}

export function createFrameRestoreScope(input: {
  designFileId: string;
  revisionId: string;
  frame: Pick<DesignFrame, "id" | "name">;
  sourcePageUrl?: string | null;
}): DesignRestoreScopeV1 {
  return withSourcePageUrl({
    version: "1.0",
    kind: "frame",
    designFileId: input.designFileId,
    revisionId: input.revisionId,
    frameId: input.frame.id,
    label: input.frame.name,
  }, input.sourcePageUrl);
}

export function createFigmaGroupRestoreScope(input: {
  designFileId: string;
  revisionId: string;
  group: GroupNode;
  sourcePageUrl?: string | null;
}): DesignRestoreScopeV1 {
  return withSourcePageUrl({
    version: "1.0",
    kind: "figma_group",
    designFileId: input.designFileId,
    revisionId: input.revisionId,
    groupId: input.group.id,
    groupName: input.group.name,
    groupPath: input.group.path,
    frameIds: input.group.frames.map((frame) => frame.id),
    label: input.group.name,
  }, input.sourcePageUrl);
}

export function createSelectionRestoreScope(input: {
  designFileId: string;
  revisionId: string;
  frame: Pick<DesignFrame, "id" | "name" | "rootLayerId">;
  layerIds: string[];
  selectionBounds?: DesignSelectionBoundsScope | null;
  sourcePageUrl?: string | null;
}): DesignRestoreScopeV1 {
  const layerIds = uniqueLayerIds(input.layerIds, input.frame.rootLayerId);
  if (input.selectionBounds) {
    return withSourcePageUrl({
      version: "1.0",
      kind: "selection_bounds",
      designFileId: input.designFileId,
      revisionId: input.revisionId,
      frameId: input.frame.id,
      selectionBounds: input.selectionBounds,
      includeIntersectingLayers: true,
      label: `${input.frame.name} · 框选区域`,
    }, input.sourcePageUrl);
  }
  return withSourcePageUrl({
    version: "1.0",
    kind: "selected_layers",
    designFileId: input.designFileId,
    revisionId: input.revisionId,
    frameId: input.frame.id,
    layerIds,
    label: `${input.frame.name} · 选中图层`,
  }, input.sourcePageUrl);
}

export function createDesignRestoreMCPPrompt(scope: DesignRestoreScopeV1, detailLevel: "compact" | "normal" | "full" = "normal") {
  const payload = {
    detailLevel,
    scope,
  };
  return [
    "使用 multica-design MCP 还原设计稿。",
    `先调用 multica_design_get_restore_pack：${JSON.stringify(payload)}`,
    "分组=同一业务页面多状态/弹窗；按 Restore Pack 实现并复用当前项目结构。",
  ].join("\n");
}
