figma.showUI(__html__, { width: 460, height: 700, themeColors: true });

figma.clientStorage.getAsync('multicaFigmaToken').then((token) => {
  figma.ui.postMessage({ type: 'stored-token', token: token || '' });
  postSelectionSummary('selected');
});

function safeName(node) {
  return String(node && node.name ? node.name : "Untitled").slice(0, 160);
}

function nodeType(node) {
  const type = String(node.type || "GROUP").toLowerCase();
  if (type === "text") return "text";
  if (type === "component") return "component";
  if (type === "instance") return "instance";
  if (type === "frame" || type === "page" || type === "section") return "frame";
  if (type === "vector") return "vector";
  if (type === "slice") return "slice";
  if (type === "rectangle" || type === "ellipse" || type === "line" || type === "polygon" || type === "star") return "shape";
  return "group";
}

function cleanId(raw) {
  return String(raw || "unknown").replace(/[^a-zA-Z0-9_-]/g, "-");
}

function rect(node, fallback, frameBox) {
  const box = node && node.absoluteBoundingBox ? node.absoluteBoundingBox : node;
  const x = typeof box.x === "number" ? box.x : fallback.x;
  const y = typeof box.y === "number" ? box.y : fallback.y;
  return {
    x: frameBox ? x - frameBox.x : x,
    y: frameBox ? y - frameBox.y : y,
    width: typeof box.width === "number" ? Math.max(box.width, 0) : fallback.width,
    height: typeof box.height === "number" ? Math.max(box.height, 0) : fallback.height,
  };
}

function colorValue(color, opacity) {
  if (!color) return undefined;
  const alpha = typeof opacity === "number" ? opacity : 1;
  const r = Math.round(color.r * 255);
  const g = Math.round(color.g * 255);
  const b = Math.round(color.b * 255);
  const hex = `#${[r, g, b].map((n) => n.toString(16).padStart(2, "0")).join("")}`.toUpperCase();
  return { r, g, b, a: alpha, hex, css: `rgba(${r}, ${g}, ${b}, ${Number(alpha.toFixed(3))})` };
}

function paintValue(paint, assets, owner) {
  if (!paint || paint.visible === false) return undefined;
  if (paint.type === "SOLID") {
    return { type: "solid", color: colorValue(paint.color, paint.opacity), visible: paint.visible !== false, opacity: paint.opacity };
  }
  if (paint.type === "IMAGE") {
    const imageHash = paint.imageHash || undefined;
    const assetId = imageHash ? `image-${cleanId(imageHash)}` : undefined;
    if (assetId && !assets[assetId]) {
      assets[assetId] = {
        id: assetId,
        kind: "image",
        url: `figma-image-hash://${imageHash}`,
        sourceNodeId: owner && owner.id,
        metadata: { imageHash, scaleMode: paint.scaleMode, note: "Figma image fill reference; binary extraction will move to design_asset storage later" },
      };
    }
    return { type: "image", assetId, imageHash, scaleMode: paint.scaleMode, visible: paint.visible !== false, opacity: paint.opacity };
  }
  if (String(paint.type || "").startsWith("GRADIENT")) {
    return {
      type: "gradient",
      gradientType: paint.type,
      stops: (paint.gradientStops || []).map((stop) => ({ position: stop.position, color: colorValue(stop.color, stop.color && stop.color.a) })),
      visible: paint.visible !== false,
      opacity: paint.opacity,
    };
  }
  return undefined;
}

function paintsValue(paints, assets, owner) {
  if (!Array.isArray(paints)) return undefined;
  const mapped = paints.map((paint) => paintValue(paint, assets, owner)).filter(Boolean);
  return mapped.length > 0 ? mapped : undefined;
}

function strokesValue(node) {
  if (!node || !Array.isArray(node.strokes)) return undefined;
  const width = typeof node.strokeWeight === "number" ? node.strokeWeight : undefined;
  const dashPattern = Array.isArray(node.dashPattern) && node.dashPattern.length > 0 ? node.dashPattern : undefined;
  const mapped = node.strokes
    .filter((stroke) => stroke && stroke.type === "SOLID" && stroke.visible !== false)
    .map((stroke) => ({
      color: colorValue(stroke.color, stroke.opacity),
      width: width || 1,
      position: String(node.strokeAlign || "CENTER").toLowerCase(),
      cap: typeof node.strokeCap === "string" ? node.strokeCap.toLowerCase() : undefined,
      join: typeof node.strokeJoin === "string" ? node.strokeJoin.toLowerCase() : undefined,
      dashPattern,
    }));
  return mapped.length > 0 ? mapped : undefined;
}

