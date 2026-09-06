package projectdesignsystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
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
}

type programmaticTheme struct {
	primary        string
	primaryText    string
	background     string
	surface        string
	surfaceMuted   string
	foreground     string
	mutedText      string
	border         string
	success        string
	warning        string
	danger         string
	fontFamily     string
	fontDisplay    string
	baseSpace      int
	borderRadius   string
	controlHeight  int
	components     []string
	componentPaths map[string][]string
	pagePatterns   []string
	pagePaths      map[string][]string
}

var (
	programmaticCustomPropertyPattern = regexp.MustCompile(`(?mi)(--[a-z0-9_-]+)\s*:\s*([^;{}\n]+)`)
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
	for _, term := range []string{"/theme/", "/themes/", "/tokens/", "/design-system/", "/design_system/"} {
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
		score += 120
	case "readme.md", "design.md", "ai_context.md":
		score += 80
	case "tailwind.config.js", "tailwind.config.ts", "theme.ts", "theme.js", "tokens.css", "variables.css", "globals.css", "global.css":
		score += 150
	}
	for _, term := range []string{"/styles/", "/theme/", "/themes/", "/tokens/", "/design-system/", "/design_system/"} {
		if strings.Contains("/"+lower, term) {
			score += 100
		}
	}
	for _, term := range []string{"/components/", "/ui/", "/widgets/"} {
		if strings.Contains("/"+lower, term) {
			score += 65
		}
	}
	for _, term := range []string{"/pages/", "/app/", "/views/", "/screens/", "/routes/"} {
		if strings.Contains("/"+lower, term) {
			score += 45
		}
	}
	for _, term := range []string{"button", "input", "card", "nav", "header", "tab", "modal", "dialog", "list", "detail", "home", "index", "layout"} {
		if strings.Contains(base, term) {
			score += 15
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

func deriveProgrammaticTheme(observation programmaticObservation) programmaticTheme {
	primary := namedProgrammaticColor(observation.customProperties, []string{"primary", "brand", "accent"})
	if primary == "" {
		primary = mostFrequentProgrammaticColor(observation.colors, false)
	}
	if primary == "" {
		primary = "#2563eb"
	}
	background := namedProgrammaticColor(observation.customProperties, []string{"background", "page-bg", "body-bg", "bg-base"})
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
	border := namedProgrammaticColor(observation.customProperties, []string{"border", "divider", "line"})
	if border == "" {
		border = "#dfe3e8"
	}
	font := mostFrequentString(observation.fonts)
	if font == "" {
		font = `system-ui, -apple-system, "Segoe UI", sans-serif`
	}
	radius := mostFrequentString(observation.radii)
	if radius == "" {
		radius = "8px"
	}
	baseSpace := chooseProgrammaticBaseSpace(observation.spacings)
	components := append([]string(nil), observation.components...)
	if len(components) == 0 {
		components = []string{"基础操作", "内容容器", "状态反馈"}
	}
	patterns := append([]string(nil), observation.pagePatterns...)
	if len(patterns) == 0 {
		patterns = []string{"核心页面结构"}
	}
	return programmaticTheme{
		primary: primary, primaryText: contrastProgrammaticText(primary),
		background: background, surface: surface, surfaceMuted: surfaceMuted,
		foreground: foreground, mutedText: mutedText, border: border,
		success: "#16a34a", warning: "#d97706", danger: "#dc2626",
		fontFamily: font, fontDisplay: font, baseSpace: baseSpace,
		borderRadius: radius, controlHeight: 40,
		components: components, componentPaths: observation.componentPaths,
		pagePatterns: patterns, pagePaths: observation.pagePaths,
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

func buildProgrammaticDesignMarkdown(input ProgrammaticInput, sources []programmaticSource, theme programmaticTheme) string {
	name := cleanProgrammaticText(firstNonEmpty(input.RepositoryName, input.ProjectName, "Repository"), 120)
	project := cleanProgrammaticText(input.ProjectName, 120)
	brief := cleanProgrammaticText(input.Brief, 1600)
	description := cleanProgrammaticText(input.ProjectDescription, 800)
	commit := cleanProgrammaticText(input.CommitSHA, 80)
	platform := cleanProgrammaticText(input.Platform, 40)
	analysisSummary := ""
	principles := make([]string, 0, 8)
	if input.RepositoryAnalysis != nil {
		analysisSummary = cleanProgrammaticText(input.RepositoryAnalysis.Summary, 1200)
		for _, fact := range input.RepositoryAnalysis.Facts {
			value := cleanProgrammaticText(fact.Value, 280)
			if value != "" {
				principles = appendUniqueString(principles, value, 8)
			}
		}
	}
	if len(principles) == 0 {
		principles = []string{
			"以仓库中重复出现的视觉变量和共享组件为基础，不把一次性页面样式提升为全局规则。",
			"快速草稿只陈述可追溯事实；证据不足的部分保持为明确的默认值，等待 AI 深度优化。",
			"组件和页面模式必须共同使用同一套 Tokens，避免预览与事实源分离。",
		}
	}

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
	if analysisSummary != "" {
		fmt.Fprintf(&b, "\n%s\n", markdownProgrammatic(analysisSummary))
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
	return fmt.Sprintf(`:root {
  --color-primary: %s;
  --color-primary-contrast: %s;
  --color-background: %s;
  --color-surface: %s;
  --color-surface-muted: %s;
  --color-text: %s;
  --color-text-muted: %s;
  --color-border: %s;
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
}
`, theme.primary, theme.primaryText, theme.background, theme.surface, theme.surfaceMuted,
		theme.foreground, theme.mutedText, theme.border, theme.success, theme.warning, theme.danger,
		theme.fontFamily, theme.fontDisplay, space, space*2, space*3, space*4, space*6, space*8,
		theme.borderRadius, theme.controlHeight)
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

func buildProgrammaticUIKit(input ProgrammaticInput, theme programmaticTheme) string {
	name := html.EscapeString(cleanProgrammaticText(firstNonEmpty(input.RepositoryName, input.ProjectName, "Repository"), 120))
	commit := html.EscapeString(shortProgrammaticCommit(input.CommitSHA))
	var components strings.Builder
	for index, component := range theme.components {
		fmt.Fprintf(&components, `<article class="specimen" data-design-node-id="kit-component-%d" data-design-node-kind="component" data-design-node-label="%s"><span class="component-index">%02d</span><div><strong>%s</strong><p>来源仓库的共享组件模式</p></div><span class="state">Default · Hover · Disabled</span></article>`, index+1, html.EscapeString(component), index+1, html.EscapeString(component))
	}
	var patterns strings.Builder
	for index, pattern := range theme.pagePatterns {
		fmt.Fprintf(&patterns, `<article class="pattern" data-design-node-id="kit-pattern-%d" data-design-node-kind="block" data-design-node-label="%s"><span>页面模式 %02d</span><strong>%s</strong><p>使用同一组 Tokens 组织导航、内容与状态反馈。</p></article>`, index+1, html.EscapeString(pattern), index+1, html.EscapeString(pattern))
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s 设计体系</title>
  <style>
    * { box-sizing: border-box; }
    body { margin: 0; background: var(--color-background); color: var(--color-text); font-family: var(--font-family-body); }
    main { min-height: 100vh; padding: var(--space-8); background: linear-gradient(180deg, var(--color-surface-muted), var(--color-background) 42%%); }
    .shell { max-width: 1180px; margin: 0 auto; }
    .eyebrow, .section-label, .pattern span { color: var(--color-text-muted); font-size: var(--font-size-caption); letter-spacing: .08em; text-transform: uppercase; }
    h1, h2, p { margin: 0; }
    h1 { margin-top: var(--space-2); font-family: var(--font-family-display); font-size: var(--font-size-display); line-height: var(--line-height-tight); }
    h2 { font-size: var(--font-size-title); }
    .hero { display: grid; gap: var(--space-6); padding: var(--space-8); border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); box-shadow: var(--shadow-card); }
    .hero-row { display: flex; flex-wrap: wrap; align-items: end; justify-content: space-between; gap: var(--space-4); }
    .meta { display: flex; gap: var(--space-2); flex-wrap: wrap; }
    .chip, .state { border: 1px solid var(--color-border); border-radius: 999px; padding: var(--space-1) var(--space-2); color: var(--color-text-muted); font-size: var(--font-size-caption); }
    section { margin-top: var(--space-8); }
    .section-head { display: flex; justify-content: space-between; align-items: baseline; gap: var(--space-4); margin-bottom: var(--space-4); }
    .palette { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--space-3); }
    .swatch { min-height: 132px; display: flex; align-items: end; border: 1px solid var(--color-border); border-radius: var(--radius-md); overflow: hidden; }
    .swatch-primary { background: var(--color-primary); }
    .swatch-background { background: var(--color-background); }
    .swatch-muted { background: var(--color-surface-muted); }
    .swatch-text { background: var(--color-text); }
    .swatch div { width: 100%%; padding: var(--space-2); background: var(--color-surface); border-top: 1px solid var(--color-border); font-size: var(--font-size-caption); }
    .type-card { display: grid; grid-template-columns: 1.4fr 1fr; gap: var(--space-4); padding: var(--space-6); border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface); }
    .display { font-family: var(--font-family-display); font-size: var(--font-size-display); line-height: var(--line-height-tight); }
    .body-copy { color: var(--color-text-muted); line-height: var(--line-height-body); }
    .components { display: grid; gap: var(--space-2); }
    .specimen { min-height: var(--control-height); display: grid; grid-template-columns: 48px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); padding: var(--space-3); border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface); }
    .component-index { color: var(--color-primary); font-weight: 700; }
    .specimen p, .pattern p { margin-top: var(--space-1); color: var(--color-text-muted); font-size: var(--font-size-caption); }
    .patterns { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--space-3); }
    .pattern { min-height: 170px; padding: var(--space-4); border-radius: var(--radius-md); background: var(--color-primary); color: var(--color-primary-contrast); }
    .pattern span, .pattern p { color: inherit; opacity: .78; }
    .pattern strong { display: block; margin-top: var(--space-6); font-size: var(--font-size-title); }
    @media (max-width: 760px) { main { padding: var(--space-4); } .palette, .patterns, .type-card { grid-template-columns: 1fr; } .specimen { grid-template-columns: 36px 1fr; } .state { grid-column: 2; justify-self: start; } }
  </style>
</head>
<body>
  <main>
    <div class="shell">
      <header class="hero" data-design-node-id="kit-identity" data-design-node-kind="block" data-design-node-label="设计体系身份">
        <div class="hero-row"><div><span class="eyebrow">Repository design system · Quick draft</span><h1>%s</h1></div><div class="meta"><span class="chip">Commit %s</span><span class="chip">程序化快速生成</span></div></div>
        <p class="body-copy">基于固定仓库和高信号源码生成的第一份可查看结果。AI 深度优化可以在此基础上继续完善业务语义与设计质量。</p>
      </header>
      <section data-design-node-id="kit-palette" data-design-node-kind="block" data-design-node-label="色彩 Tokens"><div class="section-head"><div><span class="section-label">Foundation 01</span><h2>色彩 Tokens</h2></div><span class="chip">Source backed</span></div><div class="palette"><div class="swatch swatch-primary"><div>Primary<br>%s</div></div><div class="swatch swatch-background"><div>Background<br>%s</div></div><div class="swatch swatch-muted"><div>Muted surface<br>%s</div></div><div class="swatch swatch-text"><div>Text<br>%s</div></div></div></section>
      <section data-design-node-id="kit-typography" data-design-node-kind="block" data-design-node-label="字体层级"><div class="section-head"><div><span class="section-label">Foundation 02</span><h2>字体与信息层级</h2></div></div><div class="type-card"><div class="display">清楚、稳定、可复用</div><p class="body-copy">界面正文保持舒适可读，标题建立明确层级。复杂信息通过结构与间距组织，而不是依赖装饰性噪音。</p></div></section>
      <section data-design-node-id="kit-components" data-design-node-kind="block" data-design-node-label="组件状态"><div class="section-head"><div><span class="section-label">System 01</span><h2>仓库组件与状态</h2></div><span class="chip">%d patterns</span></div><div class="components">%s</div></section>
      <section data-design-node-id="kit-patterns" data-design-node-kind="block" data-design-node-label="页面模式"><div class="section-head"><div><span class="section-label">System 02</span><h2>代表性页面模式</h2></div></div><div class="patterns">%s</div></section>
    </div>
  </main>
</body>
</html>
`, name, name, commit, theme.primary, theme.background, theme.surfaceMuted, theme.foreground, len(theme.components), components.String(), patterns.String())
}

func buildProgrammaticPatternsPreview(input ProgrammaticInput, theme programmaticTheme) string {
	name := html.EscapeString(cleanProgrammaticText(firstNonEmpty(input.RepositoryName, input.ProjectName, "Repository"), 120))
	var patterns strings.Builder
	for index, pattern := range theme.pagePatterns {
		fmt.Fprintf(&patterns, `<article class="page" data-design-node-id="preview-pattern-%d" data-design-node-kind="block" data-design-node-label="%s"><div class="chrome"><span></span><span></span><span></span></div><div class="nav">%s</div><div class="content"><span class="kicker">Pattern %02d</span><h2>%s</h2><div class="rows"><i></i><i></i><i></i></div></div></article>`, index+1, html.EscapeString(pattern), name, index+1, html.EscapeString(pattern))
	}
	return fmt.Sprintf(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>%s 页面模式</title><style>*{box-sizing:border-box}body{margin:0;background:var(--color-background);color:var(--color-text);font-family:var(--font-family-body)}main{min-height:100vh;padding:var(--space-8)}header{max-width:1180px;margin:0 auto var(--space-6)}h1,h2,p{margin:0}h1{font-size:var(--font-size-display);font-family:var(--font-family-display)}p{margin-top:var(--space-2);color:var(--color-text-muted)}.grid{max-width:1180px;margin:0 auto;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:var(--space-4)}.page{overflow:hidden;border:1px solid var(--color-border);border-radius:var(--radius-lg);background:var(--color-surface);box-shadow:var(--shadow-card)}.chrome{display:flex;gap:var(--space-1);padding:var(--space-2);border-bottom:1px solid var(--color-border);background:var(--color-surface-muted)}.chrome span{width:8px;height:8px;border-radius:50%%;background:var(--color-border)}.nav{padding:var(--space-3);background:var(--color-primary);color:var(--color-primary-contrast);font-weight:700}.content{padding:var(--space-6)}.kicker{font-size:var(--font-size-caption);color:var(--color-text-muted);text-transform:uppercase;letter-spacing:.08em}h2{margin-top:var(--space-2);font-size:var(--font-size-title)}.rows{display:grid;gap:var(--space-2);margin-top:var(--space-6)}.rows i{display:block;height:38px;border-radius:var(--radius-md);background:var(--color-surface-muted);border:1px solid var(--color-border)}@media(max-width:760px){main{padding:var(--space-4)}.grid{grid-template-columns:1fr}}</style></head><body><main data-design-node-id="preview-patterns-root" data-design-node-kind="block" data-design-node-label="页面模式预览"><header><h1>%s 页面模式</h1><p>快速草稿从仓库高信号页面中提炼的结构基线。</p></header><div class="grid">%s</div></main></body></html>`, name, name, patterns.String())
}

func buildProgrammaticTokensJSON(theme programmaticTheme) string {
	value := map[string]any{
		"color":      map[string]string{"primary": theme.primary, "background": theme.background, "surface": theme.surface, "text": theme.foreground, "mutedText": theme.mutedText, "border": theme.border},
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
