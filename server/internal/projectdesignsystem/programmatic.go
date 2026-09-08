package projectdesignsystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxProgrammaticWalkEntries = 12000
	maxProgrammaticSourceFiles = 64
	maxProgrammaticFileBytes   = 256 << 10
	maxProgrammaticTotalBytes  = 6 << 20

	programmaticProfileProduct = "product"
)

// ProgrammaticInput is the fixed input consumed by the fast repository adapter.
// It deliberately excludes agent, memory, skill and MCP state: the same repository
// checkout and input snapshot must produce the same package bytes.
type ProgrammaticInput struct {
	ProjectName         string
	ProjectDescription  string
	RepositoryName      string
	RepositoryURL       string
	CommitSHA           string
	Platform            string
	Brief               string
	InputSnapshotSHA256 string
	RepositoryAnalysis  *RepositoryDesignContext
}

// ProgrammaticProgress is emitted at deterministic checkpoints so the UI can
// show truthful building blocks without inventing a percentage.
type ProgrammaticProgress struct {
	Stage            string
	SourceFileCount  int
	ColorCount       int
	ComponentCount   int
	PagePatternCount int
}

// ProgrammaticResult summarizes the generated quick draft without returning
// source contents or local checkout paths to the server.
type ProgrammaticResult struct {
	SourceFiles      []string
	ColorCount       int
	ComponentCount   int
	PagePatternCount int
}

type programmaticCandidate struct {
	path  string
	kind  string
	score int
	size  int64
}

type programmaticSource struct {
	path    string
	kind    string
	content string
}

type programmaticObservation struct {
	customProperties map[string]string
	colors           map[string]int
	fonts            map[string]int
	radii            map[string]int
	spacings         map[int]int
	components       []string
	componentPaths   map[string][]string
	pagePatterns     []string
	pagePaths        map[string][]string
	framework        string
	webAdminSignals  map[string]struct{}
	semanticPaths    map[string][]string
}

type programmaticTheme struct {
	profile        string
	framework      string
	primary        string
	primaryText    string
	background     string
	surface        string
	surfaceMuted   string
	foreground     string
	mutedText      string
	subtleText     string
	border         string
	sidebar        string
	tableHeader    string
	info           string
	success        string
	warning        string
	danger         string
	fontFamily     string
	fontDisplay    string
	baseSpace      int
	borderRadius   string
	controlHeight  int
	sidebarWidth   int
	topbarHeight   int
	components     []string
	componentPaths map[string][]string
	pagePatterns   []string
	pagePaths      map[string][]string
	semanticPaths  map[string][]string
}

var (
	programmaticCustomPropertyPattern = regexp.MustCompile(`(?mi)(--[a-z0-9_-]+)\s*:\s*([^;{}\n]+)`)
	programmaticSassVariablePattern   = regexp.MustCompile(`(?mi)^\s*(\$[a-z0-9_-]+)\s*:\s*([^;{}\n]+)`)
	programmaticHexColorPattern       = regexp.MustCompile(`(?i)#[0-9a-f]{8}\b|#[0-9a-f]{6}\b|#[0-9a-f]{3}\b`)
	programmaticFontFamilyPattern     = regexp.MustCompile(`(?i)font-family\s*:\s*([^;}\n]+)`)
	programmaticRadiusPattern         = regexp.MustCompile(`(?i)border-radius\s*:\s*([0-9]+(?:\.[0-9]+)?(?:px|rem))`)
	programmaticSpacingPattern        = regexp.MustCompile(`(?i)(?:gap|padding|margin)(?:-[a-z]+)?\s*:\s*([0-9]+)px`)
)

// GenerateProgrammaticFirstPackage performs a bounded, read-only repository
// harvest and writes a complete V2 candidate package. The normal daemon finalize
// path remains authoritative for Audit, real-browser Preview and upload.
func GenerateProgrammaticFirstPackage(
	ctx context.Context,
	repositoryRoot string,
	outputDir string,
	input ProgrammaticInput,
	onProgress func(ProgrammaticProgress),
) (ProgrammaticResult, error) {
	if strings.TrimSpace(outputDir) == "" {
		return ProgrammaticResult{}, errors.New("programmatic design system output directory is required")
	}
	if strings.TrimSpace(input.InputSnapshotSHA256) == "" {
		return ProgrammaticResult{}, errors.New("programmatic design system input snapshot digest is required")
	}
	if strings.TrimSpace(repositoryRoot) == "" && input.RepositoryAnalysis == nil {
		return ProgrammaticResult{}, errors.New("repository checkout and frozen repository analysis are both unavailable")
	}

	sources, err := inspectProgrammaticRepository(ctx, repositoryRoot)
	if err != nil {
		return ProgrammaticResult{}, err
	}
	reportProgrammaticProgress(onProgress, ProgrammaticProgress{
		Stage:           "inventory",
		SourceFileCount: len(sources),
	})

	observation := observeProgrammaticSources(sources, input.RepositoryAnalysis)
	theme := deriveProgrammaticTheme(observation)
	reportProgrammaticProgress(onProgress, ProgrammaticProgress{
		Stage:            "extraction",
		SourceFileCount:  len(sources),
		ColorCount:       len(observation.colors),
		ComponentCount:   len(theme.components),
		PagePatternCount: len(theme.pagePatterns),
	})

	if err := ctx.Err(); err != nil {
		return ProgrammaticResult{}, err
	}
	if err := writeProgrammaticPackage(outputDir, input, sources, observation, theme); err != nil {
		return ProgrammaticResult{}, err
	}
	reportProgrammaticProgress(onProgress, ProgrammaticProgress{
		Stage:            "package",
		SourceFileCount:  len(sources),
		ColorCount:       len(observation.colors),
		ComponentCount:   len(theme.components),
		PagePatternCount: len(theme.pagePatterns),
	})

	paths := make([]string, 0, len(sources))
	for _, source := range sources {
		paths = append(paths, source.path)
	}
	return ProgrammaticResult{
		SourceFiles:      paths,
		ColorCount:       len(observation.colors),
		ComponentCount:   len(theme.components),
		PagePatternCount: len(theme.pagePatterns),
	}, nil
}

func reportProgrammaticProgress(callback func(ProgrammaticProgress), value ProgrammaticProgress) {
	if callback != nil {
		callback(value)
	}
}

