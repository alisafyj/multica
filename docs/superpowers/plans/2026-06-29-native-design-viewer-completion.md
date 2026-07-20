# Native Design Viewer Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the Native Design Viewer into a usable handoff surface with safe light editing, more honest render quality, and file-level quality visibility.

**Architecture:** Extend the existing lightweight edit API and Native Viewer UI rather than adding a full editor. Keep persistence in `native_json.source` for edit history and fidelity summaries; derive file/frame quality from native JSON until query-scale needs justify new tables.

**Tech Stack:** Go handlers/tests, PostgreSQL-backed design revisions, Next.js/React views, TypeScript shared types, native-renderer fidelity helpers, existing upload/assets APIs.

---

## File Structure

- `packages/core/types/design.ts`: add lightweight edit request fields for stroke edits, image replacement, and undo.
- `server/internal/handler/design_file.go`: validate and apply new lightweight edits; append edit history; support undo/revert.
- `server/internal/handler/design_file_test.go`: focused backend tests for stroke edits, image replacement payloads, undo, and validation failures.
- `server/internal/handler/design_fidelity.go`: keep persisted render-quality summaries consistent with client scoring.
- `packages/views/designs/design-frame-page.tsx`: inspector controls for image, stroke, undo, and file/frame quality entry points.
- `packages/views/designs/native-renderer/fidelity.ts`: classification and scoring helpers used by active frame UI.
- `packages/views/designs/native-renderer/style.ts`: renderer helpers for strokes, image fills, fallback/crop classification.
- `packages/views/designs/design-file-page.tsx`: owns the design board and frame list; add file/frame quality summary here.
- `packages/views/designs/components/design-quality-summary.tsx`: focused summary component for average quality and low-quality frames.

## Task 1: Add Stroke Lightweight Edit API

**Files:**
- Modify: `packages/core/types/design.ts`
- Modify: `server/internal/handler/design_file.go`
- Test: `server/internal/handler/design_file_test.go`

- [ ] **Step 1: Add failing backend test for stroke color and width**

Add this test near `TestUpdateDesignLayerLightweightFillAndTextColor` in `server/internal/handler/design_file_test.go`:

```go
func TestUpdateDesignLayerLightweightStrokeColorAndWidth(t *testing.T) {
	created := createDesignFileForTest(t, "Lightweight Stroke Edit Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	nativeJSON := contextDesignNativeJSON("Lightweight Stroke Edit Design")
	layers := nativeJSON["layers"].(map[string]any)
	layers["main-image"].(map[string]any)["style"] = map[string]any{"strokes": []map[string]any{{"color": map[string]any{"css": "#111111", "hex": "#111111"}, "width": float64(1)}}}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, nativeJSON)

	w := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-image", map[string]any{
		"revision_id":  created.CurrentRevision.ID,
		"stroke_color": "#abc",
		"stroke_width": 2,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateDesignLayerLightweight stroke edit: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	doc := decodeDesignRevisionNativeJSONForTest(t, resp.CurrentRevision.NativeJSON)
	style := layerFromNativeJSONForTest(t, doc, "main-image")["style"].(map[string]any)
	strokes := style["strokes"].([]any)
	stroke := strokes[0].(map[string]any)
	color := stroke["color"].(map[string]any)
	if color["hex"] != "#AABBCC" || color["css"] != "#AABBCC" {
		t.Fatalf("stroke color = %+v, want #AABBCC", color)
	}
	if stroke["width"] != float64(2) {
		t.Fatalf("stroke width = %v, want 2", stroke["width"])
	}
	assertLightweightEditChangedFieldsForTest(t, lastLightweightEditFromNativeJSONForTest(t, doc), []string{"stroke_color", "stroke_width"})
}
```

- [ ] **Step 2: Run test and verify it fails**

Run:

```bash
go test ./internal/handler -run 'TestUpdateDesignLayerLightweightStrokeColorAndWidth' -count=1
```

Expected: FAIL because `stroke_color` and `stroke_width` are not handled.

- [ ] **Step 3: Add shared TypeScript request fields**

In `packages/core/types/design.ts`, extend `DesignLayerLightweightEditRequest`:

```ts
stroke_color?: string;
stroke_width?: number;
```

