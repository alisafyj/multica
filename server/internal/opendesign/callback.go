package opendesign

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
)

const (
	RunResultPackageSchema              = "open-design.run-result-package.v1"
	RunArchiveContentType               = "application/zip"
	RunArchiveRunIDHeader               = "X-Open-Design-Run-ID"
	RunArchiveContentDigestHeader       = "X-Open-Design-Content-Digest"
	BasePackageSlotHeader               = "X-Open-Design-Base-Slot"
	BasePackageSourceTaskIDHeader       = "X-Open-Design-Base-Source-Task-ID"
	PackageAuditReceiptSchema           = "multica.open-design-package-audit/v1"
	RunArchiveMaxBytes            int64 = 100 << 20
)

const (
	packageAuditMaxIssues       = 4096
	packageAuditCodeMaxBytes    = 128
	packageAuditMessageMaxBytes = 4 << 10
	packageAuditPathMaxBytes    = 4 << 10
)

type RunStartRequest struct {
	OpenDesignRunID string `json:"open_design_run_id"`
}

type RunEvent struct {
	ID    int64           `json:"id"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type RunEventRequest struct {
	OpenDesignRunID string   `json:"open_design_run_id"`
	Event           RunEvent `json:"event"`
}

type RunResultRequest struct {
	OpenDesignRunID  string               `json:"open_design_run_id"`
	ResultPackage    json.RawMessage      `json:"result_package"`
	ArtifactIndex    []ArtifactIndexEntry `json:"artifact_index"`
	ArchiveObjectKey string               `json:"archive_object_key"`
	ContentDigest    string               `json:"content_digest"`
}

type PackageAuditIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

// PackageAudit is the fixed Open Design audit result without its local projectPath.
type PackageAudit struct {
	OK             bool                `json:"ok"`
	FilesInspected int                 `json:"filesInspected"`
	Errors         []PackageAuditIssue `json:"errors"`
	Warnings       []PackageAuditIssue `json:"warnings"`
}

type PackageAuditReceipt struct {
	Schema        string         `json:"schema"`
	Engine        EngineIdentity `json:"engine"`
	ContentDigest string         `json:"content_digest"`
	Audit         PackageAudit   `json:"audit"`
}

type RunAuditRequest struct {
	OpenDesignRunID string              `json:"open_design_run_id"`
	AuditReport     PackageAuditReceipt `json:"audit_report"`
}

type RunArchiveResponse struct {
	ArchiveObjectKey string `json:"archive_object_key"`
}

type RunTerminalRequest struct {
	OpenDesignRunID string          `json:"open_design_run_id,omitempty"`
	Status          RunStatus       `json:"status"`
	Failure         json.RawMessage `json:"failure"`
}

func NewPackageAuditReceipt(engine EngineIdentity, contentDigest string, audit PackageAudit) (PackageAuditReceipt, error) {
	receipt := PackageAuditReceipt{
		Schema:        PackageAuditReceiptSchema,
		Engine:        engine,
		ContentDigest: contentDigest,
		Audit:         audit,
	}
	if err := ValidatePackageAuditReceipt(receipt); err != nil {
		return PackageAuditReceipt{}, err
	}
	return receipt, nil
}

func ValidatePackageAuditReceipt(receipt PackageAuditReceipt) error {
	if receipt.Schema != PackageAuditReceiptSchema {
		return fmt.Errorf("Open Design package audit schema %q does not match %q", receipt.Schema, PackageAuditReceiptSchema)
	}
	if err := receipt.Engine.Validate(); err != nil {
		return fmt.Errorf("invalid Open Design package audit engine: %w", err)
	}
	if err := ValidateContentDigest(receipt.ContentDigest); err != nil {
		return err
	}
	return ValidatePackageAudit(receipt.Audit)
}

func ValidatePackageAudit(audit PackageAudit) error {
	if audit.FilesInspected < 0 {
		return errors.New("Open Design package audit filesInspected must not be negative")
	}
	if audit.Errors == nil || audit.Warnings == nil {
		return errors.New("Open Design package audit errors and warnings must be arrays")
	}
	if len(audit.Errors) > packageAuditMaxIssues || len(audit.Warnings) > packageAuditMaxIssues {
		return errors.New("Open Design package audit has too many issues")
	}
	if audit.OK != (len(audit.Errors) == 0 && len(audit.Warnings) == 0) {
		return errors.New("Open Design package audit ok does not match the fail-on-warnings policy")
	}
	for _, issue := range audit.Errors {
		if err := validatePackageAuditIssue(issue, "error"); err != nil {
			return err
		}
	}
	for _, issue := range audit.Warnings {
		if err := validatePackageAuditIssue(issue, "warning"); err != nil {
			return err
		}
	}
	return nil
}

func PackageAuditFailure(audit PackageAudit) json.RawMessage {
	if audit.OK {
		return json.RawMessage(`{}`)
	}
	message := fmt.Sprintf("Open Design package audit rejected the candidate with %d error(s) and %d warning(s)", len(audit.Errors), len(audit.Warnings))
	issues := audit.Errors
	if len(issues) == 0 {
		issues = audit.Warnings
	}
	if len(issues) > 0 {
		first := issues[0]
		message += ": " + first.Code
		if first.Path != "" {
			message += " at " + first.Path
		}
	}
	payload, _ := json.Marshal(map[string]string{
		"code":    "open_design_package_audit_failed",
		"message": message,
	})
	return payload
}

func validatePackageAuditIssue(issue PackageAuditIssue, expectedSeverity string) error {
	if issue.Severity != expectedSeverity {
		return fmt.Errorf("Open Design package audit %s issue has severity %q", expectedSeverity, issue.Severity)
	}
	if issue.Code == "" || strings.TrimSpace(issue.Code) != issue.Code || len(issue.Code) > packageAuditCodeMaxBytes {
		return errors.New("Open Design package audit issue has an invalid code")
	}
	if issue.Message == "" || len(issue.Message) > packageAuditMessageMaxBytes {
		return errors.New("Open Design package audit issue has an invalid message")
	}
	if issue.Path == "" {
		return nil
	}
	relativePath := strings.TrimSuffix(issue.Path, "/")
	if len(issue.Path) > packageAuditPathMaxBytes || strings.TrimSpace(issue.Path) != issue.Path || strings.Contains(issue.Path, "\\") || path.IsAbs(issue.Path) || path.Clean(relativePath) != relativePath || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, "../") {
		return errors.New("Open Design package audit issue has an invalid relative path")
	}
	return nil
}