func inspectProgrammaticRepository(ctx context.Context, root string) ([]programmaticSource, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return []programmaticSource{}, nil
	}
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("repository checkout is not a safe directory")
	}

	candidates := make([]programmaticCandidate, 0, 128)
	walked := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		walked++
		if walked > maxProgrammaticWalkEntries {
			return filepath.SkipAll
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative != "." && programmaticIgnoredDirectory(strings.ToLower(entry.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if relative == "." || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() <= 0 || fileInfo.Size() > maxProgrammaticFileBytes {
			return nil
		}
		score := programmaticSourceScore(relative)
		if score <= 0 {
			return nil
		}
		candidates = append(candidates, programmaticCandidate{
			path: relative, kind: programmaticSourceKind(relative), score: score, size: fileInfo.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory repository sources: %w", err)
	}

	candidates = selectProgrammaticCandidates(candidates)
	sources := make([]programmaticSource, 0, len(candidates))
	total := int64(0)
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if total+candidate.size > maxProgrammaticTotalBytes {
			continue
		}
		absolute := filepath.Join(root, filepath.FromSlash(candidate.path))
		fileInfo, err := os.Lstat(absolute)
		if err != nil || fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() || fileInfo.Size() != candidate.size {
			continue
		}
		contents, err := os.ReadFile(absolute)
		if err != nil {
			continue
		}
		total += int64(len(contents))
		sources = append(sources, programmaticSource{path: candidate.path, kind: candidate.kind, content: string(contents)})
	}
	return sources, nil
}

func programmaticIgnoredDirectory(name string) bool {
	switch name {
	case ".git", ".next", ".turbo", ".cache", "node_modules", "vendor", "dist", "build", "coverage", "tmp", "temp", "out", "target":
		return true
	default:
		return false
	}
}

func programmaticSourceKind(relative string) string {
	lower := strings.ToLower("/" + filepath.ToSlash(relative))
	base := strings.ToLower(filepath.Base(relative))
	ext := strings.ToLower(filepath.Ext(relative))
	code := ext == ".tsx" || ext == ".jsx" || ext == ".ts" || ext == ".js" || ext == ".vue" || ext == ".svelte"
	style := ext == ".css" || ext == ".scss" || ext == ".sass" || ext == ".less"

	switch base {
	case "package.json", "readme.md", "design.md", "ai_context.md", "tailwind.config.js", "tailwind.config.ts", "theme.ts", "theme.js", "tokens.css", "variables.css", "globals.css", "global.css":
		return "foundation"
	}
	for _, term := range []string{"/theme/", "/themes/", "/tokens/", "/design-system/", "/design_system/", "/layout/", "/layouts/", "/shell/"} {
		if strings.Contains(lower, term) {
			return "foundation"
		}
	}
	for _, term := range []string{"/components/", "/ui/", "/widgets/"} {
		if strings.Contains(lower, term) {
			return "component"
		}
	}
	if code {
		for _, term := range []string{"/pages/", "/app/", "/views/", "/screens/", "/routes/"} {
			if strings.Contains(lower, term) {
				return "page"
			}
		}
	}
	if style {
		return "style"
	}
	return "context"
}

func selectProgrammaticCandidates(values []programmaticCandidate) []programmaticCandidate {
	sortProgrammaticCandidates(values)
	limits := map[string]int{
		"foundation": 16,
		"component":  20,
		"page":       16,
		"style":      8,
		"context":    4,
	}
	selected := make([]programmaticCandidate, 0, maxProgrammaticSourceFiles)
	seen := make(map[string]struct{}, maxProgrammaticSourceFiles)
	for _, kind := range []string{"foundation", "component", "page", "style", "context"} {
		remaining := limits[kind]
		for _, candidate := range values {
			if remaining == 0 || candidate.kind != kind {
				continue
			}
			selected = append(selected, candidate)
			seen[candidate.path] = struct{}{}
			remaining--
		}
	}
	for _, candidate := range values {
		if len(selected) >= maxProgrammaticSourceFiles {
			break
		}
		if _, exists := seen[candidate.path]; exists {
			continue
		}
		selected = append(selected, candidate)
		seen[candidate.path] = struct{}{}
	}
	return selected
}

func programmaticSourceScore(relative string) int {
	lower := strings.ToLower(relative)
	base := strings.ToLower(filepath.Base(relative))
	ext := strings.ToLower(filepath.Ext(relative))
	allowed := map[string]bool{
		".css": true, ".scss": true, ".sass": true, ".less": true,
		".tsx": true, ".jsx": true, ".ts": true, ".js": true,
		".vue": true, ".svelte": true, ".json": true, ".md": true,
	}
	if !allowed[ext] {
		return 0
	}
	if strings.HasSuffix(base, ".min"+ext) || strings.Contains(lower, "/__snapshots__/") || strings.HasSuffix(base, ".map") {
		return 0
	}

	score := 1
	switch base {
	case "package.json":
		// Framework and component-library detection is impossible without the
		// root package contract. Keep it ahead of large layout directories whose
		// child components would otherwise consume the foundation quota.
		score += 1000
	case "readme.md", "design.md", "ai_context.md":
		score += 300
	case "tailwind.config.js", "tailwind.config.ts", "theme.ts", "theme.js", "tokens.css", "variables.css", "globals.css", "global.css":
		score += 150
	}
	for _, term := range []string{"/styles/", "/theme/", "/themes/", "/tokens/", "/design-system/", "/design_system/"} {
		if strings.Contains("/"+lower, term) {
			score += 100
		}
	}
	normalizedPath := "/" + lower
	for _, term := range []string{"/components/", "/ui/", "/widgets/"} {
		if strings.Contains(normalizedPath, term) {
			score += 65
		}
	}
	if strings.Contains(normalizedPath, "/src/components/") || strings.Contains(normalizedPath, "/app/components/") {
		score += 70
	}
	for _, term := range []string{"/pages/", "/app/", "/views/", "/screens/", "/routes/"} {
		if strings.Contains(normalizedPath, term) {
			score += 45
		}
	}
	for _, term := range []string{"/src/layout/", "/app/layout/", "/layouts/", "/shell/"} {
		if strings.Contains(normalizedPath, term) {
			score += 150
		}
	}
	for _, term := range []string{"pagination", "dialogstd", "drawer", "sidebar", "navbar", "appmain", "breadcrumb", "headersearch", "table", "form"} {
		if strings.Contains(lower, term) {
			score += 35
		}
	}
	for _, term := range []string{"inventory", "management", "serviceflow", "customer", "consult", "order", "dashboard", "admin"} {
		if strings.Contains(lower, term) {
			score += 18
		}
	}
	for _, term := range []string{"button", "input", "card", "nav", "header", "tab", "modal", "dialog", "list", "detail", "home", "index", "layout"} {
		if strings.Contains(base, term) {
			score += 15
		}
	}
	for _, term := range []string{"/demo/", "/examples/", "/template/", "/theme/", "icon-picker", "pixijs", "mock."} {
		if strings.Contains(normalizedPath, term) {
			score -= 25
		}
	}
	if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		score -= 60
	}
	return score
}

func sortProgrammaticCandidates(values []programmaticCandidate) {
	sort.Slice(values, func(left, right int) bool {
		if values[left].score != values[right].score {
			return values[left].score > values[right].score
		}
		return values[left].path < values[right].path
	})
}

func observeProgrammaticSources(sources []programmaticSource, analysis *RepositoryDesignContext) programmaticObservation {
	result := programmaticObservation{
		customProperties: make(map[string]string),
		colors:           make(map[string]int),
		fonts:            make(map[string]int),
		radii:            make(map[string]int),
		spacings:         make(map[int]int),
		componentPaths:   make(map[string][]string),
		pagePaths:        make(map[string][]string),
		webAdminSignals:  make(map[string]struct{}),
		semanticPaths:    make(map[string][]string),
	}
	componentSeen := make(map[string]struct{})
	pageSeen := make(map[string]struct{})

	for _, source := range sources {
		for _, match := range programmaticCustomPropertyPattern.FindAllStringSubmatch(source.content, -1) {
			name := strings.ToLower(strings.TrimSpace(match[1]))
			value := strings.TrimSpace(match[2])
			if _, exists := result.customProperties[name]; !exists && len(value) <= 160 {
				result.customProperties[name] = value
			}
			if color := normalizeProgrammaticHex(value); color != "" {
				result.colors[color]++
			}
		}
		for _, match := range programmaticSassVariablePattern.FindAllStringSubmatch(source.content, -1) {
			name := strings.ToLower(strings.TrimSpace(match[1]))
			value := strings.TrimSpace(match[2])
			if _, exists := result.customProperties[name]; !exists && len(value) <= 160 {
				result.customProperties[name] = value
			}
		}
		for _, raw := range programmaticHexColorPattern.FindAllString(source.content, -1) {
			if color := normalizeProgrammaticHex(raw); color != "" {
				result.colors[color]++
			}
		}
		for _, match := range programmaticFontFamilyPattern.FindAllStringSubmatch(source.content, -1) {
			if family := normalizeProgrammaticFont(match[1]); family != "" {
				result.fonts[family]++
			}
		}
		for _, match := range programmaticRadiusPattern.FindAllStringSubmatch(source.content, -1) {
			result.radii[strings.ToLower(match[1])]++
		}
		for _, match := range programmaticSpacingPattern.FindAllStringSubmatch(source.content, -1) {
			value, err := strconv.Atoi(match[1])
			if err == nil && value > 0 && value <= 96 {
				result.spacings[value]++
			}
		}
		observeProgrammaticStructure(&result, source)

		if component := componentNameFromPath(source.path); component != "" {
			key := strings.ToLower(component)
			result.componentPaths[key] = appendUniqueString(result.componentPaths[key], source.path, 4)
			if _, exists := componentSeen[key]; !exists {
				componentSeen[key] = struct{}{}
				result.components = append(result.components, component)
			}
		}
		if page := pagePatternFromPath(source.path); page != "" {
			key := strings.ToLower(page)
			result.pagePaths[key] = appendUniqueString(result.pagePaths[key], source.path, 4)
			if _, exists := pageSeen[key]; !exists {
				pageSeen[key] = struct{}{}
				result.pagePatterns = append(result.pagePatterns, page)
			}
		}
	}

	if analysis != nil {
		for _, workflow := range analysis.RepresentativeWorkflows {
			name := cleanProgrammaticText(workflow.Name, 120)
			if name != "" {
				key := strings.ToLower(name)
				result.pagePaths[key] = appendUniqueStrings(result.pagePaths[key], safeProgrammaticSourcePaths(workflow.SourcePaths), 4)
				if _, exists := pageSeen[key]; !exists {
					pageSeen[key] = struct{}{}
					result.pagePatterns = append(result.pagePatterns, name)
				}
			}
			for _, region := range workflow.Regions {
				for _, control := range region.Controls {
					component := cleanProgrammaticText(control, 80)
					if component == "" {
						continue
					}
					key := strings.ToLower(component)
					result.componentPaths[key] = appendUniqueStrings(result.componentPaths[key], safeProgrammaticSourcePaths(workflow.SourcePaths), 4)
					if _, exists := componentSeen[key]; !exists {
						componentSeen[key] = struct{}{}
						result.components = append(result.components, component)
					}
				}
			}
		}
	}

	if len(result.components) > 16 {
		result.components = result.components[:16]
	}
	if len(result.pagePatterns) > 10 {
		result.pagePatterns = result.pagePatterns[:10]
	}
	return result
}

func observeProgrammaticStructure(result *programmaticObservation, source programmaticSource) {
	lowerPath := strings.ToLower("/" + filepath.ToSlash(source.path))
	lowerContent := strings.ToLower(source.content)
	base := strings.ToLower(filepath.Base(source.path))
	mark := func(signal, semantic string) {
		if signal != "" {
			result.webAdminSignals[signal] = struct{}{}
		}
		if semantic != "" {
			result.semanticPaths[semantic] = appendUniqueString(result.semanticPaths[semantic], source.path, 8)
		}
	}

	if base == "package.json" {
		switch {
		case strings.Contains(lowerContent, `"element-ui"`):
			result.framework = "Vue 2 + Element UI"
			mark("admin-framework", "framework")
		case strings.Contains(lowerContent, `"ant-design-vue"`):
			result.framework = "Vue + Ant Design"
			mark("admin-framework", "framework")
		case strings.Contains(lowerContent, `"@mui/material"`):
			result.framework = "React + Material UI"
			mark("admin-framework", "framework")
		case strings.Contains(lowerContent, `"antd"`):
			result.framework = "React + Ant Design"
			mark("admin-framework", "framework")
		case strings.Contains(lowerContent, `"vue"`):
			result.framework = "Vue"
		case strings.Contains(lowerContent, `"react"`):
			result.framework = "React"
		}
	}
	if strings.Contains(lowerPath, "/layout/") || strings.Contains(lowerPath, "/layouts/") || strings.Contains(lowerPath, "/shell/") || strings.Contains(lowerContent, "main-container") {
		mark("application-shell", "shell")
	}
	if strings.Contains(lowerPath, "sidebar") || strings.Contains(lowerContent, "sidebar-container") || strings.Contains(lowerContent, "aside-width") {
		mark("sidebar", "sidebar")
	}
	if strings.Contains(lowerPath, "navbar") || strings.Contains(lowerContent, "breadcrumb-container") || strings.Contains(lowerContent, "header-search") {
		mark("top-navigation", "navbar")
	}
	if strings.Contains(lowerContent, "<el-form") || strings.Contains(lowerContent, "<form") {
		mark("filter-form", "filter")
	}
	if strings.Contains(lowerContent, "<el-button") || strings.Contains(lowerContent, "<button") {
		mark("actions", "actions")
	}
	if strings.Contains(lowerContent, "<el-tabs") || strings.Contains(lowerContent, "__tabs") || strings.Contains(lowerContent, "tablist") {
		mark("segmented-navigation", "tabs")
	}
	if strings.Contains(lowerContent, "<el-table") || strings.Contains(lowerContent, "<table") {
		mark("data-table", "table")
	}
	if strings.Contains(lowerContent, `type="expand"`) || strings.Contains(lowerContent, "expanded-cell") {
		mark("expanded-table", "expanded-table")
	}
	if strings.Contains(lowerPath, "pagination") || strings.Contains(lowerContent, "<el-pagination") || strings.Contains(lowerContent, "pagination-container") {
		mark("pagination", "pagination")
	}
	if strings.Contains(lowerPath, "drawer") || strings.Contains(lowerContent, "<el-drawer") {
		mark("overlay-detail", "drawer")
	}
	if strings.Contains(lowerPath, "dialogstd") || strings.Contains(lowerContent, "<el-dialog") || strings.Contains(lowerContent, "message-box") {
		mark("overlay-dialog", "dialog")
	}
	if strings.Contains(lowerContent, "grid-template-columns") || strings.Contains(lowerContent, "basic-grid") || strings.Contains(lowerContent, "info-grid") {
		mark("information-grid", "info-grid")
	}
	if strings.Contains(lowerContent, "<el-tag") || strings.Contains(lowerContent, "status-tag") || strings.Contains(lowerContent, "is-effective") {
		mark("status-display", "status")
	}
	if strings.Contains(lowerContent, "v-loading") || strings.Contains(lowerContent, "empty-text") || strings.Contains(lowerContent, "暂无数据") {
		mark("feedback-state", "feedback")
	}
	if strings.Contains(lowerPath, "/styles/") || strings.Contains(lowerPath, "/theme/") {
		mark("", "style")
	}
}

func deriveProgrammaticTheme(observation programmaticObservation) programmaticTheme {
	primary := namedProgrammaticColor(observation.customProperties, []string{"color-primary", "brand-primary", "primary", "accent"})
	if primary == "" {
		primary = mostFrequentProgrammaticColor(observation.colors, false)
	}
	if primary == "" {
		primary = "#2563eb"
	}
	background := namedProgrammaticColor(observation.customProperties, []string{"page-background", "color-background", "body-bg", "background"})
	if background == "" {
		background = "#ffffff"
	}
	foreground := namedProgrammaticColor(observation.customProperties, []string{"foreground", "text-primary", "text-color", "color-text"})
	if foreground == "" {
		foreground = "#111827"
	}
	surface := namedProgrammaticColor(observation.customProperties, []string{"surface", "card-bg", "container-bg"})
	if surface == "" {
		surface = "#ffffff"
	}
	surfaceMuted := namedProgrammaticColor(observation.customProperties, []string{"muted-bg", "fill-secondary", "background-secondary"})
	if surfaceMuted == "" {
		surfaceMuted = "#f5f7fa"
	}
	mutedText := namedProgrammaticColor(observation.customProperties, []string{"text-secondary", "muted", "text-muted"})
	if mutedText == "" {
		mutedText = "#667085"
	}
	subtleText := namedProgrammaticColor(observation.customProperties, []string{"text-tertiary", "text-subtle", "placeholder"})
	if subtleText == "" {
		subtleText = "#94a3b8"
	}
	border := namedProgrammaticColor(observation.customProperties, []string{"border", "border-default", "divider", "line"})
	if border == "" {
		border = "#dfe3e8"
	}
	sidebar := namedProgrammaticColor(observation.customProperties, []string{"sidebar-background", "aside-background", "menu-bg"})
	if sidebar == "" {
		sidebar = foreground
	}
	tableHeader := namedProgrammaticColor(observation.customProperties, []string{"table-header", "table-head", "fill-secondary"})
	if tableHeader == "" {
		tableHeader = surfaceMuted
	}
	info := namedProgrammaticColor(observation.customProperties, []string{"color-info", "info"})
	if info == "" {
		info = primary
	}
	success := namedProgrammaticColor(observation.customProperties, []string{"color-success", "success"})
	if success == "" {
		success = "#16a34a"
	}
	warning := namedProgrammaticColor(observation.customProperties, []string{"color-warning", "warning"})
	if warning == "" {
		warning = "#d97706"
	}
	danger := namedProgrammaticColor(observation.customProperties, []string{"color-danger", "danger", "error"})
	if danger == "" {
		danger = "#dc2626"
	}
	font := mostFrequentString(observation.fonts)
	if font == "" {
		font = `system-ui, -apple-system, "Segoe UI", sans-serif`
	}
	radius := mostFrequentString(observation.radii)
	if radius == "" {
		radius = "8px"
	}
	components := append([]string(nil), observation.components...)
	patterns := append([]string(nil), observation.pagePatterns...)
	if len(components) == 0 {
		components = []string{"Base actions", "Content containers", "Status feedback"}
	}
	if len(patterns) == 0 {
		patterns = []string{"Core page structure"}
	}
	framework := strings.TrimSpace(observation.framework)
	if framework == "" {
		framework = "Repository UI"
	}
	return programmaticTheme{
		profile: programmaticProfileProduct, framework: framework,
		primary: primary, primaryText: contrastProgrammaticText(primary),
		background: background, surface: surface, surfaceMuted: surfaceMuted,
		foreground: foreground, mutedText: mutedText, subtleText: subtleText, border: border,
		sidebar: sidebar, tableHeader: tableHeader, info: info,
		success: success, warning: warning, danger: danger,
		fontFamily: font, fontDisplay: font, baseSpace: chooseProgrammaticBaseSpace(observation.spacings),
		borderRadius: radius, controlHeight: 40,
		sidebarWidth: namedProgrammaticPixels(observation.customProperties, []string{"sidebar-width", "aside-width"}),
		topbarHeight: namedProgrammaticPixels(observation.customProperties, []string{"topbar-height", "header-height"}),
		components:   components, componentPaths: observation.componentPaths,
		pagePatterns: patterns, pagePaths: observation.pagePaths,
		semanticPaths: observation.semanticPaths,
	}
}

func writeProgrammaticPackage(outputDir string, input ProgrammaticInput, sources []programmaticSource, observation programmaticObservation, theme programmaticTheme) error {
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("reset programmatic output: %w", err)
	}
	for _, relative := range []string{"source", "ui-kit", "preview"} {
		if err := os.MkdirAll(filepath.Join(outputDir, relative), 0o755); err != nil {
			return err
		}
	}
	write := func(relative, body string) error {
		return os.WriteFile(filepath.Join(outputDir, filepath.FromSlash(relative)), []byte(body), 0o644)
	}

	if err := write("DESIGN.md", buildProgrammaticDesignMarkdown(input, sources, theme)); err != nil {
		return err
	}
	if err := write("tokens.css", buildProgrammaticTokensCSS(theme)); err != nil {
		return err
	}
	sourceIndex, err := buildProgrammaticSourceIndex(input, sources, observation)
	if err != nil {
		return err
	}
	if err := write("source/index.json", sourceIndex); err != nil {
		return err
	}
	if err := write("ui-kit/index.html", buildProgrammaticUIKit(input, theme)); err != nil {
		return err
	}
	if err := write("preview/page-patterns.html", buildProgrammaticPatternsPreview(input, theme)); err != nil {
		return err
	}
	if err := write("design-tokens.json", buildProgrammaticTokensJSON(theme)); err != nil {
		return err
	}
	if err := write("components.manifest.json", buildProgrammaticComponentsJSON(theme)); err != nil {
		return err
	}
	return write("USAGE.md", buildProgrammaticUsage(input))
}

