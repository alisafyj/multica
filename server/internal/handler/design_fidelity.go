package handler

import (
	"encoding/json"
	"math"
	"strings"
	"time"
)

type importFidelityDoc struct {
	Frames []struct {
		ID          string `json:"id"`
		RootLayerID string `json:"rootLayerId"`
	} `json:"frames"`
	Layers map[string]importFidelityLayer `json:"layers"`
	Assets map[string]struct {
		URL string `json:"url"`
	} `json:"assets"`
	Source map[string]any `json:"source"`
}

type importFidelityLayer struct {
	ID      string         `json:"id"`
	FrameID string         `json:"frameId"`
	Type    string         `json:"type"`
	Visible *bool          `json:"visible"`
	Text    map[string]any `json:"text"`
	Image   *struct {
		AssetID string `json:"assetId"`
	} `json:"image"`
	Shape *struct {
		ShapeType string `json:"shapeType"`
	} `json:"shape"`
	Children []string       `json:"children"`
	Style    map[string]any `json:"style"`
	Source   map[string]any `json:"source"`
}

func annotateImportFidelityReport(raw json.RawMessage) (json.RawMessage, error) {
	var doc importFidelityDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	source, _ := root["source"].(map[string]any)
	if source == nil {
		source = map[string]any{}
		root["source"] = source
	}
	byFrameID := map[string]any{}
	for _, frame := range doc.Frames {
		total, native, fallback, unsupported := 0, 0, 0, 0
		for _, layer := range doc.Layers {
			if layer.FrameID != frame.ID || layer.ID == frame.RootLayerID || (layer.Visible != nil && !*layer.Visible) {
				continue
			}
			total++
			switch classifyImportFidelityLayer(doc, layer) {
			case "native":
				native++
			case "unsupported":
				unsupported++
			default:
				fallback++
			}
		}
		nativePercent := 100
		if total > 0 {
			nativePercent = int(math.Round(float64(native) / float64(total) * 100))
		}
		byFrameID[frame.ID] = map[string]any{"total": total, "native": native, "fallback": fallback, "unsupported": unsupported, "nativePercent": nativePercent}
	}
	source["importFidelityReport"] = map[string]any{"byFrameId": byFrameID, "updatedAt": time.Now().UTC().Format(time.RFC3339)}
	return json.Marshal(root)
}

func classifyImportFidelityLayer(doc importFidelityDoc, layer importFidelityLayer) string {
	switch layer.Type {
	case "text":
		return "native"
	case "image":
		if layer.Image != nil && hasUsableImportAsset(doc, layer.Image.AssetID) {
			return "native"
		}
		if hasImportImageFill(layer.Style) || layer.Image != nil {
			return "fallback"
		}
		return "unsupported"
	case "shape":
		if layer.Shape != nil && layer.Shape.ShapeType != "" || hasImportFillOrStroke(layer.Style) {
			return "native"
		}
		return "fallback"
	case "frame", "group", "component", "instance":
		return "native"
	case "vector", "slice", "custom":
		return "fallback"
	default:
		if hasImportFillOrStroke(layer.Style) {
			return "fallback"
		}
		return "unsupported"
	}
}

func hasUsableImportAsset(doc importFidelityDoc, assetID string) bool {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return false
	}
	asset, ok := doc.Assets[assetID]
	return ok && strings.TrimSpace(asset.URL) != "" && !strings.HasPrefix(asset.URL, "figma-image-hash://")
}

func hasImportImageFill(style map[string]any) bool {
	for _, fill := range objectSliceFromAny(style["fills"]) {
		if stringAny(fill["assetId"]) != "" || stringAny(fill["imageHash"]) != "" || stringAny(fill["type"]) == "image" {
			return true
		}
	}
	return false
}

func hasImportFillOrStroke(style map[string]any) bool {
	return len(objectSliceFromAny(style["fills"])) > 0 || len(objectSliceFromAny(style["strokes"])) > 0 || style["fill"] != nil || style["stroke"] != nil || style["backgroundColor"] != nil || style["borderColor"] != nil
}

func objectSliceFromAny(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func stringAny(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