function shadowsValue(node) {
  if (!node || !Array.isArray(node.effects)) return undefined;
  const mapped = node.effects
    .filter((effect) => effect && effect.visible !== false && (effect.type === "DROP_SHADOW" || effect.type === "INNER_SHADOW"))
    .map((effect) => ({
      type: effect.type === "INNER_SHADOW" ? "inner" : "drop",
      color: colorValue(effect.color, effect.color && effect.color.a),
      offsetX: effect.offset && typeof effect.offset.x === "number" ? effect.offset.x : 0,
      offsetY: effect.offset && typeof effect.offset.y === "number" ? effect.offset.y : 0,
      blur: typeof effect.radius === "number" ? effect.radius : 0,
      spread: typeof effect.spread === "number" ? effect.spread : undefined,
    }));
  return mapped.length > 0 ? mapped : undefined;
}

function radiusValue(node) {
  if (!node) return undefined;
  if (typeof node.cornerRadius === "number") return node.cornerRadius;
  const radii = [node.topLeftRadius, node.topRightRadius, node.bottomRightRadius, node.bottomLeftRadius];
  if (radii.some((value) => typeof value === "number")) return radii.map((value) => (typeof value === "number" ? value : 0));
  return undefined;
}

function textValue(node, assets) {
  if (!node || node.type !== "TEXT" || typeof node.characters !== "string") return undefined;
  const fills = paintsValue(node.fills, assets, node);
  const text = { characters: node.characters.slice(0, 5000) };
  if (node.fontName && node.fontName !== figma.mixed) {
    text.fontFamily = node.fontName.family;
    text.fontStyle = node.fontName.style;
  }
  if (typeof node.fontSize === "number") text.fontSize = node.fontSize;
  if (typeof node.fontWeight === "number") text.fontWeight = node.fontWeight;
  if (node.lineHeight && node.lineHeight !== figma.mixed) {
    if (node.lineHeight.unit === "AUTO") text.lineHeight = "AUTO";
    if (node.lineHeight.unit === "PIXELS") text.lineHeight = node.lineHeight.value;
    if (node.lineHeight.unit === "PERCENT" && typeof node.fontSize === "number") text.lineHeight = (node.fontSize * node.lineHeight.value) / 100;
  }
  if (node.letterSpacing && node.letterSpacing !== figma.mixed && node.letterSpacing.unit === "PIXELS") text.letterSpacing = node.letterSpacing.value;
  if (typeof node.textAlignHorizontal === "string") text.textAlignHorizontal = node.textAlignHorizontal.toLowerCase();
  if (typeof node.textAlignVertical === "string") text.textAlignVertical = node.textAlignVertical.toLowerCase();
  if (typeof node.textAutoResize === "string") text.textAutoResize = node.textAutoResize.toLowerCase();
  if (typeof node.paragraphSpacing === "number") text.paragraphSpacing = node.paragraphSpacing;
  if (typeof node.paragraphIndent === "number") text.paragraphIndent = node.paragraphIndent;
  if (typeof node.textCase === "string") text.textCase = node.textCase.toLowerCase();
  if (typeof node.textDecoration === "string") text.textDecoration = node.textDecoration.toLowerCase();
  if (fills && fills[0] && fills[0].type === "solid") text.color = fills[0].color;
  return text;
}

