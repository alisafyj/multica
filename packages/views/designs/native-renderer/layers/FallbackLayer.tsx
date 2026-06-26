import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";
import { firstFillBackground, isMaskLayer, layerFallbackAssetUrl, nativeLayerStyle } from "../style";

export function FallbackLayer({ nativeJson, layer }: { nativeJson: GalleryNativeJson; layer: DesignLayer }) {
  const fallbackUrl = layerFallbackAssetUrl(nativeJson, layer);
  if (fallbackUrl) {
    return <div style={nativeLayerStyle(layer, { transparent: true })}><img src={fallbackUrl} alt={layer.name} className="h-full w-full" style={{ objectFit: "fill" }} /></div>;
  }
  const canRenderStyle = !!firstFillBackground(layer.style);
  return <div style={{ ...nativeLayerStyle(layer, { transparent: !canRenderStyle }), outline: isMaskLayer(layer) ? "1px dashed rgba(92, 84, 240, 0.55)" : undefined, outlineOffset: isMaskLayer(layer) ? -1 : undefined }} />;
}
