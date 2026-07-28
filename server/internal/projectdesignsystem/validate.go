package projectdesignsystem

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"regexp"
	"strings"
)

var (
	ErrInvalidPackage = errors.New("project design system package is invalid")
	locatorIDPattern  = regexp.MustCompile("^[a-z0-9][a-z0-9._:-]{0,127}$")
)

func Validate(input ArtifactInput, allowedHosts []string) (ValidatedPackage, error) {
	input = normalizeArtifacts(input)
	pkg := ValidatedPackage{
		Artifacts: input,
		Manifest: Manifest{
			SchemaVersion: SchemaVersion,
			Files:         buildFileManifest(input),
		},
	}

	diagnostics := validateArtifactBounds(input)
	if hasErrors(diagnostics) {
		pkg.Validation = ValidationReport{Passed: false, Diagnostics: diagnostics}
		return pkg, ErrInvalidPackage
	}

	hosts := normalizeAllowedHosts(allowedHosts)
	sections, markdownDiagnostics := parseMarkdownSections(input.DesignMD)
	tokens := parseTokens(input.TokensCSS, hosts)
	components := parseComponentsHTML(input.ComponentsHTML, tokens.declared, hosts)
	diagnostics = append(diagnostics, markdownDiagnostics...)
	diagnostics = append(diagnostics, tokens.diagnostics...)
	diagnostics = append(diagnostics, components.diagnostics...)

	pkg.Manifest.Digest = packageDigest(input)
	pkg.Manifest.Sections = nonNilSections(sections)
	pkg.Manifest.TokenGroups = nonNilTokenGroups(tokens.groups)
	pkg.Manifest.Locators = nonNilLocators(components.locators)
	pkg.Validation = ValidationReport{
		Passed:      !hasErrors(diagnostics),
		Diagnostics: nonNilDiagnostics(diagnostics),
	}
	if !pkg.Validation.Passed {
		return pkg, ErrInvalidPackage
	}
	return pkg, nil
}

func validateArtifactBounds(input ArtifactInput) []Diagnostic {
	artifacts := []struct {
		path  string
		value string
		limit int
	}{
		{path: "DESIGN.md", value: input.DesignMD, limit: MaxDesignMDBytes},
		{path: "tokens.css", value: input.TokensCSS, limit: MaxTokensCSSBytes},
		{path: "components.html", value: input.ComponentsHTML, limit: MaxComponentsHTMLBytes},
	}
	diagnostics := make([]Diagnostic, 0)
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.value) == "" {
			diagnostics = append(diagnostics, errorDiagnostic(
				"artifact_missing",
				artifact.path,
				artifact.path+" must be present and non-empty",
			))
			continue
		}
		if len(artifact.value) > artifact.limit {
			diagnostics = append(diagnostics, errorDiagnostic(
				"artifact_too_large",
				artifact.path,
				artifact.path+" exceeds its size limit",
			))
		}
	}
	if len(input.DesignMD)+len(input.TokensCSS)+len(input.ComponentsHTML) > MaxAggregateBytes {
		diagnostics = append(diagnostics, errorDiagnostic(
			"package_too_large",
			"",
			"Project design system package exceeds its aggregate size limit",
		))
	}
	return diagnostics
}

func normalizeArtifacts(input ArtifactInput) ArtifactInput {
	input.DesignMD = normalizeLineEndings(input.DesignMD)
	input.TokensCSS = normalizeLineEndings(input.TokensCSS)
	input.ComponentsHTML = normalizeLineEndings(input.ComponentsHTML)
	return input
}

func normalizeLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func buildFileManifest(input ArtifactInput) map[string]FileManifest {
	return map[string]FileManifest{
		"DESIGN.md": {
			SizeBytes: len(input.DesignMD),
			SHA256:    contentDigest(input.DesignMD),
			MediaType: "text/markdown; charset=utf-8",
		},
		"tokens.css": {
			SizeBytes: len(input.TokensCSS),
			SHA256:    contentDigest(input.TokensCSS),
			MediaType: "text/css; charset=utf-8",
		},
		"components.html": {
			SizeBytes: len(input.ComponentsHTML),
			SHA256:    contentDigest(input.ComponentsHTML),
			MediaType: "text/html; charset=utf-8",
		},
	}
}

func contentDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func packageDigest(input ArtifactInput) string {
	hasher := sha256.New()
	writeDigestPart(hasher, "DESIGN.md", []byte(input.DesignMD))
	writeDigestPart(hasher, "tokens.css", []byte(input.TokensCSS))
	writeDigestPart(hasher, "components.html", []byte(input.ComponentsHTML))
	return hex.EncodeToString(hasher.Sum(nil))
}

func writeDigestPart(hasher hash.Hash, name string, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(name)))
	_, _ = hasher.Write(length[:])
	_, _ = hasher.Write([]byte(name))
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = hasher.Write(length[:])
	_, _ = hasher.Write(value)
}

func errorDiagnostic(code, path, message string) Diagnostic {
	return Diagnostic{Code: code, Severity: DiagnosticError, Path: path, Message: message}
}

func hasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == DiagnosticError {
			return true
		}
	}
	return false
}

func validLocatorID(value string) bool {
	return locatorIDPattern.MatchString(value)
}

func nonNilSections(values []Section) []Section {
	if values == nil {
		return []Section{}
	}
	return values
}

func nonNilTokenGroups(values []TokenGroup) []TokenGroup {
	if values == nil {
		return []TokenGroup{}
	}
	return values
}

func nonNilLocators(values []Locator) []Locator {
	if values == nil {
		return []Locator{}
	}
	return values
}

func nonNilDiagnostics(values []Diagnostic) []Diagnostic {
	if values == nil {
		return []Diagnostic{}
	}
	return values
}
