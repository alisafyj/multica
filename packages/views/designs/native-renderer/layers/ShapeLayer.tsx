import type { DesignLayer } from "@multica/core/types";
import { firstStroke, nativeLayerStyle } from "../style";

export function ShapeLayer({ layer }: { layer: DesignLayer }) {
  if (layer.shape?.shapeType === "line") {
    const stroke = firstStroke(layer.style);
    const thickness = stroke?.width ?? Math.max(1, Math.min(layer.height || 1, layer.width || 1));
    return (
      <div
        style={{
          ...nativeLayerStyle(layer, { transparent: true }),
          height: thickness,
          minHeight: thickness,
          border: 0,
          borderTop: `${thickness}px ${stroke?.dashed ? "dashed" : "solid"} ${stroke?.color ?? "currentColor"}`,
          background: "transparent",
        }}
      />
    );
  }
  return <div style={nativeLayerStyle(layer)} />;
}
