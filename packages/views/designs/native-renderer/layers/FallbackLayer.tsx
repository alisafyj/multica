import type { DesignLayer } from "@multica/core/types";
import { firstFillBackground, nativeLayerStyle } from "../style";

export function FallbackLayer({ layer }: { layer: DesignLayer }) {
  const canRenderStyle = !!firstFillBackground(layer.style);
  return <div style={nativeLayerStyle(layer, { transparent: !canRenderStyle })} />;
}