func programmaticPrinciples(analysis *RepositoryDesignContext) []string {
	principles := make([]string, 0, 8)
	kinds := make(map[string]struct{})
	if analysis != nil {
		for _, fact := range analysis.Facts {
			kinds[strings.ToLower(strings.TrimSpace(fact.Kind))] = struct{}{}
		}
	}
	has := func(values ...string) bool {
		for _, value := range values {
			if _, exists := kinds[value]; exists {
				return true
			}
		}
		return false
	}
	if has("product", "scope") {
		principles = append(principles, "设计体系只覆盖该仓库真实承担的产品范围，不把仓库之外的界面或一次性活动风格提升为全局规则。")
	}
	if has("architecture", "route", "routing") {
		principles = append(principles, "组件、页面模式和交互必须兼容仓库现有技术架构、路由方式与服务端渲染边界。")
	}
	if has("layout", "viewport", "responsive") {
		principles = append(principles, "布局优先遵守仓库已有画布宽度、响应规则、安全区和固定操作区，不用桌面端模式替代移动端结构。")
	}
	if has("typography", "font") {
		principles = append(principles, "字体家族、字号层级和字重以全局样式与高频真实页面为准，确保中文内容清晰可读。")
	}
	if has("color", "surface", "theme") {
		principles = append(principles, "主色、语义色、页面背景和内容表面从仓库高频证据中提取，局部主题不得破坏核心交互语义。")
	}
	if has("component", "control", "navigation", "interaction", "state") {
		principles = append(principles, "共享组件必须覆盖默认、悬停、禁用、加载、空和失败状态，并保持导航与反馈行为一致。")
	}
	for _, fallback := range []string{
		"以仓库中重复出现的视觉变量和共享组件为基础，不把单个页面的偶然写法当作设计体系。",
		"快速草稿只陈述可追溯事实；证据不足的部分使用明确默认值，并留给 AI 深度优化继续判断。",
		"设计规则、Tokens、组件状态、页面模式和 UI Kit 必须同步演进，不能只修改展示表面。",
	} {
		principles = appendUniqueString(principles, fallback, 8)
	}
	return principles
}

