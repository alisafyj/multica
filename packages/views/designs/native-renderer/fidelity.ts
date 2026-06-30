import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";
import { firstFillBackground, firstStroke, framePreviewAsset, layerExportableAssetUrl, layerFallbackAsset, layerFallbackAssetUrl, layerHasImageFill, layerImageUrl, layerPreviewCrop, styleArray } from "./style";

type InspectFrame = GalleryNativeJson["frames"][number];

export type LayerFidelityStatus = "native" | "fallback" | "unsupported";

export type LayerFidelity = {
  layerId: string;
  status: LayerFidelityStatus;
  reason: string;
};

export type FrameFidelityReport = {
  byLayerId: Record<string, LayerFidelity>;
  total: number;
  native: number;
  fallback: number;
  unsupported: number;
  nativePercent: number;
  renderQualityPercent: number;
};

function hasStyleSignal(layer: DesignLayer) {
  return Boolean(
    firstFillBackground(layer.style)
      || firstStroke(layer.style)
      || styleArray(layer.style, "shadows").length
      || layer.style?.cornerRadius !== undefined
      || layer.opacity !== undefined,
  );
}

function layerSource(layer: DesignLayer) {
  return (layer.source ?? {}) as { isMask?: unknown; clipsContent?: unknown; layoutMode?: unknown };
}

function layerAutoLayout(layer: DesignLayer) {
  return (layer.style?.autoLayout ?? null) as { layoutMode?: unknown } | null;
}

function hasUnuploadedImageFill(layer: DesignLayer) {
  return styleArray<{ assetId?: string; imageHash?: string; type?: string }>(layer.style, "fills").some((paint) => paint.type === "image" && Boolean(paint.assetId || paint.imageHash));
}

function hasAssetFallback(nativeJson: GalleryNativeJson, layer: DesignLayer) {
  if (layerFallbackAssetUrl(nativeJson, layer)) return true;
  if (layer.parentId && layerExportableAssetUrl(nativeJson, layer)) return true;
  if (layer.exportable?.length) return true;
  if (layer.image?.assetId && nativeJson.assets[layer.image.assetId]) return true;
  return styleArray<{ assetId?: string; imageHash?: string; type?: string }>(layer.style, "fills").some((paint) => {
    if (paint.assetId && nativeJson.assets[paint.assetId]) return true;
    return paint.type === "image" || Boolean(paint.imageHash);
  });
}

