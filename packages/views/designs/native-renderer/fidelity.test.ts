import { describe, expect, it } from "vitest";
import { analyzeFrameFidelity } from "./fidelity";
import type { GalleryNativeJson } from "@multica/core/types";

describe("analyzeFrameFidelity", () => {
  it("scores shape layers with uploaded fallback assets as local fallback renders", () => {
    const nativeJson = {
      version: "1.0",
      file: { id: "file", title: "Test", sourceType: "figma" },
      frames: [{ id: "frame-1", name: "Frame", rootLayerId: "root", width: 100, height: 100 }],
      layers: {
        root: { id: "root", frameId: "frame-1", name: "Root", type: "frame", visible: true, x: 0, y: 0, width: 100, height: 100, children: ["mask"] },
        mask: { id: "mask", frameId: "frame-1", parentId: "root", name: "Mask", type: "shape", visible: true, x: 0, y: 0, width: 40, height: 40, style: { fallbackAssetId: "fallback-mask" } },
      },
      assets: {
        "fallback-mask": { id: "fallback-mask", kind: "image", url: "http://localhost:8080/fallback.png", contentType: "image/png", width: 40, height: 40 },
      },
    } as unknown as GalleryNativeJson;

    const report = analyzeFrameFidelity(nativeJson, nativeJson.frames[0]!);

    expect(report.fallback).toBe(1);
    expect(report.renderQualityPercent).toBe(98);
    expect(report.byLayerId.mask!.reason).toContain("局部");
  });

  it("scores shape layers with uploaded image fills as native renders", () => {
    const nativeJson = {
      version: "1.0",
      file: { id: "file", title: "Test", sourceType: "figma" },
      frames: [{ id: "frame-1", name: "Frame", rootLayerId: "root", width: 100, height: 100 }],
      layers: {
        root: { id: "root", frameId: "frame-1", name: "Root", type: "frame", visible: true, x: 0, y: 0, width: 100, height: 100, children: ["photo"] },
        photo: { id: "photo", frameId: "frame-1", parentId: "root", name: "Photo", type: "shape", visible: true, x: 0, y: 0, width: 40, height: 40, style: { fills: [{ type: "image", assetId: "image-photo" }] } },
      },
      assets: {
        "image-photo": { id: "image-photo", kind: "image", url: "http://localhost:8080/photo.png", contentType: "image/png", width: 40, height: 40 },
      },
    } as unknown as GalleryNativeJson;

    const report = analyzeFrameFidelity(nativeJson, nativeJson.frames[0]!);

    expect(report.native).toBe(1);
    expect(report.renderQualityPercent).toBe(100);
    expect(report.byLayerId.photo!.reason).toContain("图片填充");
  });

  it("does not penalize transparent utility shapes without visual output", () => {
    const nativeJson = {
      version: "1.0",
      file: { id: "file", title: "Test", sourceType: "figma" },
      frames: [{ id: "frame-1", name: "Frame", rootLayerId: "root", width: 100, height: 100 }],
      layers: {
        root: { id: "root", frameId: "frame-1", name: "Root", type: "frame", visible: true, x: 0, y: 0, width: 100, height: 100, children: ["mask"] },
        mask: { id: "mask", frameId: "frame-1", parentId: "root", name: "蒙版", type: "shape", visible: true, x: 0, y: 0, width: 40, height: 40, style: { opacity: 1, cornerRadius: 0 } },
      },
      assets: {},
    } as unknown as GalleryNativeJson;

    const report = analyzeFrameFidelity(nativeJson, nativeJson.frames[0]!);

    expect(report.unsupported).toBe(0);
    expect(report.renderQualityPercent).toBe(100);
    expect(report.byLayerId.mask!.reason).toContain("透明");
  });
});