func buildProgrammaticDesignMarkdown(input ProgrammaticInput, sources []programmaticSource, theme programmaticTheme) string {
	name := cleanProgrammaticText(firstNonEmpty(input.RepositoryName, input.ProjectName, "Repository"), 120)
	project := cleanProgrammaticText(input.ProjectName, 120)
	brief := cleanProgrammaticText(input.Brief, 1600)
	description := cleanProgrammaticText(input.ProjectDescription, 800)
	commit := cleanProgrammaticText(input.CommitSHA, 80)
	platform := cleanProgrammaticText(input.Platform, 40)
	principles := programmaticPrinciples(input.RepositoryAnalysis)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s 设计体系\n\n", markdownProgrammatic(name))
	b.WriteString("## 体系身份\n\n")
	fmt.Fprintf(&b, "- 项目：%s\n", markdownProgrammatic(firstNonEmpty(project, "未命名项目")))
	fmt.Fprintf(&b, "- 仓库：%s\n", markdownProgrammatic(name))
	fmt.Fprintf(&b, "- 固定提交：`%s`\n", markdownProgrammatic(firstNonEmpty(commit, "未记录")))
	fmt.Fprintf(&b, "- 目标平台：%s\n", markdownProgrammatic(firstNonEmpty(platform, "web")))
	if description != "" {
		fmt.Fprintf(&b, "- 产品背景：%s\n", markdownProgrammatic(description))
	}
	if brief != "" {
		fmt.Fprintf(&b, "- 本次目标：%s\n", markdownProgrammatic(brief))
	}
	if input.RepositoryAnalysis != nil {
		fmt.Fprintf(&b, "- 仓库证据：已复用 %d 条结构化事实和 %d 个来源文件；原始内容保留在来源索引中。\n", len(input.RepositoryAnalysis.Facts), len(input.RepositoryAnalysis.SourceFiles))
	}

	b.WriteString("\n## 设计原则\n\n")
	for _, principle := range principles {
		fmt.Fprintf(&b, "- %s\n", markdownProgrammatic(principle))
	}

	b.WriteString("\n## 色彩\n\n")
	fmt.Fprintf(&b, "- 主操作使用 `%s`，前景使用 `%s`。\n", theme.primary, theme.primaryText)
	fmt.Fprintf(&b, "- 页面背景使用 `%s`，内容表面使用 `%s`，弱化表面使用 `%s`。\n", theme.background, theme.surface, theme.surfaceMuted)
	fmt.Fprintf(&b, "- 正文使用 `%s`，辅助文本使用 `%s`，边界使用 `%s`。\n", theme.foreground, theme.mutedText, theme.border)

	b.WriteString("\n## 字体\n\n")
	fmt.Fprintf(&b, "- 正文与界面字体：`%s`。\n", markdownProgrammatic(theme.fontFamily))
	b.WriteString("- 使用清晰的标题、正文、说明三级层级；紧凑信息不通过缩小到不可读字号解决。\n")

	b.WriteString("\n## 间距与布局\n\n")
	fmt.Fprintf(&b, "- 基础间距单位为 `%dpx`，常用节奏为 `%dpx / %dpx / %dpx / %dpx`。\n", theme.baseSpace, theme.baseSpace, theme.baseSpace*2, theme.baseSpace*3, theme.baseSpace*4)
	fmt.Fprintf(&b, "- 基础圆角为 `%s`，标准控件高度为 `%dpx`。\n", theme.borderRadius, theme.controlHeight)
	b.WriteString("- 页面应优先保持清楚的信息层级和稳定内容宽度，再处理装饰性变化。\n")

	b.WriteString("\n## 组件与状态\n\n")
	for _, component := range theme.components {
		fmt.Fprintf(&b, "- %s：沿用仓库已有结构，并补齐默认、悬停、禁用、加载和错误状态。\n", markdownProgrammatic(component))
	}

	b.WriteString("\n## 页面模式\n\n")
	for _, pattern := range theme.pagePatterns {
		fmt.Fprintf(&b, "- %s：作为仓库代表性页面结构，在 UI Kit 中验证 Tokens 与组件组合。\n", markdownProgrammatic(pattern))
	}

	b.WriteString("\n## 来源与边界\n\n")
	fmt.Fprintf(&b, "本快速草稿有界读取了 %d 个高信号文件。完整来源摘要位于 `source/index.json`。", len(sources))
	b.WriteString("程序化结果用于尽快提供可查看草稿；复杂业务语义、冲突取舍和高保真质量由后续 AI 深度优化完成。\n")
	return b.String()
}

