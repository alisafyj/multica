import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";
import { layerImageFit, layerImageScaleMode, layerImageUrl, layerPreviewCrop, nativeLayerStyle } from "../style";

export function ImageLayer({ nativeJson, layer }: { nativeJson: GalleryNativeJson; layer: DesignLayer }) {
  const imageUrl = layerImageUrl(nativeJson, layer);
  const scaleMode = layerImageScaleMode(layer);
  if (!imageUrl) {
    const previewCrop = layerPreviewCrop(nativeJson, layer, layer.y < 0 ? 1 : 0.25);
    if (previewCrop) {
      return (
        <div
          style={{
            ...nativeLayerStyle(layer, { transparent: true }),
            ...previewCrop.style,
          }}
          role="img"
          aria-label={layer.image?.alt ?? layer.name}
        />
      );
    }
    return <div style={nativeLayerStyle(layer)} />;
  }
  if (scaleMode === "TILE") {
    return <div style={{ ...nativeLayerStyle(layer), backgroundImage: `url(${imageUrl})`, backgroundRepeat: "repeat", backgroundSize: "auto" }} role="img" aria-label={layer.image?.alt ?? layer.name} />;
  }
  return <div style={nativeLayerStyle(layer)}><img src={imageUrl} alt={layer.image?.alt ?? layer.name} className="h-full w-full" style={{ objectFit: layerImageFit(layer) }} /></div>;
}