function layerStyle(node, assets) {
  const style = {};
  const fills = paintsValue(node.fills, assets, node);
  const strokes = strokesValue(node);
  const shadows = shadowsValue(node);
  const cornerRadius = radiusValue(node);
  if (fills) style.fills = fills;
  if (strokes) style.strokes = strokes;
  if (shadows) style.shadows = shadows;
  if (cornerRadius !== undefined) style.cornerRadius = cornerRadius;
  if (typeof node.opacity === "number") style.opacity = node.opacity;
  if (typeof node.blendMode === "string") style.blendMode = node.blendMode;
  if (node.constraints) style.constraints = node.constraints;
  if (node.layoutMode) {
    style.autoLayout = {
      layoutMode: node.layoutMode,
      primaryAxisSizingMode: node.primaryAxisSizingMode,
      counterAxisSizingMode: node.counterAxisSizingMode,
      primaryAxisAlignItems: node.primaryAxisAlignItems,
      counterAxisAlignItems: node.counterAxisAlignItems,
      itemSpacing: node.itemSpacing,
      paddingLeft: node.paddingLeft,
      paddingRight: node.paddingRight,
      paddingTop: node.paddingTop,
      paddingBottom: node.paddingBottom,
    };
  }
  return Object.keys(style).length > 0 ? style : undefined;
}

function exportableValue(node) {
  if (!node || !Array.isArray(node.exportSettings) || node.exportSettings.length === 0) return undefined;
  return node.exportSettings.map((setting, index) => ({
    id: `slice-${cleanId(node.id)}-${index}`,
    assetId: `slice-${cleanId(node.id)}-${index}`,
    kind: "slice",
    format: setting.format,
    suffix: setting.suffix,
    constraint: setting.constraint,
  }));
}

function buildLayer(node, frameId, parentId, layers, frameBox, assets) {
  const box = rect(node, { x: 0, y: 0, width: 1, height: 1 }, frameBox);
  const id = cleanId(node.id);
  const layer = {
    id,
    sourceNodeId: node.id,
    frameId,
    parentId: parentId || undefined,
    name: safeName(node),
    type: nodeType(node),
    visible: node.visible !== false,
    x: box.x,
    y: box.y,
    width: box.width,
    height: box.height,
  };

  if (typeof node.rotation === "number") layer.rotation = node.rotation;
  if (typeof node.opacity === "number") layer.opacity = node.opacity;
  const style = layerStyle(node, assets);
  if (style) layer.style = style;
  const text = textValue(node, assets);
  if (text) {
    layer.text = text;
    layer.semantic = { role: "text" };
  }
  if (layer.type === "shape") {
    const shapeType = String(node.type || "rectangle").toLowerCase();
    layer.shape = { shapeType: shapeType === "ellipse" ? "ellipse" : shapeType === "line" ? "line" : shapeType === "rectangle" ? "rectangle" : "custom" };
  }
  if (style && style.fills && style.fills.some((fill) => fill.type === "image" && fill.assetId)) {
    const fill = style.fills.find((item) => item.type === "image" && item.assetId);
    layer.image = { assetId: fill.assetId, alt: safeName(node) };
  }
  const exportable = exportableValue(node);
  if (exportable) layer.exportable = exportable;
  layer.source = {
    tool: "figma",
    nodeId: node.id,
    nodeType: node.type,
    componentId: node.componentId,
    mainComponentId: node.type === "INSTANCE" && node.mainComponent ? node.mainComponent.id : undefined,
    layoutMode: node.layoutMode,
    clipsContent: node.clipsContent,
    isMask: node.isMask,
  };

  if ("children" in node && node.children && node.children.length > 0) {
    layer.children = node.children.map((child) => cleanId(child.id));
  }

  layers[id] = layer;
  if ("children" in node && node.children) {
    for (const child of node.children) buildLayer(child, frameId, id, layers, frameBox, assets);
  }
  return id;
}

function isExportableRoot(node) {
  return !!node && ["FRAME", "COMPONENT", "INSTANCE", "SECTION", "GROUP"].includes(node.type);
}

function rootsForScope(scope) {
  if (scope === 'page') return (figma.currentPage.children || []).filter(isExportableRoot);
  const selected = figma.currentPage.selection || [];
  return selected.filter(isExportableRoot);
}

function nodeSummary(node) {
  const box = rect(node, { x: 0, y: 0, width: 0, height: 0 });
  return { id: node.id, name: safeName(node), type: node.type, width: box.width, height: box.height, x: box.x, y: box.y };
}

function selectionSummary(scope) {
  const selected = (figma.currentPage.selection || []).filter(isExportableRoot).map(nodeSummary);
  const page = (figma.currentPage.children || []).filter(isExportableRoot).map(nodeSummary);
  const active = rootsForScope(scope).map(nodeSummary);
  return { pageName: figma.currentPage.name, scope: scope || 'selected', selected, page, active };
}

