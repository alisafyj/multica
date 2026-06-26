import type { CSSProperties } from "react";
import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";

export type Paint = { type?: string; gradientType?: string; color?: unknown; opacity?: number; assetId?: string; scaleMode?: string; stops?: Array<{ position?: number; color?: unknown }> };
export type Stroke = { color?: unknown; width?: number; dashPattern?: number[]; dash?: number[] };
export type Shadow = { type?: string; color?: unknown; offsetX?: number; offsetY?: number; blur?: number; spread?: number };

export function cssColor(value: unknown) {
  if (!value || typeof value !== "object") return null;
  const color = value as { css?: unknown; hex?: unknown; r?: unknown; g?: unknown; b?: unknown; a?: unknown };
  if (typeof color.css === "string") return color.css;
  if (typeof color.hex === "string") return color.hex;
  if (typeof color.r === "number" && typeof color.g === "number" && typeof color.b === "number") {
    const alpha = typeof color.a === "number" ? color.a : 1;
    return `rgba(${Math.round(color.r * 255)}, ${Math.round(color.g * 255)}, ${Math.round(color.b * 255)}, ${alpha})`;
  }
  return null;
}

export function styleArray<T>(style: Record<string, unknown> | undefined, key: string): T[] {
  const value = style?.[key];
  return Array.isArray(value) ? (value as T[]) : [];
}

export function firstStyleColor(style: Record<string, unknown> | undefined, keys: string[]) {
  if (!style) return null;
  for (const key of keys) {
    const raw = style[key];
    if (Array.isArray(raw) && raw[0]) {
      const color = cssColor((raw[0] as Record<string, unknown>).color ?? raw[0]);
      if (color) return color;
    }
    const color = cssColor(raw);
    if (color) return color;
  }
  return null;
}

export function paintColor(paint: Paint) {
  const color = cssColor(paint.color);
  if (!color || paint.opacity === undefined) return color;
  return color.startsWith("rgba(") ? color : color;
}

function gradientBackground(paint: Paint) {
  if (!paint.stops?.length) return null;
  const stops = paint.stops.map((stop) => `${cssColor(stop.color) ?? "transparent"} ${Math.round((stop.position ?? 0) * 100)}%`).join(", ");
  const type = `${paint.gradientType ?? paint.type ?? ""}`.toUpperCase();
  if (type.includes("RADIAL")) return `radial-gradient(circle, ${stops})`;
  return `linear-gradient(90deg, ${stops})`;
}

export function firstFillBackground(style: Record<string, unknown> | undefined) {
  const fill = styleArray<Paint>(style, "fills")[0];
  if (!fill) return firstStyleColor(style, ["fill", "backgroundColor"]);
  const gradient = gradientBackground(fill);
  if (gradient) return gradient;
  return paintColor(fill) ?? firstStyleColor(style, ["fill", "backgroundColor"]);
}

export function firstStroke(style: Record<string, unknown> | undefined) {
  const stroke = styleArray<Stroke>(style, "strokes")[0];
  const color = stroke ? cssColor(stroke.color) : firstStyleColor(style, ["stroke", "borderColor"]);
  const width = stroke?.width ?? 1;
  const dash = stroke?.dashPattern ?? stroke?.dash;
  return color ? { color, width, dashed: Array.isArray(dash) && dash.length > 0 } : null;
}

export function styleRadius(style: Record<string, unknown> | undefined) {
  const value = style?.cornerRadius;
  if (typeof value === "number") return `${value}px`;
  if (Array.isArray(value)) return value.map((item) => typeof item === "number" ? `${item}px` : "0px").join(" / ");
  return undefined;
}

export function layerImageUrl(nativeJson: GalleryNativeJson | undefined, layer: DesignLayer) {
  if (layer.image?.assetId) {
    const url = nativeJson?.assets?.[layer.image.assetId]?.url ?? null;
    return url && !url.startsWith("figma-image-hash://") ? url : null;
  }
  for (const paint of styleArray<Paint>(layer.style, "fills")) {
    if (paint.assetId) {
      const url = nativeJson?.assets?.[paint.assetId]?.url ?? null;
      return url && !url.startsWith("figma-image-hash://") ? url : null;
    }
  }
  return null;
}

export function layerFallbackAssetUrl(nativeJson: GalleryNativeJson | undefined, layer: DesignLayer) {
  const fallbackAssetId = typeof layer.style?.fallbackAssetId === "string" ? layer.style.fallbackAssetId : null;
  if (!fallbackAssetId) return null;
  const url = nativeJson?.assets?.[fallbackAssetId]?.url ?? null;
  return url && !url.startsWith("figma-image-hash://") ? url : null;
}

export function layerImageFit(layer: DesignLayer): CSSProperties["objectFit"] {
  switch (layerImageScaleMode(layer)) {
    case "FIT":
      return "contain";
    case "STRETCH":
      return "fill";
    default:
      return "cover";
  }
}

export function layerImageScaleMode(layer: DesignLayer) {
  const paint = styleArray<Paint>(layer.style, "fills").find((item) => item.assetId || item.type === "image");
  return (paint?.scaleMode ?? "FILL").toUpperCase();
}

export function layerBoxShadow(layer: DesignLayer) {
  const shadows = styleArray<Shadow>(layer.style, "shadows");
  if (!shadows.length) return undefined;
  return shadows.map((shadow) => {
    const color = cssColor(shadow.color) ?? "rgba(0,0,0,.18)";
    return `${shadow.offsetX ?? 0}px ${shadow.offsetY ?? 0}px ${shadow.blur ?? 0}px ${shadow.spread ?? 0}px ${color}`;
  }).join(", ");
}

export function nativeLayerStyle(layer: DesignLayer, options?: { transparent?: boolean }): CSSProperties {
  const fill = options?.transparent ? null : firstFillBackground(layer.style);
  const stroke = firstStroke(layer.style);
  return {
    position: "absolute",
    left: layer.x,
    top: layer.y,
    width: layer.width,
    height: layer.height,
    opacity: layer.opacity ?? 1,
    transform: layer.rotation ? `rotate(${layer.rotation}deg)` : undefined,
    transformOrigin: "center",
    borderRadius: layer.shape?.shapeType === "ellipse" ? "9999px" : styleRadius(layer.style),
    background: fill ?? undefined,
    border: stroke ? `${stroke.width}px ${stroke.dashed ? "dashed" : "solid"} ${stroke.color}` : undefined,
    boxShadow: layerBoxShadow(layer),
    overflow: "hidden",
  };
}
