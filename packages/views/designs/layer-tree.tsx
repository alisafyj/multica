"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronRight } from "lucide-react";
import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import type { FrameFidelityReport, LayerFidelityStatus } from "./native-renderer/fidelity";

type InspectFrame = GalleryNativeJson["frames"][number];

export type LayerTreeNode = {
  id: string;
  name: string;
  type: DesignLayer["type"];
  layer: DesignLayer | null;
  children: LayerTreeNode[];
};

type LayerTreeProps = {
  nativeJson: GalleryNativeJson;
  frame: InspectFrame;
  selectedLayerId: string | null;
  hoveredLayerId: string | null;
  fidelityReport?: FrameFidelityReport;
  onSelectLayer: (layerId: string) => void;
  onHoverLayer: (layerId: string | null) => void;
};

const TYPE_LABELS: Record<DesignLayer["type"], string> = {
  frame: "画板",
  group: "组",
  text: "文",
  image: "图",
  shape: "形",
  component: "组件",
  instance: "实例",
  vector: "矢量",
  slice: "切片",
  table: "表",
  form: "表单",
  custom: "层",
};

const STATUS_DOT_CLASS: Record<LayerFidelityStatus, string> = {
  native: "bg-emerald-500",
  fallback: "bg-amber-500",
  unsupported: "bg-destructive",
};

function canExpand(layer: DesignLayer | null) {
  return layer?.type === "frame" || layer?.type === "group" || layer?.type === "component" || layer?.type === "instance";
}

function buildLayerTree(nativeJson: GalleryNativeJson, frame: InspectFrame): LayerTreeNode | null {
  const visited = new Set<string>();

  const buildNode = (layerId: string): LayerTreeNode | null => {
    if (visited.has(layerId)) return null;
    visited.add(layerId);
    const layer = nativeJson.layers[layerId];
    if (!layer || layer.visible === false) return null;
    const children = (layer.children ?? []).map(buildNode).filter((node): node is LayerTreeNode => !!node);
    return { id: layer.id, name: layer.name, type: layer.type, layer, children };
  };

  const rootLayer = buildNode(frame.rootLayerId);
  if (rootLayer) return rootLayer;

  return {
    id: frame.rootLayerId,
    name: frame.name,
    type: "frame",
    layer: null,
    children: [],
  };
}

function collectExpandableIds(node: LayerTreeNode | null, depth = 0): string[] {
  if (!node) return [];
  const own = node.children.length && (depth === 0 || canExpand(node.layer)) ? [node.id] : [];
  return [...own, ...node.children.flatMap((child) => collectExpandableIds(child, depth + 1))];
}

function visibleNodeCount(node: LayerTreeNode | null): number {
  if (!node) return 0;
  return 1 + node.children.reduce((sum, child) => sum + visibleNodeCount(child), 0);
}

