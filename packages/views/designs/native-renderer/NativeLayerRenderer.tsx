import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";
import { layerFallbackAssetUrl, layerImageUrl } from "./style";
import { FallbackLayer } from "./layers/FallbackLayer";
import { FrameLayer } from "./layers/FrameLayer";
import { ImageLayer } from "./layers/ImageLayer";
import { ShapeLayer } from "./layers/ShapeLayer";
import { TextLayer } from "./layers/TextLayer";

export function NativeLayerRenderer({ nativeJson, layer }: { nativeJson: GalleryNativeJson; layer: DesignLayer }) {
  if (layer.type === "text") return <TextLayer layer={layer} />;
  if (layerImageUrl(nativeJson, layer) || layer.type === "image") return <ImageLayer nativeJson={nativeJson} layer={layer} />;
  if (layerFallbackAssetUrl(nativeJson, layer)) return <FallbackLayer nativeJson={nativeJson} layer={layer} />;
  if (layer.type === "shape") return <ShapeLayer layer={layer} />;
  if (layer.type === "frame" || layer.type === "group" || layer.type === "component" || layer.type === "instance") return <FrameLayer layer={layer} />;
  return <FallbackLayer nativeJson={nativeJson} layer={layer} />;
}
