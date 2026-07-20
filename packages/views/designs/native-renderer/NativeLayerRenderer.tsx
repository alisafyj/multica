import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";
import { layerExportableAssetUrl, layerFallbackAssetUrl, layerHasImageFill, layerImageUrl, layerPreviewCrop } from "./style";
import { FallbackLayer } from "./layers/FallbackLayer";
import { FrameLayer } from "./layers/FrameLayer";
import { ImageLayer } from "./layers/ImageLayer";
import { ShapeLayer } from "./layers/ShapeLayer";
import { TextLayer } from "./layers/TextLayer";

export function layerRendersAsReplacement(nativeJson: GalleryNativeJson, layer: DesignLayer) {
  if (layerImageUrl(nativeJson, layer)) return false;
  if (layerFallbackAssetUrl(nativeJson, layer) || (layer.parentId && layerExportableAssetUrl(nativeJson, layer))) return true;
  if (layerHasImageFill(layer) || layer.type === "image") return true;
  return (layer.type === "group" || layer.type === "component" || layer.type === "instance" || layer.type === "vector" || layer.type === "custom") && Boolean(layerPreviewCrop(nativeJson, layer));
}

export function NativeLayerRenderer({ nativeJson, layer }: { nativeJson: GalleryNativeJson; layer: DesignLayer }) {
  if (layer.type === "text") return <TextLayer layer={layer} />;
  if (layerImageUrl(nativeJson, layer)) return <ImageLayer nativeJson={nativeJson} layer={layer} />;
  if (layerRendersAsReplacement(nativeJson, layer)) {
    if (layerHasImageFill(layer) || layer.type === "image") return <ImageLayer nativeJson={nativeJson} layer={layer} />;
    return <FallbackLayer nativeJson={nativeJson} layer={layer} />;
  }
  if (layer.type === "shape") return <ShapeLayer layer={layer} />;
  if (layer.type === "frame" || layer.type === "group" || layer.type === "component" || layer.type === "instance") return <FrameLayer layer={layer} />;
  return <FallbackLayer nativeJson={nativeJson} layer={layer} />;
}