function postSelectionSummary(scope) {
  figma.ui.postMessage({ type: 'selection-changed', summary: selectionSummary(scope) });
}

function buildSourceKey(scope, roots) {
  const fileKey = figma.fileKey || 'local-file';
  const nodeIds = roots.map((root) => String(root.id)).sort().join(',');
  return `figma:${fileKey}:page:${figma.currentPage.id}:scope:${scope || 'selected'}:nodes:${nodeIds}`;
}

figma.on('selectionchange', () => postSelectionSummary('selected'));

async function exportFrameAsset(node, frameId, kind, constraint) {
  if (!node || typeof node.exportAsync !== 'function') return undefined;
  try {
    const bytes = await node.exportAsync({ format: 'PNG', constraint });
    return {
      id: `${kind}-${frameId}`,
      kind,
      url: '',
      contentType: 'image/png',
      width: typeof node.width === 'number' ? node.width : undefined,
      height: typeof node.height === 'number' ? node.height : undefined,
      sizeBytes: bytes.length,
      sourceNodeId: node.id,
      frameId,
      bytes: Array.from(bytes),
      metadata: { exportFormat: 'PNG', constraint },
    };
  } catch (_error) {
    return undefined;
  }
}

async function exportNativeJson(scope) {
  const roots = rootsForScope(scope);
  if (roots.length === 0) throw new Error(scope === 'page' ? '当前页面没有可上传画板' : '请先在 Figma 中选中要上传的画板');
  const sourceKey = buildSourceKey(scope, roots);
  const layers = {};
  const assets = {};
  const frames = [];
  const selectedNames = roots.map(safeName);
  const title = selectedNames.length === 1 ? selectedNames[0] : `${figma.currentPage.name} (${roots.length} selections)`;

  for (let index = 0; index < roots.length; index += 1) {
    const root = roots[index];
    figma.ui.postMessage({ type: 'export-progress', stage: 'read', current: index + 1, total: roots.length, name: safeName(root) });
    const frameId = `frame-${index + 1}`;
    const box = rect(root, { x: 0, y: 0, width: 1440, height: 900 });
    const frameBox = root.absoluteBoundingBox || { x: box.x, y: box.y, width: box.width, height: box.height };
    const rootLayerId = buildLayer(root, frameId, undefined, layers, frameBox, assets);
    const previewAsset = await exportFrameAsset(root, frameId, 'frame_preview', { type: 'SCALE', value: 1 });
    const thumbnailAsset = await exportFrameAsset(root, frameId, 'frame_thumbnail', { type: 'WIDTH', value: 600 });
    if (previewAsset) assets[previewAsset.id] = previewAsset;
    if (thumbnailAsset) assets[thumbnailAsset.id] = thumbnailAsset;
    frames.push({
      id: frameId,
      sourceNodeId: root.id,
      name: safeName(root),
      rootLayerId,
      x: box.x,
      y: box.y,
      width: box.width,
      height: box.height,
      previewAssetId: previewAsset && previewAsset.id,
      thumbnailAssetId: thumbnailAsset && thumbnailAsset.id,
      board: { x: box.x, y: box.y, order: index },
    });
  }

  if (frames.length === 0) {
    const rootLayerId = "empty-root";
    layers[rootLayerId] = { id: rootLayerId, frameId: "frame-1", name: "Empty page", type: "frame", visible: true, x: 0, y: 0, width: 1440, height: 900 };
    frames.push({ id: "frame-1", name: safeName(figma.currentPage), rootLayerId, width: 1440, height: 900 });
  }

  return {
    version: "1.0",
    file: {
      title,
      sourceType: "import",
      sourceId: figma.fileKey || undefined,
      sourceTool: "figma",
      sourceKey,
      importedAt: new Date().toISOString(),
    },
    frames,
    layers,
    assets,
    slots: {},
    componentBindings: {},
    source: {
      tool: 'figma',
      fileKey: figma.fileKey || undefined,
      pageId: figma.currentPage.id,
      pageName: figma.currentPage.name,
      scope: scope || 'selected',
      sourceKey,
      nodeIds: roots.map((root) => root.id),
    },
  };
}

function exportFormat(format) {
  const value = String(format || 'PNG').toUpperCase();
  if (value === 'JPG' || value === 'JPEG') return 'JPG';
  if (value === 'SVG') return 'SVG';
  if (value === 'PDF') return 'PDF';
  return 'PNG';
}

