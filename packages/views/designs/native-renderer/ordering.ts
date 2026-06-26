import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";

type NativeFrame = GalleryNativeJson["frames"][number];

export function orderedFrameLayers(nativeJson: GalleryNativeJson, frame: NativeFrame) {
  const seen = new Set<string>();
  const ordered: DesignLayer[] = [];
  const visit = (layerId: string) => {
    const layer = nativeJson.layers[layerId];
    if (!layer || seen.has(layerId)) return;
    seen.add(layerId);
    if (layer.id !== frame.rootLayerId && layer.frameId === frame.id && layer.visible !== false && layer.width > 0 && layer.height > 0) ordered.push(layer);
    for (const childId of layer.children ?? []) visit(childId);
  };
  visit(frame.rootLayerId);
  for (const layer of Object.values(nativeJson.layers)) {
    if (!seen.has(layer.id) && layer.frameId === frame.id && layer.id !== frame.rootLayerId && layer.visible !== false && layer.width > 0 && layer.height > 0) ordered.push(layer);
  }
  return ordered;
}
