import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";
import { firstFillBackground, layerFallbackAssetUrl, nativeLayerStyle } from "../style";

export function FallbackLayer({ nativeJson, layer }: { nativeJson: GalleryNativeJson; layer: DesignLayer }) {
  const fallbackUrl = layerFallbackAssetUrl(nativeJson, layer);
  if (fallbackUrl) {
    return <div style={nativeLayerStyle(layer, { transparent: true })}><img src={fallbackUrl} alt={layer.name} className="h-full w-full" style={{ objectFit: "fill" }} /></div>;
  }
  const canRenderStyle = !!firstFillBackground(layer.style);
  return <div style={nativeLayerStyle(layer, { transparent: !canRenderStyle })} />;
}