func buildProgrammaticTokensCSS(theme programmaticTheme) string {
	space := theme.baseSpace
	css := fmt.Sprintf(`:root {
  --color-primary: %s;
  --color-primary-contrast: %s;
  --color-background: %s;
  --color-surface: %s;
  --color-surface-muted: %s;
  --color-text: %s;
  --color-text-muted: %s;
  --color-text-subtle: %s;
  --color-border: %s;
  --color-sidebar: %s;
  --color-table-header: %s;
  --color-info: %s;
  --color-success: %s;
  --color-warning: %s;
  --color-danger: %s;
  --font-family-body: %s;
  --font-family-display: %s;
  --font-size-caption: 12px;
  --font-size-body: 14px;
  --font-size-title: 20px;
  --font-size-display: 32px;
  --line-height-tight: 1.25;
  --line-height-body: 1.6;
  --space-1: %dpx;
  --space-2: %dpx;
  --space-3: %dpx;
  --space-4: %dpx;
  --space-6: %dpx;
  --space-8: %dpx;
  --radius-sm: 4px;
  --radius-md: %s;
  --radius-lg: 16px;
  --control-height: %dpx;
  --shadow-card: 0 10px 30px rgba(15, 23, 42, 0.08);
`, theme.primary, theme.primaryText, theme.background, theme.surface, theme.surfaceMuted,
		theme.foreground, theme.mutedText, theme.subtleText, theme.border, theme.sidebar, theme.tableHeader, theme.info,
		theme.success, theme.warning, theme.danger,
		theme.fontFamily, theme.fontDisplay, space, space*2, space*3, space*4, space*6, space*8,
		theme.borderRadius, theme.controlHeight)
	if theme.sidebarWidth > 0 {
		css += fmt.Sprintf("  --sidebar-width: %dpx;\n", theme.sidebarWidth)
	}
	if theme.topbarHeight > 0 {
		css += fmt.Sprintf("  --topbar-height: %dpx;\n", theme.topbarHeight)
	}
	return css + "}\n"
}