export function classifyLayerFidelity(nativeJson: GalleryNativeJson, layer: DesignLayer): LayerFidelity {
  if (layer.type === "text") return { layerId: layer.id, status: "native", reason: "文本图层可原生渲染" };

  if (layer.type === "image") {
    if (layerImageUrl(nativeJson, layer)) return { layerId: layer.id, status: "native", reason: "图片资产可原生渲染" };
    if (framePreviewAsset(nativeJson, layer.frameId)) return { layerId: layer.id, status: "fallback", reason: "图片资产缺失，使用画板预览局部裁切兜底" };
    if (hasAssetFallback(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: "图片资产缺失，暂以占位呈现" };
    return { layerId: layer.id, status: "unsupported", reason: "图片资产缺失" };
  }

  if (hasUnuploadedImageFill(layer) && !layerImageUrl(nativeJson, layer)) {
    if (framePreviewAsset(nativeJson, layer.frameId)) return { layerId: layer.id, status: "fallback", reason: "图片填充缺失，使用画板预览局部裁切兜底" };
    return { layerId: layer.id, status: "fallback", reason: "图片填充尚未上传，暂以占位呈现" };
  }

  if (layerHasImageFill(layer) && !layerImageUrl(nativeJson, layer) && framePreviewAsset(nativeJson, layer.frameId)) {
    return { layerId: layer.id, status: "fallback", reason: "图片填充缺失，使用画板预览局部裁切兜底" };
  }

  if (layer.type === "shape") {
    if (layerImageUrl(nativeJson, layer)) return { layerId: layer.id, status: "native", reason: "图片填充可原生渲染" };
    if (layerFallbackAssetUrl(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: "形状使用局部兜底图呈现" };
    if (layer.shape?.shapeType === "line") return { layerId: layer.id, status: "native", reason: "线条可原生渲染" };
    if (layer.shape?.shapeType === "ellipse") return { layerId: layer.id, status: "native", reason: "椭圆可原生渲染" };
    if (firstFillBackground(layer.style) || firstStroke(layer.style)) return { layerId: layer.id, status: "native", reason: "形状样式可原生渲染" };
    if (hasAssetFallback(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: "形状缺少可渲染样式，暂以占位呈现" };
    return { layerId: layer.id, status: "native", reason: "透明辅助形状无独立视觉输出" };
  }

  if (layer.type === "frame" || layer.type === "group" || layer.type === "component" || layer.type === "instance") {
    const source = layerSource(layer);
    if (layerFallbackAssetUrl(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: "使用局部兜底图呈现" };
    if (layer.parentId && layerExportableAssetUrl(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: "使用旧版导出切片呈现" };
    if (layerPreviewCrop(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: "复杂小组件使用画板预览局部裁切兜底" };
    if (source.isMask) return { layerId: layer.id, status: "fallback", reason: "包含蒙版信息，当前以结构容器呈现" };
    if (source.clipsContent) return { layerId: layer.id, status: "native", reason: "裁切容器可原生渲染" };
    if (layerAutoLayout(layer)?.layoutMode || source.layoutMode) return { layerId: layer.id, status: "native", reason: "Auto Layout 已识别，当前按结构容器呈现" };
    if (hasStyleSignal(layer)) return { layerId: layer.id, status: "native", reason: "容器样式可原生渲染" };
    if (layer.children?.length) return { layerId: layer.id, status: "native", reason: "结构容器可原生渲染" };
    return { layerId: layer.id, status: "unsupported", reason: "空容器暂无可渲染内容" };
  }

  if (layer.type === "vector" || layer.type === "slice" || layer.type === "custom") {
    const fallbackAsset = layerFallbackAsset(nativeJson, layer);
    if (layerFallbackAssetUrl(nativeJson, layer)) {
      const isSvg = fallbackAsset?.contentType === "image/svg+xml" || fallbackAsset?.metadata?.exportFormat === "SVG";
      return { layerId: layer.id, status: "fallback", reason: `${layer.type === "vector" ? "矢量" : layer.type === "slice" ? "切片" : "自定义图层"}使用局部${isSvg ? " SVG" : "兜底图"}呈现` };
    }
    if (layer.parentId && layerExportableAssetUrl(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: `${layer.type === "vector" ? "矢量" : layer.type === "slice" ? "切片" : "自定义图层"}使用旧版导出切片呈现` };
    if (layerPreviewCrop(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: `${layer.type === "vector" ? "矢量" : layer.type === "slice" ? "切片" : "自定义图层"}使用画板预览局部裁切兜底` };
    return { layerId: layer.id, status: "fallback", reason: `${layer.type === "vector" ? "矢量" : layer.type === "slice" ? "切片" : "自定义图层"}暂以占位呈现` };
  }

  if (hasStyleSignal(layer) || hasAssetFallback(nativeJson, layer)) return { layerId: layer.id, status: "fallback", reason: "复杂图层暂以占位呈现" };
  return { layerId: layer.id, status: "unsupported", reason: "暂无可识别的渲染信号" };
}

export function analyzeFrameFidelity(nativeJson: GalleryNativeJson, frame: InspectFrame): FrameFidelityReport {
  const byLayerId: Record<string, LayerFidelity> = {};
  const visibleLayerIds: string[] = [];
  const visited = new Set<string>();

  const visit = (layerId: string) => {
    if (visited.has(layerId)) return;
    visited.add(layerId);
    const layer = nativeJson.layers[layerId];
    if (!layer || layer.visible === false) return;
    visibleLayerIds.push(layer.id);
    byLayerId[layer.id] = classifyLayerFidelity(nativeJson, layer);
    for (const childId of layer.children ?? []) visit(childId);
  };

  visit(frame.rootLayerId);

  const aggregateIds = visibleLayerIds.filter((layerId) => layerId !== frame.rootLayerId);
  const native = aggregateIds.filter((layerId) => byLayerId[layerId]?.status === "native").length;
  const fallback = aggregateIds.filter((layerId) => byLayerId[layerId]?.status === "fallback").length;
  const unsupported = aggregateIds.filter((layerId) => byLayerId[layerId]?.status === "unsupported").length;
  const total = aggregateIds.length;
  const qualityScore = aggregateIds.reduce((sum, layerId) => sum + layerRenderQualityScore(byLayerId[layerId]), 0);

  return {
    byLayerId,
    total,
    native,
    fallback,
    unsupported,
    nativePercent: total ? Math.round((native / total) * 100) : 100,
    renderQualityPercent: total ? Math.round((qualityScore / total) * 100) : 100,
  };
}

function layerRenderQualityScore(fidelity?: LayerFidelity) {
  if (!fidelity) return 0;
  if (fidelity.status === "native") return 1;
  if (fidelity.status === "unsupported") return 0;
  if (fidelity.reason.includes("占位")) return 0.35;
  if (fidelity.reason.includes("局部") || fidelity.reason.includes("导出切片") || fidelity.reason.includes("裁切")) return 0.98;
  return 0.7;
}
