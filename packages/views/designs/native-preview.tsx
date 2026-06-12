import { useMemo, useState } from "react";
import type { CSSProperties } from "react";
import type { DesignLayer, GalleryNativeJson } from "@multica/core/types";

type NativeFrame = GalleryNativeJson["frames"][number];

type Paint = { type?: string; color?: unknown; opacity?: number; assetId?: string; scaleMode?: string; stops?: Array<{ position?: number; color?: unknown }> };
type Stroke = { color?: unknown; width?: number };
type Shadow = { type?: string; color?: unknown; offsetX?: number; offsetY?: number; blur?: number; spread?: number };

function cssColor(value: unknown) {
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

function styleArray<T>(style: Record<string, unknown> | undefined, key: string): T[] {
  const value = style?.[key];
  return Array.isArray(value) ? (value as T[]) : [];
}

function firstStyleColor(style: Record<string, unknown> | undefined, keys: string[]) {
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

function paintColor(paint: Paint) {
  const color = cssColor(paint.color);
  if (!color || paint.opacity === undefined) return color;
  return color.startsWith("rgba(") ? color : color;
}

function firstFillBackground(style: Record<string, unknown> | undefined) {
  const fill = styleArray<Paint>(style, "fills")[0];
  if (!fill) return firstStyleColor(style, ["fill", "backgroundColor"]);
  if (fill.type === "gradient" && fill.stops?.length) {
    const stops = fill.stops.map((stop) => `${cssColor(stop.color) ?? "transparent"} ${Math.round((stop.position ?? 0) * 100)}%`).join(", ");
    return `linear-gradient(90deg, ${stops})`;
  }
  return paintColor(fill) ?? firstStyleColor(style, ["fill", "backgroundColor"]);
}

function styleRadius(style: Record<string, unknown> | undefined) {
  const value = style?.cornerRadius;
  if (typeof value === "number") return `${value}px`;
  if (Array.isArray(value)) return value.map((item) => typeof item === "number" ? `${item}px` : "0px").join(" / ");
  return undefined;
}

function layerImageUrl(nativeJson: GalleryNativeJson | undefined, layer: DesignLayer) {
  if (layer.image?.assetId) return nativeJson?.assets?.[layer.image.assetId]?.url ?? null;
  for (const paint of styleArray<Paint>(layer.style, "fills")) {
    if (paint.assetId) return nativeJson?.assets?.[paint.assetId]?.url ?? null;
  }
  return null;
}

function layerImageFit(layer: DesignLayer): CSSProperties["objectFit"] {
  const paint = styleArray<Paint>(layer.style, "fills").find((item) => item.assetId || item.type === "image");
  switch ((paint?.scaleMode ?? "").toUpperCase()) {
    case "FIT":
      return "contain";
    case "STRETCH":
      return "fill";
    default:
      return "cover";
  }
}

function layerBoxShadow(layer: DesignLayer) {
  const shadows = styleArray<Shadow>(layer.style, "shadows");
  if (!shadows.length) return undefined;
  return shadows.map((shadow) => {
    const color = cssColor(shadow.color) ?? "rgba(0,0,0,.18)";
    return `${shadow.offsetX ?? 0}px ${shadow.offsetY ?? 0}px ${shadow.blur ?? 0}px ${shadow.spread ?? 0}px ${color}`;
  }).join(", ");
}

function orderedFrameLayers(nativeJson: GalleryNativeJson, frame: NativeFrame) {
  const seen = new Set<string>();
  const ordered: DesignLayer[] = [];
  const visit = (layerId: string) => {
    const layer = nativeJson.layers[layerId];
    if (!layer || seen.has(layerId)) return;
    seen.add(layerId);
    if (layer.id !== frame.rootLayerId && layer.frameId === frame.id && layer.visible !== false && layer.width > 0 && layer.height > 0) ordered.push(layer);
    for (const childId of layer.children ?? []) visit(childId);
  };
  visit(frame.rootLayerId);
  for (const layer of Object.values(nativeJson.layers)) {
    if (!seen.has(layer.id) && layer.frameId === frame.id && layer.id !== frame.rootLayerId && layer.visible !== false && layer.width > 0 && layer.height > 0) ordered.push(layer);
  }
  return ordered;
}

function NativeLayer({ nativeJson, layer }: { nativeJson: GalleryNativeJson; layer: DesignLayer }) {
  const imageUrl = layerImageUrl(nativeJson, layer);
  const fill = firstFillBackground(layer.style);
  const textFill = firstStyleColor(layer.style, ["fills", "fill", "backgroundColor"]);
  const stroke = firstStyleColor(layer.style, ["strokes", "stroke", "borderColor"]);
  const strokeWidth = styleArray<Stroke>(layer.style, "strokes")[0]?.width ?? 1;
  const baseStyle: CSSProperties = {
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
    border: stroke ? `${strokeWidth}px solid ${stroke}` : undefined,
    boxShadow: layerBoxShadow(layer),
    overflow: "hidden",
  };
  if (layer.type === "text" && layer.text) {
    return (
      <div
        style={{
          ...baseStyle,
          color: cssColor(layer.text.color) ?? textFill ?? "#18181b",
          fontFamily: layer.text.fontFamily ?? "Inter, Arial, sans-serif",
          fontSize: layer.text.fontSize ?? 14,
          fontWeight: layer.text.fontWeight as CSSProperties["fontWeight"],
          lineHeight: typeof layer.text.lineHeight === "number" ? `${layer.text.lineHeight}px` : undefined,
          letterSpacing: typeof layer.text.letterSpacing === "number" ? layer.text.letterSpacing : undefined,
          textAlign: layer.text.textAlignHorizontal === "justified" ? "justify" : layer.text.textAlignHorizontal,
          whiteSpace: "pre-wrap",
          display: "flex",
          alignItems: layer.text.textAlignVertical === "center" ? "center" : layer.text.textAlignVertical === "bottom" ? "flex-end" : "flex-start",
        }}
      >
        {layer.text.characters ?? layer.text.text ?? ""}
      </div>
    );
  }
  if (imageUrl || layer.type === "image") {
    return <div style={baseStyle}>{imageUrl ? <img src={imageUrl} alt={layer.name} className="h-full w-full" style={{ objectFit: layerImageFit(layer) }} /> : null}</div>;
  }
  if (layer.type === "shape" || layer.type === "frame" || layer.type === "group" || layer.type === "component" || layer.type === "instance") {
    return <div style={baseStyle} />;
  }
  return null;
}

export function NativeFramePreview({ nativeJson, frame, className }: { nativeJson: GalleryNativeJson | undefined; frame: NativeFrame | undefined; className?: string }) {
  if (!nativeJson || !frame) return <div className={className}>暂无可预览设计数据</div>;
  const layers = orderedFrameLayers(nativeJson, frame);
  const rootLayer = nativeJson.layers[frame.rootLayerId];
  const background = firstFillBackground(rootLayer?.style) ?? "hsl(var(--background))";
  return (
    <div className={className} style={{ background }}>
      {layers.map((layer) => <NativeLayer key={layer.id} nativeJson={nativeJson} layer={layer} />)}
    </div>
  );
}

export function NativeDesignPreview({ nativeJson, className }: { nativeJson: GalleryNativeJson | undefined; className?: string }) {
  const frames = useMemo(() => nativeJson?.frames ?? [], [nativeJson]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const safeIndex = Math.min(selectedIndex, Math.max(frames.length - 1, 0));
  const frame = frames[safeIndex];
  if (!nativeJson || !frame) return <div className={className}>暂无可预览设计数据</div>;
  const scale = Math.min(1, 760 / Math.max(frame.width, 1), 520 / Math.max(frame.height, 1));
  return (
    <div className={className}>
      <div className="mb-2 flex items-center justify-between text-xs text-muted-foreground">
        <span className="truncate">{frame.name}</span>
        <span className="font-mono">{Math.round(frame.width)}×{Math.round(frame.height)}</span>
      </div>
      {frames.length > 1 ? (
        <div className="mb-3 flex gap-2 overflow-auto pb-1">
          {frames.map((item, index) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setSelectedIndex(index)}
              className={`shrink-0 rounded-full border px-3 py-1 text-xs ${index === safeIndex ? "border-primary bg-primary text-primary-foreground" : "bg-background text-muted-foreground hover:bg-accent hover:text-foreground"}`}
            >
              {item.name}
            </button>
          ))}
        </div>
      ) : null}
      <div className="overflow-auto rounded-lg border bg-muted/30 p-3">
        <div className="mx-auto" style={{ width: frame.width * scale, height: frame.height * scale }}>
          <div className="relative overflow-hidden bg-background shadow-sm" style={{ width: frame.width, height: frame.height, transform: `scale(${scale})`, transformOrigin: "top left" }}>
            <NativeFramePreview nativeJson={nativeJson} frame={frame} className="absolute inset-0" />
          </div>
        </div>
      </div>
    </div>
  );
}