- [ ] **Step 4: Add Go request fields and stroke appliers**

In `server/internal/handler/design_file.go`, extend `DesignLayerLightweightEditRequest`:

```go
StrokeColor *string  `json:"stroke_color"`
StrokeWidth *float64 `json:"stroke_width"`
```

Add helpers near `applyLayerFillColor`:

```go
func applyLayerStrokeColor(layer map[string]any, color map[string]any) {
	style, _ := layer["style"].(map[string]any)
	if style == nil {
		style = map[string]any{}
		layer["style"] = style
	}
	strokes, _ := style["strokes"].([]any)
	if len(strokes) == 0 {
		strokes = []any{map[string]any{"width": float64(1)}}
	}
	stroke, _ := strokes[0].(map[string]any)
	if stroke == nil {
		stroke = map[string]any{"width": float64(1)}
		strokes[0] = stroke
	}
	stroke["color"] = color
	style["strokes"] = strokes
	if _, ok := style["stroke"]; ok {
		style["stroke"] = color
	}
}

func applyLayerStrokeWidth(layer map[string]any, width float64) error {
	if width < 0 || width > 100 {
		return errBadRequest("stroke_width must be between 0 and 100")
	}
	style, _ := layer["style"].(map[string]any)
	if style == nil {
		style = map[string]any{}
		layer["style"] = style
	}
	strokes, _ := style["strokes"].([]any)
	if len(strokes) == 0 {
		strokes = []any{map[string]any{}}
	}
	stroke, _ := strokes[0].(map[string]any)
	if stroke == nil {
		stroke = map[string]any{}
		strokes[0] = stroke
	}
	stroke["width"] = width
	style["strokes"] = strokes
	return nil
}
```

Inside `applyDesignLayerLightweightEdit`, after color handling:

```go
if req.StrokeColor != nil {
	color, err := parseLightweightHexColor(*req.StrokeColor)
	if err != nil {
		return nil, false, nil, err
	}
	applyLayerStrokeColor(layer, color)
	changed = true
	changedFields = append(changedFields, "stroke_color")
}
if req.StrokeWidth != nil {
	if err := applyLayerStrokeWidth(layer, *req.StrokeWidth); err != nil {
		return nil, false, nil, err
	}
	changed = true
	changedFields = append(changedFields, "stroke_width")
}
```

- [ ] **Step 5: Run focused test**

Run:

```bash
go test ./internal/handler -run 'TestUpdateDesignLayerLightweightStrokeColorAndWidth' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/core/types/design.ts server/internal/handler/design_file.go server/internal/handler/design_file_test.go
git commit -m "feat: add lightweight stroke editing"
```

## Task 2: Add Stroke Controls to Inspector

**Files:**
- Modify: `packages/views/designs/design-frame-page.tsx`

- [ ] **Step 1: Add state and current stroke helpers**

In `design-frame-page.tsx`, add helper:

```ts
function primaryStroke(layer: DesignLayer | null) {
  if (!layer) return { color: "", width: "" };
  const stroke = styleArray<Stroke>(layer.style, "strokes")[0];
  return { color: hexColor(stroke?.color ?? layer.style?.stroke ?? layer.style?.borderColor), width: stroke?.width !== undefined ? String(stroke.width) : "" };
}
```

Add state near existing edit color state:

```ts
const [editStrokeColor, setEditStrokeColor] = useState("");
const [editStrokeWidth, setEditStrokeWidth] = useState("");
```

Derive current values:

```ts
const currentStroke = primaryStroke(selectedLayer);
```

Extend `hasLayerEditChanges` with:

```ts
|| (!!editStrokeColor && editStrokeColor !== currentStroke.color)
|| (!!editStrokeWidth && editStrokeWidth !== currentStroke.width)
```

In the selected-layer `useEffect`, set:

```ts
const stroke = primaryStroke(selectedLayer);
setEditStrokeColor(stroke.color);
setEditStrokeWidth(stroke.width);
```

- [ ] **Step 2: Send stroke edits in mutation payload**

Add to `api.updateDesignLayerLightweight` payload:

```ts
stroke_color: editStrokeColor && editStrokeColor !== currentStroke.color ? editStrokeColor : undefined,
stroke_width: editStrokeWidth && editStrokeWidth !== currentStroke.width ? Number(editStrokeWidth) : undefined,
```

- [ ] **Step 3: Add inspector controls**

Add this below fill/text color controls:

```tsx
<div className="grid grid-cols-2 gap-2">
  <label className="space-y-1.5 rounded-lg border p-2 text-xs">
    <span className="font-medium text-muted-foreground">描边色</span>
    <div className="flex items-center gap-2">
      <input type="color" value={editStrokeColor || "#000000"} disabled={!selectedLayer || selectedLayer.id === frame.rootLayerId} className="h-7 w-9 rounded border bg-transparent" onChange={(event) => setEditStrokeColor(event.target.value.toUpperCase())} />
      <span className="font-mono text-[11px] text-muted-foreground">{editStrokeColor || "—"}</span>
    </div>
  </label>
  <label className="space-y-1.5 rounded-lg border p-2 text-xs">
    <span className="font-medium text-muted-foreground">描边宽度</span>
    <Input value={editStrokeWidth} disabled={!selectedLayer || selectedLayer.id === frame.rootLayerId} inputMode="decimal" placeholder="0" onChange={(event) => setEditStrokeWidth(event.target.value)} />
  </label>
</div>
```

- [ ] **Step 4: Typecheck**

Run:

```bash
pnpm --filter @multica/views exec tsc --noEmit --pretty false
pnpm --filter @multica/web exec tsc --noEmit --pretty false
```

Expected: both exit 0.

- [ ] **Step 5: Commit**

```bash
git add packages/views/designs/design-frame-page.tsx
git commit -m "feat: add stroke controls to native viewer"
```

## Task 3: Add Lightweight Edit Undo/Revert

**Files:**
- Modify: `packages/core/types/design.ts`
- Modify: `server/internal/handler/design_file.go`
- Test: `server/internal/handler/design_file_test.go`
- Modify: `packages/views/designs/design-frame-page.tsx`

- [ ] **Step 1: Add failing backend test for undo**

Add test:

```go
func TestUpdateDesignLayerLightweightUndoLastEdit(t *testing.T) {
	created := createDesignFileForTest(t, "Lightweight Undo Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}

	editW := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-title", map[string]any{
		"revision_id": created.CurrentRevision.ID,
		"name":        "Edited title",
	})
	if editW.Code != http.StatusOK {
		t.Fatalf("edit expected 200, got %d: %s", editW.Code, editW.Body.String())
	}
	var editResp DesignFileDetailResponse
	if err := json.NewDecoder(editW.Body).Decode(&editResp); err != nil {
		t.Fatalf("decode edit response: %v", err)
	}

	undoW := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-title", map[string]any{
		"revision_id": editResp.CurrentRevision.ID,
		"undo_last":   true,
	})
	if undoW.Code != http.StatusOK {
		t.Fatalf("undo expected 200, got %d: %s", undoW.Code, undoW.Body.String())
	}
	var undoResp DesignFileDetailResponse
	if err := json.NewDecoder(undoW.Body).Decode(&undoResp); err != nil {
		t.Fatalf("decode undo response: %v", err)
	}
	doc := decodeDesignRevisionNativeJSONForTest(t, undoResp.CurrentRevision.NativeJSON)
	layer := layerFromNativeJSONForTest(t, doc, "main-title")
	if layer["name"] == "Edited title" {
		t.Fatalf("expected undo to restore previous name")
	}
}
```

- [ ] **Step 2: Run test and verify failure**

```bash
go test ./internal/handler -run 'TestUpdateDesignLayerLightweightUndoLastEdit' -count=1
```

Expected: FAIL because `undo_last` is unsupported.

- [ ] **Step 3: Store before snapshots in edit history**

In `applyDesignLayerLightweightEdit`, before mutating a layer, copy the layer:

```go
beforeLayer := cloneJSONMap(layer)
```

When appending the lightweight edit metadata, include:

```go
"before": beforeLayer,
"after": layer,
```

Add helper:

```go
func cloneJSONMap(input map[string]any) map[string]any {
	raw, _ := json.Marshal(input)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
```

- [ ] **Step 4: Add undo request support**

Add field:

```go
UndoLast *bool `json:"undo_last"`
```

At start of `applyDesignLayerLightweightEdit`, if `UndoLast` is true, find the most recent `source.lightweightEdits` item for `layerID` with a `before` object and replace `layers[layerID]` with it. Append a new history item with `changedFields: ["undo_last"]`.

- [ ] **Step 5: Add UI button**

In inspector edit section, if `editSummary` exists, show:

```tsx
<Button size="sm" variant="outline" disabled={!selectedLayer || editMutation.isPending} onClick={() => editMutation.mutate({ undo_last: true })}>撤销上次轻编辑</Button>
```

If mutation typing does not accept this payload directly, extend the shared request type first.

- [ ] **Step 6: Run tests and typechecks**

```bash
go test ./internal/handler -run 'TestUpdateDesignLayerLightweightUndoLastEdit|TestUpdateDesignLayerLightweight' -count=1
pnpm --filter @multica/views exec tsc --noEmit --pretty false
pnpm --filter @multica/web exec tsc --noEmit --pretty false
```

Expected: all exit 0.

- [ ] **Step 7: Commit**

```bash
git add packages/core/types/design.ts server/internal/handler/design_file.go server/internal/handler/design_file_test.go packages/views/designs/design-frame-page.tsx
git commit -m "feat: add lightweight edit undo"
```

## Task 4: Add Image Replacement by URL

**Files:**
- Modify: `packages/core/types/design.ts`
- Modify: `server/internal/handler/design_file.go`
- Test: `server/internal/handler/design_file_test.go`
- Modify: `packages/views/designs/design-frame-page.tsx`

- [ ] **Step 1: Add failing backend test for image replacement**

Add this test near the other lightweight edit tests:

```go
func TestUpdateDesignLayerLightweightImageURL(t *testing.T) {
	created := createDesignFileForTest(t, "Lightweight Image Replace Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	nativeJSON := contextDesignNativeJSON("Lightweight Image Replace Design")
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, nativeJSON)

	w := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-image", map[string]any{
		"revision_id": created.CurrentRevision.ID,
		"image_url":   "https://example.com/replacement.png",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("image_url edit expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	doc := decodeDesignRevisionNativeJSONForTest(t, resp.CurrentRevision.NativeJSON)
	layer := layerFromNativeJSONForTest(t, doc, "main-image")
	image := layer["image"].(map[string]any)
	assetID := image["assetId"].(string)
	assets := doc["assets"].(map[string]any)
	asset := assets[assetID].(map[string]any)
	if asset["url"] != "https://example.com/replacement.png" {
		t.Fatalf("asset url = %v", asset["url"])
	}
	assertLightweightEditChangedFieldsForTest(t, lastLightweightEditFromNativeJSONForTest(t, doc), []string{"image_url"})
}
```

- [ ] **Step 2: Run test and verify it fails**

```bash
go test ./internal/handler -run 'TestUpdateDesignLayerLightweightImageURL' -count=1
```

Expected: FAIL because `image_url` is unsupported.

- [ ] **Step 3: Add shared request field**

In `packages/core/types/design.ts`, add:

```ts
image_url?: string;
```

- [ ] **Step 4: Add server request field and validator**

In `DesignLayerLightweightEditRequest`, add:

```go
ImageURL *string `json:"image_url"`
```

Add helper near lightweight edit helpers:

```go
func parseLightweightImageURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errBadRequest("image_url is required")
	}
	if !(strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "/uploads/")) {
		return "", errBadRequest("image_url must be http(s) or an uploaded asset path")
	}
	return value, nil
}
```

- [ ] **Step 5: Apply image URL to layer and assets**

Add helper:

