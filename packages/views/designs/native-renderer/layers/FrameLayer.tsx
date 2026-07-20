import type { DesignLayer } from "@multica/core/types";
import { isMaskLayer, nativeLayerStyle } from "../style";

export function FrameLayer({ layer }: { layer: DesignLayer }) {
  return <div style={{ ...nativeLayerStyle(layer), outline: isMaskLayer(layer) ? "1px dashed rgba(92, 84, 240, 0.55)" : undefined, outlineOffset: isMaskLayer(layer) ? -1 : undefined }} />;
}