func buildProgrammaticSourceIndex(input ProgrammaticInput, sources []programmaticSource, observation programmaticObservation) (string, error) {
	index := SourceIndex{
		SchemaVersion:       SourceIndexSchemaV1,
		InputSnapshotSHA256: input.InputSnapshotSHA256,
		Evidence:            []SourceEvidence{},
		Conflicts:           []SourceConflict{},
		Fallbacks:           []SourceFallback{},
	}
	if input.RepositoryAnalysis != nil {
		for factIndex, fact := range input.RepositoryAnalysis.Facts {
			paths := safeProgrammaticSourcePaths(fact.SourcePaths)
			if len(paths) == 0 {
				continue
			}
			index.Evidence = append(index.Evidence, SourceEvidence{
				ID:         fmt.Sprintf("analysis-fact-%02d", factIndex+1),
				Kind:       firstNonEmpty(cleanProgrammaticID(fact.Kind), "repository_fact"),
				Summary:    cleanProgrammaticText(strings.TrimSpace(fact.Label+": "+fact.Value), 600),
				References: paths,
			})
			if len(index.Evidence) >= 20 {
				break
			}
		}
		for conflictIndex, conflict := range input.RepositoryAnalysis.Conflicts {
			paths := safeProgrammaticSourcePaths(conflict.SourcePaths)
			if len(paths) == 0 {
				continue
			}
			index.Conflicts = append(index.Conflicts, SourceConflict{
				ID:         fmt.Sprintf("analysis-conflict-%02d", conflictIndex+1),
				Summary:    cleanProgrammaticText(strings.TrimSpace(conflict.Label+": "+conflict.RepositoryFact+"；用户意图："+conflict.UserIntent), 800),
				References: paths,
			})
		}
	}
	if len(sources) > 0 {
		categoryNames := map[string]string{
			"foundation": "基础样式与项目配置",
			"component":  "共享组件",
			"page":       "代表性页面",
			"style":      "页面样式",
			"context":    "项目说明",
		}
		for _, category := range []string{"foundation", "component", "page", "style", "context"} {
			paths := make([]string, 0, 12)
			count := 0
			for _, source := range sources {
				if source.kind != category {
					continue
				}
				count++
				paths = appendUniqueString(paths, source.path, 12)
			}
			if len(paths) == 0 {
				continue
			}
			index.Evidence = append(index.Evidence, SourceEvidence{
				ID:         "programmatic-" + category,
				Kind:       "repository_fact",
				Summary:    fmt.Sprintf("快速引擎有界读取了 %d 个%s文件。", count, categoryNames[category]),
				References: paths,
			})
		}
		if commit := strings.TrimSpace(input.CommitSHA); commit != "" {
			index.Evidence = append(index.Evidence, SourceEvidence{
				ID: "repository-commit", Kind: "repository_commit",
				Summary:    "快速草稿固定到仓库 Commit " + commit + "。",
				References: []string{sources[0].path},
			})
		}
	} else {
		index.Fallbacks = append(index.Fallbacks, SourceFallback{
			ID:      "frozen-analysis-only",
			Summary: "本次无法重新打开仓库 checkout，快速草稿使用已冻结的仓库分析事实生成。",
		})
	}
	if len(observation.colors) == 0 {
		index.Fallbacks = append(index.Fallbacks, SourceFallback{ID: "default-color-seed", Summary: "没有提取到稳定颜色字面量，快速草稿使用安全的中性基础色和蓝色主操作。"})
	}
	if len(observation.fonts) == 0 {
		index.Fallbacks = append(index.Fallbacks, SourceFallback{ID: "system-font-stack", Summary: "没有提取到稳定字体声明，快速草稿使用系统字体栈。"})
	}
	if len(observation.components) == 0 {
		index.Fallbacks = append(index.Fallbacks, SourceFallback{ID: "minimal-component-set", Summary: "没有识别到稳定共享组件入口，UI Kit 仅展示最小基础状态，等待 AI 深度优化。"})
	}
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded) + "\n", nil
}

