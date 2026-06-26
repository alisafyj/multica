import type { DesignLayer } from "@multica/core/types";
import { nativeLayerStyle } from "../style";

export function FrameLayer({ layer }: { layer: DesignLayer }) {
  return <div style={nativeLayerStyle(layer)} />;
}
