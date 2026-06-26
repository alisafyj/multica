import type { CSSProperties } from "react";
import type { DesignLayer } from "@multica/core/types";
import { cssColor, firstStyleColor, nativeLayerStyle } from "../style";

export function TextLayer({ layer }: { layer: DesignLayer }) {
  if (!layer.text) return null;
  const textFill = firstStyleColor(layer.style, ["fills", "fill", "backgroundColor"]);
  const verticalAlign = layer.text.textAlignVertical === "center" ? "center" : layer.text.textAlignVertical === "bottom" ? "flex-end" : "flex-start";
  return (
    <div
      style={{
        ...nativeLayerStyle(layer),
        color: cssColor(layer.text.color) ?? textFill ?? "#18181b",
        fontFamily: layer.text.fontFamily ?? "Inter, Arial, sans-serif",
        fontSize: layer.text.fontSize ?? 14,
        fontWeight: layer.text.fontWeight as CSSProperties["fontWeight"],
        lineHeight: typeof layer.text.lineHeight === "number" ? `${layer.text.lineHeight}px` : undefined,
        letterSpacing: typeof layer.text.letterSpacing === "number" ? layer.text.letterSpacing : undefined,
        textAlign: layer.text.textAlignHorizontal === "justified" ? "justify" : layer.text.textAlignHorizontal,
        whiteSpace: "pre-wrap",
        display: "flex",
        alignItems: "stretch",
        justifyContent: verticalAlign,
        overflow: "hidden",
      }}
    >
      <span className="block min-w-0 flex-1">{layer.text.characters ?? layer.text.text ?? ""}</span>
    </div>
  );
}
