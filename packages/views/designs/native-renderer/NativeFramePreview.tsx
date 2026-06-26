import type { GalleryNativeJson } from "@multica/core/types";
import { NativeLayerRenderer } from "./NativeLayerRenderer";
import { orderedFrameLayers } from "./ordering";
import { firstFillBackground } from "./style";

type NativeFrame = GalleryNativeJson["frames"][number];

export function NativeFramePreview({ nativeJson, frame, className }: { nativeJson: GalleryNativeJson | undefined; frame: NativeFrame | undefined; className?: string }) {
  if (!nativeJson || !frame) return <div className={className}>暂无可预览设计数据</div>;
  const layers = orderedFrameLayers(nativeJson, frame);
  const rootLayer = nativeJson.layers[frame.rootLayerId];
  const background = firstFillBackground(rootLayer?.style) ?? "hsl(var(--background))";
  return (
    <div className={className} style={{ background }}>
      {layers.map((layer) => <NativeLayerRenderer key={layer.id} nativeJson={nativeJson} layer={layer} />)}
    </div>
  );
}
