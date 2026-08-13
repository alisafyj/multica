package projectdesignsystem

const (
	SchemaVersion = "multica.project-design-system/v1"

	MaxDesignMDBytes       = 256 << 10
	MaxTokensCSSBytes      = 256 << 10
	MaxComponentsHTMLBytes = 1 << 20
	MaxAggregateBytes      = 3 << 19
)

type ArtifactInput struct {
	DesignMD       string `json:"design_md"`
	TokensCSS      string `json:"tokens_css"`
	ComponentsHTML string `json:"components_html"`
}

type Section struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

type TokenValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type TokenGroup struct {
	ID     string       `json:"id"`
	Label  string       `json:"label"`
	Tokens []TokenValue `json:"tokens"`
}

type Locator struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type FileManifest struct {
	SizeBytes int    `json:"size_bytes"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type"`
}

type Manifest struct {
	SchemaVersion string                  `json:"schema_version"`
	Digest        string                  `json:"digest"`
	Files         map[string]FileManifest `json:"files"`
	Sections      []Section               `json:"sections"`
	TokenGroups   []TokenGroup            `json:"token_groups"`
	Locators      []Locator               `json:"locators"`
}

type DiagnosticSeverity string

const (
	DiagnosticWarning DiagnosticSeverity = "warning"
	DiagnosticError   DiagnosticSeverity = "error"
)

type Diagnostic struct {
	Code     string             `json:"code"`
	Severity DiagnosticSeverity `json:"severity"`
	Path     string             `json:"path,omitempty"`
	Message  string             `json:"message"`
}

type ValidationReport struct {
	Passed      bool         `json:"passed"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type ValidatedPackage struct {
	Artifacts  ArtifactInput    `json:"artifacts"`
	Manifest   Manifest         `json:"manifest"`
	Validation ValidationReport `json:"validation"`
}