function LayerTreeRow({
  node,
  depth,
  expanded,
  selectedLayerId,
  hoveredLayerId,
  fidelityReport,
  onToggle,
  onSelectLayer,
  onHoverLayer,
}: {
  node: LayerTreeNode;
  depth: number;
  expanded: Set<string>;
  selectedLayerId: string | null;
  hoveredLayerId: string | null;
  fidelityReport?: FrameFidelityReport;
  onToggle: (layerId: string) => void;
  onSelectLayer: (layerId: string) => void;
  onHoverLayer: (layerId: string | null) => void;
}) {
  const isOpen = expanded.has(node.id);
  const hasChildren = node.children.length > 0 && (depth === 0 || canExpand(node.layer));
  const isSelected = selectedLayerId === node.id;
  const isHovered = hoveredLayerId === node.id;
  const fidelity = fidelityReport?.byLayerId[node.id];

  return (
    <div className="min-w-max">
      <div
        className={cn(
          "group flex h-8 w-full min-w-0 cursor-pointer items-center gap-1.5 rounded-lg pr-2 text-left text-xs transition-colors",
          isSelected && "bg-primary text-primary-foreground shadow-sm",
          !isSelected && isHovered && "bg-muted text-foreground",
          !isSelected && !isHovered && "text-muted-foreground hover:bg-muted/70 hover:text-foreground",
        )}
        style={{ paddingLeft: depth * 12 + 8 }}
        onClick={() => onSelectLayer(node.id)}
        onMouseEnter={() => onHoverLayer(node.id)}
        onMouseLeave={() => onHoverLayer(null)}
      >
        <span className="flex h-5 w-5 shrink-0 items-center justify-center">
          {hasChildren ? (
            <button
              type="button"
              aria-label={isOpen ? "收起图层" : "展开图层"}
              className="rounded-sm p-0.5 hover:bg-background/60"
              onClick={(event) => {
                event.stopPropagation();
                onToggle(node.id);
              }}
            >
              <ChevronRight className={cn("h-3.5 w-3.5 transition-transform", isOpen && "rotate-90")} />
            </button>
          ) : (
            <span className="h-1.5 w-1.5 rounded-full bg-current opacity-35" />
          )}
        </span>
        <Badge variant={isSelected ? "secondary" : "outline"} className="h-5 shrink-0 px-1.5 text-[10px] font-medium">
          {TYPE_LABELS[node.type] ?? "层"}
        </Badge>
        {fidelity ? <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", STATUS_DOT_CLASS[fidelity.status])} title={fidelity.reason} /> : null}
        <span className="truncate font-medium">{node.name || "未命名图层"}</span>
        {node.children.length ? <span className="ml-auto shrink-0 text-[10px] opacity-60">{node.children.length}</span> : null}
      </div>
      {hasChildren && isOpen ? (
        <div className="mt-0.5 space-y-0.5 border-l border-dashed border-border/70" style={{ marginLeft: depth * 12 + 17 }}>
          {node.children.map((child) => (
            <LayerTreeRow
              key={child.id}
              node={child}
              depth={depth + 1}
              expanded={expanded}
              selectedLayerId={selectedLayerId}
              hoveredLayerId={hoveredLayerId}
              fidelityReport={fidelityReport}
              onToggle={onToggle}
              onSelectLayer={onSelectLayer}
              onHoverLayer={onHoverLayer}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

export function LayerTree({ nativeJson, frame, selectedLayerId, hoveredLayerId, fidelityReport, onSelectLayer, onHoverLayer }: LayerTreeProps) {
  const tree = useMemo(() => buildLayerTree(nativeJson, frame), [nativeJson, frame]);
  const expandableIds = useMemo(() => collectExpandableIds(tree), [tree]);
  const totalCount = useMemo(() => visibleNodeCount(tree), [tree]);
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(expandableIds));

  useEffect(() => {
    setExpanded(new Set(expandableIds));
  }, [expandableIds]);

  const toggle = (layerId: string) => {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(layerId)) next.delete(layerId);
      else next.add(layerId);
      return next;
    });
  };

  return (
    <aside className="min-h-0 overflow-hidden rounded-2xl border bg-background shadow-sm">
      <div className="sticky top-0 z-10 border-b bg-background/95 p-4 backdrop-blur">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0">
            <div className="text-sm font-semibold">图层</div>
            <p className="mt-1 truncate text-xs text-muted-foreground">可见图层 · {totalCount} 项</p>
          </div>
          <Button type="button" size="sm" variant="outline" className="h-7 px-2 text-xs" onClick={() => setExpanded(new Set(expandableIds))}>
            展开
          </Button>
        </div>
      </div>
      <div className="h-full overflow-auto p-2">
        {tree ? (
          <LayerTreeRow
            node={tree}
            depth={0}
            expanded={expanded}
            selectedLayerId={selectedLayerId}
            hoveredLayerId={hoveredLayerId}
            fidelityReport={fidelityReport}
            onToggle={toggle}
            onSelectLayer={onSelectLayer}
            onHoverLayer={onHoverLayer}
          />
        ) : (
          <div className="rounded-xl border border-dashed p-4 text-xs text-muted-foreground">暂无可见图层。</div>
        )}
      </div>
    </aside>
  );
}