```go
func applyLayerImageURL(doc map[string]any, layerID string, layer map[string]any, imageURL string) error {
	if stringField(layer, "type") != "image" && !layerHasImageFillForEdit(layer) {
		return errBadRequest("image_url edits are only allowed on image layers or image-fill layers")
	}
	assets, _ := doc["assets"].(map[string]any)
	if assets == nil {
		assets = map[string]any{}
		doc["assets"] = assets
	}
	image, _ := layer["image"].(map[string]any)
	if image == nil {
		image = map[string]any{}
		layer["image"] = image
	}
	assetID := strings.TrimSpace(stringAny(image["assetId"]))
	if assetID == "" {
		assetID = "manual-image-" + strings.ReplaceAll(layerID, ":", "-")
		image["assetId"] = assetID
	}
	assets[assetID] = map[string]any{"id": assetID, "kind": "image", "url": imageURL, "sourceNodeId": stringAny(layer["sourceNodeId"]), "frameId": stringAny(layer["frameId"])}
	style, _ := layer["style"].(map[string]any)
	if style == nil {
		style = map[string]any{}
		layer["style"] = style
	}
	fills, _ := style["fills"].([]any)
	if len(fills) == 0 {
		fills = []any{map[string]any{"type": "image"}}
	}
	fill, _ := fills[0].(map[string]any)
	if fill == nil {
		fill = map[string]any{"type": "image"}
		fills[0] = fill
	}
	fill["type"] = "image"
	fill["assetId"] = assetID
	style["fills"] = fills
	return nil
}

func layerHasImageFillForEdit(layer map[string]any) bool {
	style, _ := layer["style"].(map[string]any)
	for _, fill := range objectSliceFromAny(style["fills"]) {
		if stringAny(fill["type"]) == "image" || stringAny(fill["assetId"]) != "" || stringAny(fill["imageHash"]) != "" {
			return true
		}
	}
	return layer["image"] != nil
}
```

Inside `applyDesignLayerLightweightEdit`, add:

```go
if req.ImageURL != nil {
	imageURL, err := parseLightweightImageURL(*req.ImageURL)
	if err != nil {
		return nil, false, nil, err
	}
	if err := applyLayerImageURL(doc, layerID, layer, imageURL); err != nil {
		return nil, false, nil, err
	}
	changed = true
	changedFields = append(changedFields, "image_url")
}
```

- [ ] **Step 6: Add inspector image URL input**

In `design-frame-page.tsx`, add state:

```ts
const [editImageUrl, setEditImageUrl] = useState("");
```

Set it on selected layer changes from current image asset URL if available. Add to payload:

```ts
image_url: editImageUrl.trim() && editImageUrl.trim() !== currentImageUrl ? editImageUrl.trim() : undefined,
```

Render input only when the selected layer has image fill or `type === "image"`:

```tsx
<div className="space-y-1.5">
  <div className="text-xs font-medium text-muted-foreground">替换图片 URL</div>
  <Input value={editImageUrl} placeholder="https://..." onChange={(event) => setEditImageUrl(event.target.value)} />
</div>
```

- [ ] **Step 7: Run checks**

```bash
go test ./internal/handler -run 'TestUpdateDesignLayerLightweightImageURL|TestUpdateDesignLayerLightweight' -count=1
pnpm --filter @multica/views exec tsc --noEmit --pretty false
pnpm --filter @multica/web exec tsc --noEmit --pretty false
```

Expected: all exit 0.

- [ ] **Step 8: Commit**

```bash
git add packages/core/types/design.ts server/internal/handler/design_file.go server/internal/handler/design_file_test.go packages/views/designs/design-frame-page.tsx
git commit -m "feat: add lightweight image replacement"
```

## Task 5: Add File and Frame Quality Summary

**Files:**
- Create: `packages/views/designs/components/design-quality-summary.tsx`
- Modify: `packages/views/designs/design-file-page.tsx`
- Modify: `packages/views/designs/index.ts` if exports are needed

- [ ] **Step 1: Locate frame list owner**

Use:

```bash
python3 - <<'PY'
from pathlib import Path
for p in Path('packages/views/designs').rglob('*.tsx'):
    text=p.read_text()
    if 'frames' in text and ('thumbnail' in text or 'frame' in text):
        print(p)
PY
```

Expected: identify the page/component that renders design file frames.

- [ ] **Step 2: Create quality summary component**

Create `packages/views/designs/components/design-quality-summary.tsx`:

```tsx
import type { GalleryNativeJson } from "@multica/core/types";
import { Badge } from "@multica/ui/components/badge";
import { analyzeFrameFidelity } from "../native-renderer/fidelity";

export function designQualitySummary(nativeJson: GalleryNativeJson | undefined) {
  if (!nativeJson?.frames.length) return null;
  const frameReports = nativeJson.frames.map((frame) => ({ frame, report: analyzeFrameFidelity(nativeJson, frame) }));
  const average = Math.round(frameReports.reduce((sum, item) => sum + item.report.renderQualityPercent, 0) / frameReports.length);
  const lowest = [...frameReports].sort((a, b) => a.report.renderQualityPercent - b.report.renderQualityPercent).slice(0, 3);
  return { average, frameReports, lowest };
}

export function DesignQualitySummary({ nativeJson }: { nativeJson: GalleryNativeJson | undefined }) {
  const summary = designQualitySummary(nativeJson);
  if (!summary) return null;
  return (
    <section className="rounded-2xl border bg-background p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-medium">导入质量</div>
          <div className="mt-1 text-xs text-muted-foreground">按真实图层、局部兜底和缺失情况计算</div>
        </div>
        <Badge variant="outline" className="h-8 px-2 text-xs">平均还原度 {summary.average}%</Badge>
      </div>
      <div className="mt-3 space-y-2 text-xs">
        {summary.lowest.map(({ frame, report }) => (
          <div key={frame.id} className="flex items-center justify-between rounded-lg bg-muted/40 px-3 py-2">
            <span className="truncate">{frame.name}</span>
            <span className="font-mono">{report.renderQualityPercent}%</span>
          </div>
        ))}
      </div>
    </section>
  );
}
```

- [ ] **Step 3: Add summary to design detail page**

In the frame list owner from Step 1, import and render:

```tsx
<DesignQualitySummary nativeJson={data?.current_revision?.native_json} />
```

Place it above frame cards or near file metadata.

- [ ] **Step 4: Add per-frame quality badge to frame cards**

For each frame card, compute:

```ts
const report = nativeJson ? analyzeFrameFidelity(nativeJson, frame) : null;
```

Render:

```tsx
{report ? <Badge variant="outline">还原度 {report.renderQualityPercent}%</Badge> : null}
```

- [ ] **Step 5: Typecheck and browser check**

```bash
pnpm --filter @multica/views exec tsc --noEmit --pretty false
pnpm --filter @multica/web exec tsc --noEmit --pretty false
```

Open:

```text
http://localhost:3031/amc/designs/82c1e643-3530-443a-a531-3cb275b0ba1e
```

Expected: file-level quality summary visible and frame badges visible.

- [ ] **Step 6: Commit**

```bash
git add packages/views/designs
git commit -m "feat: show design import quality summary"
```

## Task 6: Final Regression and Integration

**Files:**
- No planned code changes unless verification reveals a bug.

- [ ] **Step 1: Run backend focused checks**

```bash
go test ./internal/handler -run 'TestUpdateDesignLayerLightweight|TestFigmaPluginImportWithProjectAndFolder|TestFigmaPluginImportTargetDesignFileMergesNewSourceNode' -count=1
go test ./cmd/server -run '^$'
```

Expected: both exit 0.

- [ ] **Step 2: Run frontend checks**

```bash
pnpm --filter @multica/views exec tsc --noEmit --pretty false
pnpm --filter @multica/web exec tsc --noEmit --pretty false
node --check apps/figma-plugin/code.js
```

Expected: all exit 0.

- [ ] **Step 3: Run diff and impact checks**

```bash
git diff --check
npx gitnexus detect-changes
```

Expected: no whitespace errors; GitNexus risk low or understood.

- [ ] **Step 4: Browser regression**

Open each frame and record quality values:

```text
http://localhost:3031/amc/designs/82c1e643-3530-443a-a531-3cb275b0ba1e/frames/frame-1
http://localhost:3031/amc/designs/82c1e643-3530-443a-a531-3cb275b0ba1e/frames/frame-0-468
http://localhost:3031/amc/designs/82c1e643-3530-443a-a531-3cb275b0ba1e/frames/frame-0-651
```

Expected:

```text
frame-1: 85% or better
frame-0-468: 93% or better
frame-0-651: 92% or better
console errors: 0
```

- [ ] **Step 5: Final commit if needed**

If verification required small fixes:

```bash
git add <changed-files>
git commit -m "fix: stabilize native viewer completion"
```