func buildProgrammaticTokensJSON(theme programmaticTheme) string {
	value := map[string]any{
		"profile":   theme.profile,
		"framework": theme.framework,
		"color": map[string]string{
			"primary": theme.primary, "background": theme.background, "surface": theme.surface,
			"surfaceMuted": theme.surfaceMuted, "text": theme.foreground, "mutedText": theme.mutedText,
			"subtleText": theme.subtleText, "border": theme.border, "sidebar": theme.sidebar,
			"tableHeader": theme.tableHeader, "info": theme.info, "success": theme.success,
			"warning": theme.warning, "danger": theme.danger,
		},
		"typography": map[string]any{"body": theme.fontFamily, "display": theme.fontDisplay},
		"spacing":    map[string]int{"base": theme.baseSpace, "controlHeight": theme.controlHeight},
		"radius":     map[string]string{"base": theme.borderRadius},
	}
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return string(encoded) + "\n"
}

func buildProgrammaticComponentsJSON(theme programmaticTheme) string {
	components := make([]map[string]any, 0, len(theme.components))
	for index, name := range theme.components {
		components = append(components, map[string]any{
			"id":           fmt.Sprintf("component-%02d", index+1),
			"name":         name,
			"source_paths": safeProgrammaticSourcePaths(theme.componentPaths[strings.ToLower(name)]),
			"states":       []string{"default", "hover", "disabled", "loading", "error"},
		})
	}
	value := map[string]any{"schema_version": "multica.programmatic-components/v1", "components": components}
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return string(encoded) + "\n"
}

func buildProgrammaticUsage(input ProgrammaticInput) string {
	name := cleanProgrammaticText(firstNonEmpty(input.RepositoryName, input.ProjectName, "Repository"), 120)
	return fmt.Sprintf("# %s 设计体系使用说明\n\n1. `DESIGN.md` 是人和 Agent 共用的设计规则。\n2. `tokens.css` 是唯一 Token 事实源。\n3. `ui-kit/index.html` 和 `preview/page-patterns.html` 用于真实浏览器验收。\n4. 快速草稿通过验证后可以保存；AI 深度优化产生新的草稿，不会覆盖最近一次已保存版本。\n", markdownProgrammatic(name))
}

func normalizeProgrammaticHex(value string) string {
	match := programmaticHexColorPattern.FindString(strings.TrimSpace(value))
	if match == "" {
		return ""
	}
	value = strings.ToLower(match)
	if len(value) == 4 {
		return fmt.Sprintf("#%c%c%c%c%c%c", value[1], value[1], value[2], value[2], value[3], value[3])
	}
	if len(value) == 9 {
		return value[:7]
	}
	return value
}

func normalizeProgrammaticFont(value string) string {
	for _, part := range strings.Split(value, ",") {
		candidate := strings.Trim(strings.TrimSpace(part), `"'`)
		lower := strings.ToLower(candidate)
		if candidate == "" || strings.Contains(candidate, "var(") || lower == "inherit" || lower == "sans-serif" || lower == "serif" || lower == "monospace" || lower == "system-ui" {
			continue
		}
		for _, r := range candidate {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '_') {
				candidate = ""
				break
			}
		}
		if candidate != "" {
			return fmt.Sprintf(`"%s", system-ui, -apple-system, "Segoe UI", sans-serif`, candidate)
		}
	}
	return ""
}

func namedProgrammaticPixels(properties map[string]string, terms []string) int {
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, term := range terms {
		for _, key := range keys {
			if !strings.Contains(key, term) {
				continue
			}
			value := strings.TrimSpace(strings.ToLower(properties[key]))
			if !strings.HasSuffix(value, "px") {
				continue
			}
			pixels, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(value, "px")))
			if err == nil && pixels > 0 && pixels <= 2048 {
				return pixels
			}
		}
	}
	return 0
}

