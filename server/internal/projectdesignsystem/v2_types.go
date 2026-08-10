package projectdesignsystem

const (
	PackageSchemaV2           = "multica.project-design-system/v2"
	SourceIndexSchemaV1       = "multica.project-design-system-source-index/v1"
	AuditSchemaV1             = "multica.project-design-system-audit/v1"
	maxV2ArchiveBytes         = 64 << 20
	maxV2Files                = 512
	maxV2FileBytes      int64 = 16 << 20
	maxV2TotalBytes           = 128 << 20
	maxV2PreviewTargets       = 8
)

type PackageBinding struct {
	WorkspaceID         string `json:"workspace_id"`
	ProjectID           string `json:"project_id"`
	DesignSystemID      string `json:"design_system_id"`
	TaskID              string `json:"task_id"`
	AgentID             string `json:"agent_id"`
	Operation           string `json:"operation"`
	InputSnapshotSHA256 string `json:"input_snapshot_sha256"`
	BasePackageSHA256   string `json:"base_package_sha256,omitempty"`
}

type ArtifactIndexEntry struct {
	Path      string `json:"path"`
	Role      string `json:"role"`
	MediaType string `json:"media_type"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type PreviewTarget struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type ManifestV2 struct {
	SchemaVersion  string               `json:"schema_version"`
	Binding        PackageBinding       `json:"binding"`
	ContentDigest  string               `json:"content_digest"`
	Files          []ArtifactIndexEntry `json:"files"`
	PreviewTargets []PreviewTarget      `json:"preview_targets"`
	Sections       []Section            `json:"sections"`
	TokenGroups    []TokenGroup         `json:"token_groups"`
	Locators       []Locator            `json:"locators"`
}

type SourceIndex struct {
	SchemaVersion       string           `json:"schema_version"`
	InputSnapshotSHA256 string           `json:"input_snapshot_sha256"`
	Evidence            []SourceEvidence `json:"evidence"`
	Conflicts           []SourceConflict `json:"conflicts"`
	Fallbacks           []SourceFallback `json:"fallbacks"`
}

type SourceEvidence struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Summary    string   `json:"summary"`
	References []string `json:"references"`
}

type SourceConflict struct {
	ID         string   `json:"id"`
	Summary    string   `json:"summary"`
	References []string `json:"references"`
}

type SourceFallback struct {
	ID         string   `json:"id"`
	Summary    string   `json:"summary"`
	References []string `json:"references,omitempty"`
}

type AuditReport struct {
	SchemaVersion string       `json:"schema_version"`
	Passed        bool         `json:"passed"`
	ContentDigest string       `json:"content_digest"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}

type CollectedV2Package struct {
	Archive  []byte      `json:"-"`
	Manifest ManifestV2  `json:"manifest"`
	Audit    AuditReport `json:"audit"`
}

type ValidatedV2Package struct {
	Manifest ManifestV2  `json:"manifest"`
	Audit    AuditReport `json:"audit"`
}

type v2AuditResult struct {
	sections    []Section
	tokenGroups []TokenGroup
	locators    []Locator
	report      AuditReport
}
