package designcore

import "encoding/json"

const NativeJSONVersion = "1.0"

type NativeJSON struct {
	Version           string                      `json:"version"`
	File              FileMeta                    `json:"file"`
	Frames            []Frame                     `json:"frames"`
	Layers            map[string]Layer            `json:"layers"`
	Assets            map[string]Asset            `json:"assets"`
	Tokens            map[string]any              `json:"tokens,omitempty"`
	Slots             map[string]SlotBinding      `json:"slots,omitempty"`
	Modules           map[string]ModuleBinding    `json:"modules,omitempty"`
	States            map[string]StateBinding     `json:"states,omitempty"`
	ComponentBindings map[string]ComponentBinding `json:"componentBindings,omitempty"`
	RestoreHints      map[string]any              `json:"restoreHints,omitempty"`
	Source            map[string]any              `json:"source,omitempty"`
}

type FileMeta struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SourceType  string `json:"sourceType"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type Frame struct {
	ID               string         `json:"id"`
	SourceNodeID     string         `json:"sourceNodeId,omitempty"`
	Name             string         `json:"name"`
	RootLayerID      string         `json:"rootLayerId"`
	Width            float64        `json:"width"`
	Height           float64        `json:"height"`
	X                float64        `json:"x,omitempty"`
	Y                float64        `json:"y,omitempty"`
	PreviewAssetID   string         `json:"previewAssetId,omitempty"`
	ThumbnailAssetID string         `json:"thumbnailAssetId,omitempty"`
	ThumbnailDataURL string         `json:"thumbnailDataUrl,omitempty"`
	ThumbnailURL     string         `json:"thumbnailUrl,omitempty"`
	Board            map[string]any `json:"board,omitempty"`
}

type Layer struct {
	ID           string           `json:"id"`
	SourceNodeID string           `json:"sourceNodeId,omitempty"`
	FrameID      string           `json:"frameId"`
	ParentID     string           `json:"parentId,omitempty"`
	Children     []string         `json:"children,omitempty"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Visible      bool             `json:"visible"`
	X            float64          `json:"x"`
	Y            float64          `json:"y"`
	Width        float64          `json:"width"`
	Height       float64          `json:"height"`
	Rotation     float64          `json:"rotation,omitempty"`
	Opacity      float64          `json:"opacity,omitempty"`
	Text         map[string]any   `json:"text,omitempty"`
	Image        *ImageData       `json:"image,omitempty"`
	Shape        map[string]any   `json:"shape,omitempty"`
	Exportable   []map[string]any `json:"exportable,omitempty"`
	Semantic     map[string]any   `json:"semantic,omitempty"`
	Style        map[string]any   `json:"style,omitempty"`
	Source       map[string]any   `json:"source,omitempty"`
}

type ImageData struct {
	AssetID string `json:"assetId"`
	Alt     string `json:"alt,omitempty"`
}

type Asset struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	URL          string         `json:"url"`
	ContentType  string         `json:"contentType,omitempty"`
	Width        float64        `json:"width,omitempty"`
	Height       float64        `json:"height,omitempty"`
	SizeBytes    int64          `json:"sizeBytes,omitempty"`
	SourceNodeID string         `json:"sourceNodeId,omitempty"`
	FrameID      string         `json:"frameId,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type SlotBinding struct {
	SlotKey  string   `json:"slotKey"`
	LayerIDs []string `json:"layerIds"`
	Value    any      `json:"value,omitempty"`
}

type ModuleBinding struct {
	ModuleKey string   `json:"moduleKey"`
	LayerIDs  []string `json:"layerIds"`
	EntityKey string   `json:"entityKey,omitempty"`
}

type StateBinding struct {
	StateKey  string   `json:"stateKey"`
	LayerIDs  []string `json:"layerIds"`
	StateType string   `json:"stateType,omitempty"`
}

type ComponentBinding struct {
	ComponentKey string         `json:"componentKey"`
	Target       string         `json:"target,omitempty"`
	Props        map[string]any `json:"props,omitempty"`
}

func ParseNativeJSON(raw []byte) (NativeJSON, error) {
	var doc NativeJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return NativeJSON{}, err
	}
	return doc, nil
}