func namedProgrammaticColor(properties map[string]string, terms []string) string {
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, term := range terms {
		for _, key := range keys {
			if strings.Contains(key, term) {
				if color := normalizeProgrammaticHex(properties[key]); color != "" {
					return color
				}
			}
		}
	}
	return ""
}

func mostFrequentProgrammaticColor(values map[string]int, allowNeutral bool) string {
	type entry struct {
		value string
		count int
	}
	entries := make([]entry, 0, len(values))
	for value, count := range values {
		if !allowNeutral && programmaticNeutralColor(value) {
			continue
		}
		entries = append(entries, entry{value: value, count: count})
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].count != entries[right].count {
			return entries[left].count > entries[right].count
		}
		return entries[left].value < entries[right].value
	})
	if len(entries) == 0 {
		return ""
	}
	return entries[0].value
}

func programmaticNeutralColor(value string) bool {
	if len(value) != 7 {
		return false
	}
	red, err1 := strconv.ParseInt(value[1:3], 16, 64)
	green, err2 := strconv.ParseInt(value[3:5], 16, 64)
	blue, err3 := strconv.ParseInt(value[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	maxValue := maxInt64(red, green, blue)
	minValue := minInt64(red, green, blue)
	return maxValue-minValue < 14 || maxValue < 28 || minValue > 238
}

func contrastProgrammaticText(color string) string {
	if len(color) != 7 {
		return "#ffffff"
	}
	red, err1 := strconv.ParseInt(color[1:3], 16, 64)
	green, err2 := strconv.ParseInt(color[3:5], 16, 64)
	blue, err3 := strconv.ParseInt(color[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return "#ffffff"
	}
	luma := 0.299*float64(red) + 0.587*float64(green) + 0.114*float64(blue)
	if luma > 170 {
		return "#111827"
	}
	return "#ffffff"
}

func mostFrequentString(values map[string]int) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool {
		if values[keys[left]] != values[keys[right]] {
			return values[keys[left]] > values[keys[right]]
		}
		return keys[left] < keys[right]
	})
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func chooseProgrammaticBaseSpace(values map[int]int) int {
	for _, preferred := range []int{8, 4, 6, 12} {
		if values[preferred] > 0 {
			return preferred
		}
	}
	bestValue, bestCount := 8, 0
	for value, count := range values {
		if count > bestCount || (count == bestCount && value < bestValue) {
			bestValue, bestCount = value, count
		}
	}
	if bestValue < 4 {
		return 4
	}
	if bestValue > 12 {
		return 8
	}
	return bestValue
}

func componentNameFromPath(relative string) string {
	ext := strings.ToLower(filepath.Ext(relative))
	if ext == ".md" || ext == ".json" {
		return ""
	}
	lower := strings.ToLower("/" + filepath.ToSlash(relative))
	if !strings.Contains(lower, "/components/") && !strings.Contains(lower, "/ui/") && !strings.Contains(lower, "/widgets/") {
		return ""
	}
	name := strings.TrimSuffix(filepath.Base(relative), filepath.Ext(relative))
	name = strings.TrimSuffix(name, ".module")
	if strings.EqualFold(name, "index") {
		name = filepath.Base(filepath.Dir(relative))
	}
	if strings.Contains(strings.ToLower(name), "test") || strings.Contains(strings.ToLower(name), "story") || strings.Contains(strings.ToLower(name), "type") {
		return ""
	}
	return humanizeProgrammaticIdentifier(name)
}

func pagePatternFromPath(relative string) string {
	lower := strings.ToLower("/" + filepath.ToSlash(relative))
	ext := strings.ToLower(filepath.Ext(relative))
	if ext != ".tsx" && ext != ".jsx" && ext != ".ts" && ext != ".js" && ext != ".vue" && ext != ".svelte" {
		return ""
	}
	if strings.Contains(lower, "/components/") || strings.Contains(lower, "/ui/") || strings.Contains(lower, "/widgets/") {
		return ""
	}
	if !strings.Contains(lower, "/pages/") && !strings.Contains(lower, "/app/") && !strings.Contains(lower, "/views/") && !strings.Contains(lower, "/screens/") && !strings.Contains(lower, "/routes/") {
		return ""
	}
	name := strings.TrimSuffix(filepath.Base(relative), ext)
	if strings.EqualFold(name, "index") || strings.EqualFold(name, "page") || strings.EqualFold(name, "route") || (strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]")) {
		name = filepath.Base(filepath.Dir(relative))
	}
	if strings.HasPrefix(name, "_") || strings.Contains(strings.ToLower(name), "test") || strings.Contains(strings.ToLower(name), "spec") {
		return ""
	}
	return humanizeProgrammaticIdentifier(name)
}

func humanizeProgrammaticIdentifier(value string) string {
	var b strings.Builder
	previousLower := false
	for _, r := range value {
		switch {
		case unicode.IsUpper(r) && previousLower:
			b.WriteByte(' ')
			b.WriteRune(r)
			previousLower = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			previousLower = unicode.IsLower(r)
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") {
				b.WriteByte(' ')
			}
			previousLower = false
		}
	}
	cleaned := cleanProgrammaticText(b.String(), 80)
	runes := []rune(cleaned)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

func safeProgrammaticSourcePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, value := range paths {
		value = filepath.ToSlash(strings.TrimSpace(value))
		if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "..") || strings.Contains(value, ":") || value == "." {
			continue
		}
		out = appendUniqueString(out, value, 20)
	}
	return out
}

func appendUniqueStrings(current []string, values []string, limit int) []string {
	for _, value := range values {
		current = appendUniqueString(current, value, limit)
	}
	return current
}

func appendUniqueString(current []string, value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" || len(current) >= limit {
		return current
	}
	for _, existing := range current {
		if existing == value {
			return current
		}
	}
	return append(current, value)
}

func cleanProgrammaticText(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)), " ")
	runes := []rune(value)
	if len(runes) > limit {
		value = strings.TrimSpace(string(runes[:limit]))
	}
	return value
}

func cleanProgrammaticID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == ':' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func markdownProgrammatic(value string) string {
	return strings.NewReplacer("`", "'", "\r", " ", "\n", " ").Replace(value)
}

func shortProgrammaticCommit(value string) string {
	value = cleanProgrammaticText(value, 80)
	if len(value) > 12 {
		return value[:12]
	}
	return firstNonEmpty(value, "未记录")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxInt64(values ...int64) int64 {
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}

func minInt64(values ...int64) int64 {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