function fileExtension(format) {
  const value = exportFormat(format).toLowerCase();
  return value === 'jpg' ? 'jpg' : value;
}

async function collectSliceUploads(nativeJson) {
  const uploads = [];
  const layers = Object.values(nativeJson.layers || {});
  for (const layer of layers) {
    if (!Array.isArray(layer.exportable) || !layer.exportable.length || !layer.sourceNodeId) continue;
    const node = await figma.getNodeByIdAsync(layer.sourceNodeId);
    if (!node || typeof node.exportAsync !== 'function') continue;
    for (let index = 0; index < layer.exportable.length; index += 1) {
      const item = layer.exportable[index];
      const format = exportFormat(item.format);
      if (format === 'PDF') continue;
      try {
        const bytes = await node.exportAsync({ format, constraint: item.constraint });
        uploads.push({
          assetId: item.assetId || item.id || `slice-${cleanId(layer.sourceNodeId)}-${index}`,
          layerId: layer.id,
          frameId: layer.frameId,
          sourceNodeId: layer.sourceNodeId,
          name: `${safeName({ name: layer.name })}${item.suffix || ''}.${fileExtension(format)}`,
          format: fileExtension(format),
          contentType: format === 'SVG' ? 'image/svg+xml' : format === 'JPG' ? 'image/jpeg' : 'image/png',
          width: layer.width,
          height: layer.height,
          bytes: Array.from(bytes),
        });
      } catch (error) {
        console.warn('[multica-figma] slice export failed', layer.name, error);
      }
    }
  }
  return uploads;
}

async function collectImageFillUploads(nativeJson) {
  const uploads = [];
  const assets = Object.values(nativeJson.assets || {});
  for (const asset of assets) {
    const imageHash = asset && asset.metadata && asset.metadata.imageHash;
    if (!imageHash || !String(asset.url || '').startsWith('figma-image-hash://')) continue;
    try {
      const image = figma.getImageByHash(imageHash);
      if (!image || typeof image.getBytesAsync !== 'function') continue;
      const bytes = await image.getBytesAsync();
      uploads.push({
        assetId: asset.id,
        kind: 'image',
        name: `${asset.id || 'figma-image-fill'}.png`,
        format: 'png',
        contentType: 'image/png',
        width: asset.width,
        height: asset.height,
        bytes: Array.from(bytes),
        sourceNodeId: asset.sourceNodeId,
        metadata: { ...(asset.metadata || {}), exportedFromImageHash: true },
      });
    } catch (error) {
      console.warn('[multica-figma] image fill export failed', asset.id, error);
    }
  }
  return uploads;
}

figma.ui.onmessage = (message) => {
  if (!message) return;
  if (message.type === 'open-auth' && message.url) {
    figma.openExternal(message.url);
    return;
  }
  if (message.type === 'store-token') {
    figma.clientStorage.setAsync('multicaFigmaToken', message.token || '').then(() => {
      figma.ui.postMessage({ type: 'stored-token', token: message.token || '' });
    });
    return;
  }
  if (message.type === 'logout') {
    figma.clientStorage.setAsync('multicaFigmaToken', '').then(() => {
      figma.ui.postMessage({ type: 'stored-token', token: '' });
    });
    return;
  }
  if (message.type === 'get-selection') {
    postSelectionSummary(message.scope || 'selected');
    return;
  }
  if (message.type !== "export") return;
  try {
    exportNativeJson(message.scope || 'selected').then((nativeJson) => {
      Promise.all([collectImageFillUploads(nativeJson), collectSliceUploads(nativeJson)]).then(([imageUploads, sliceUploads]) => {
        figma.ui.postMessage({ type: "exported", nativeJson, title: nativeJson.file.title, sliceUploads: [...imageUploads, ...sliceUploads] });
      }).catch((error) => {
        figma.ui.postMessage({ type: "error", error: error instanceof Error ? error.message : String(error) });
      });
    }).catch((error) => {
      figma.ui.postMessage({ type: "error", error: error instanceof Error ? error.message : String(error) });
    });
  } catch (error) {
    figma.ui.postMessage({ type: "error", error: error instanceof Error ? error.message : String(error) });
  }
};
