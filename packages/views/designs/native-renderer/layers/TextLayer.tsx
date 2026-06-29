import type { CSSProperties } from "react";
import type { DesignLayer } from "@multica/core/types";
import { cssColor, firstStyleColor, nativeLayerStyle } from "../style";

export function TextLayer({ layer }: { layer: DesignLayer }) {
  if (!layer.text) return null;
  const text = layer.text.characters ?? layer.text.text ?? "";
  const textFill = firstStyleColor(layer.style, ["fills", "fill", "backgroundColor"]);
  const verticalAlign = layer.text.textAlignVertical === "center" ? "center" : layer.text.textAlignVertical === "bottom" ? "flex-end" : "flex-start";
  const lineHeight = typeof layer.text.lineHeight === "number" ? layer.text.lineHeight : (layer.text.fontSize ?? 14) * 1.2;
  const isSingleLine = !text.includes("\n") && layer.height <= lineHeight * 1.6;
  return (
    <div
      style={{
        ...nativeLayerStyle(layer),
        background: "transparent",
        color: cssColor(layer.text.color) ?? textFill ?? "#18181b",
        fontFamily: layer.text.fontFamily ?? "Inter, Arial, sans-serif",
        fontSize: layer.text.fontSize ?? 14,
        fontWeight: layer.text.fontWeight as CSSProperties["fontWeight"],
        lineHeight: typeof layer.text.lineHeight === "number" ? `${layer.text.lineHeight}px` : undefined,
        letterSpacing: typeof layer.text.letterSpacing === "number" ? layer.text.letterSpacing : undefined,
        textAlign: layer.text.textAlignHorizontal === "justified" ? "justify" : layer.text.textAlignHorizontal,
        whiteSpace: isSingleLine ? "nowrap" : "pre-wrap",
        display: "flex",
        flexDirection: "column",
        alignItems: "stretch",
        justifyContent: verticalAlign,
        overflow: "visible",
      }}
    >
      <span className={isSingleLine ? "block min-w-max flex-1" : "block min-w-0 flex-1"}>{text}</span>
    </div>
  );
}
