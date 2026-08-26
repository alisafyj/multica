package designsystemcatalogue

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// The kit-view content of a bundled package, parsed the way Open Design reads
// its own packages (daemon extractSwatches/pickSwatchRow, web parseDesignMd):
// named colours scraped from DESIGN.md drive both the list row's stripe tile
// and the 调色板 cards, the Typography section drives the family cards, the
// first spacing/layout section drives 布局准则, system/tokens.default.json
// drives the contract chips, and system/artifacts drives the asset grid.
//
// One deliberate divergence: our bundle prefers the zh DESIGN.md where Open
// Design ships one, so the scrapers accept bold CJK labels and fullwidth
// punctuation that upstream's ASCII-only patterns would miss.

// PaletteEntry is one named colour of the 调色板 module: the label the
// document gave it, the role chip Open Design infers from that label, the hex,
// and the usage text after it.
type PaletteEntry struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Value string `json:"value"`
	Usage string `json:"usage"`
}

// Typography is the package's three declared families plus its weight scale.
type Typography struct {
	Display string   `json:"display"`
	Body    string   `json:"body"`
	Mono    string   `json:"mono"`
	Weights []string `json:"weights"`
}

// TokenContractEntry is one entry of system/tokens.default.json, in file
// order — the order is the display order of the kit view's contract chips.
type TokenContractEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Artifact is one derived page under system/artifacts.
type Artifact struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// artifactPages is Open Design's ASSET_TILES: the display order and captions
// of the 设计系统素材 grid. A package missing a page simply has no card.
var artifactPages = []Artifact{
	{ID: "landing", Label: "Landing page"},
	{ID: "deck", Label: "Pitch deck"},
	{ID: "poster", Label: "Poster"},
	{ID: "email", Label: "Email"},
	{ID: "newsletter", Label: "Newsletter"},
	{ID: "form", Label: "Form page"},
}

// namedColor is one scraped label/hex pair.
type namedColor struct {
	name  string
	value string
	usage string
}

var (
	hexPattern       = regexp.MustCompile(`#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{8}\b|#[0-9a-fA-F]{3,4}\b`)
	boldLabelPattern = regexp.MustCompile(`\*\*(.+?)\*\*`)
	asciiLabelForm   = regexp.MustCompile(`^[\s>*-]*([A-Za-z][A-Za-z0-9 /&()+_-]{0,40}?)\s*[:：]`)
	headingNumbering = regexp.MustCompile(`^\d+[.)]\s*`)
	weightPattern    = regexp.MustCompile(`\b\d{3}\b`)
)

// designMarkdownDocument is DESIGN.md split into its parts, sections kept in
// document order because Open Design's findSection returns the first match.
type designMarkdownDocument struct {
	Title       string
	Preamble    []string
	Sections    []designMarkdownSection
	Frontmatter []string
}

type designMarkdownSection struct {
	Heading string
	Lines   []string
}

func parseDesignMarkdown(markdown string) designMarkdownDocument {
	document := designMarkdownDocument{}
	lines := strings.Split(markdown, "\n")
	start := 0
	// YAML front matter: a leading --- pair. Kept raw for the colours map.
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				document.Frontmatter = lines[1:i]
				start = i + 1
				break
			}
		}
	}
	inSection := false
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			heading := headingNumbering.ReplaceAllString(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), "")
			document.Sections = append(document.Sections, designMarkdownSection{Heading: strings.ToLower(heading)})
			inSection = true
			continue
		}
		if !inSection {
			if strings.HasPrefix(trimmed, "# ") && document.Title == "" {
				document.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
				continue
			}
			document.Preamble = append(document.Preamble, trimmed)
			continue
		}
		last := &document.Sections[len(document.Sections)-1]
		last.Lines = append(last.Lines, trimmed)
	}
	return document
}

// section returns the FIRST section, in document order, whose heading contains
// any of the keywords — Open Design's findSection semantics.
func (d designMarkdownDocument) section(keywords ...string) []string {
	for _, section := range d.Sections {
		for _, keyword := range keywords {
			if strings.Contains(section.Heading, keyword) {
				return section.Lines
			}
		}
	}
	return nil
}

// identityText is the positioning line 品牌标识 shows: the preamble blockquote
// minus the Category/Surface lines the header already carries.
func (d designMarkdownDocument) identityText() string {
	parts := make([]string, 0, len(d.Preamble))
	for _, line := range d.Preamble {
		line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		if line == "" || strings.HasPrefix(line, "Category:") || strings.HasPrefix(line, "Surface:") {
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, " ")
}

// scrapeNamedColors pulls every labelled hex out of the given lines: bullet
// and prose labels (bold or ASCII "Name:"), and Markdown table rows. Duplicate
// name|hex pairs keep their first occurrence.
func scrapeNamedColors(lines []string) []namedColor {
	seen := map[string]struct{}{}
	colors := []namedColor{}
	appendColor := func(name, value, usage string) {
		name = strings.Trim(strings.TrimSpace(name), "`*")
		name = strings.TrimSpace(strings.TrimRight(name, ":："))
		value = strings.ToLower(value)
		key := strings.ToLower(name) + "|" + value
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		colors = append(colors, namedColor{name: name, value: value, usage: usage})
	}
	for _, line := range lines {
		if line == "" || !strings.Contains(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "|") {
			cells := strings.Split(strings.Trim(line, "|"), "|")
			if len(cells) < 2 {
				continue
			}
			name := strings.TrimSpace(cells[0])
			if name == "" || strings.HasPrefix(name, "---") || strings.Trim(name, "-: ") == "" {
				continue
			}
			if hex := hexPattern.FindString(line); hex != "" {
				usage := ""
				if len(cells) >= 3 {
					usage = strings.TrimSpace(cells[len(cells)-1])
				}
				appendColor(name, hex, usage)
			}
			continue
		}
		hexIndex := hexPattern.FindStringIndex(line)
		if hexIndex == nil {
			continue
		}
		hex := line[hexIndex[0]:hexIndex[1]]
		before := line[:hexIndex[0]]
		name := ""
		if match := boldLabelPattern.FindStringSubmatch(before); match != nil {
			name = match[1]
		} else if match := asciiLabelForm.FindStringSubmatch(before); match != nil {
			name = match[1]
		}
		if strings.TrimSpace(name) == "" {
			continue
		}
		usage := strings.TrimSpace(line[hexIndex[1]:])
		usage = strings.TrimLeft(usage, "`)）]。 \t")
		usage = strings.TrimLeft(usage, "—–-:：( （")
		usage = strings.TrimSuffix(strings.TrimSpace(usage), ")")
		appendColor(name, hex, usage)
	}
	return colors
}

// frontmatterColors reads a YAML front matter `colors:` map of "key: #hex"
// entries, as Open Design's swatchesFromFrontmatter does.
func frontmatterColors(frontmatter []string) []namedColor {
	colors := []namedColor{}
	inColors := false
	for _, line := range frontmatter {
		if !strings.HasPrefix(line, " ") {
			inColors = strings.HasPrefix(strings.TrimSpace(line), "colors:")
			continue
		}
		if !inColors {
			continue
		}
		key, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		if hex := hexPattern.FindString(value); hex != "" {
			colors = append(colors, namedColor{name: strings.TrimSpace(key), value: strings.ToLower(hex)})
		}
	}
	return colors
}

// isNeutralColor is Open Design's neutrality test: the RGB channel spread is
// under 10, i.e. the colour is a grey.
func isNeutralColor(value string) bool {
	hex := strings.TrimPrefix(strings.ToLower(value), "#")
	if len(hex) == 3 || len(hex) == 4 {
		expanded := ""
		for _, r := range hex {
			expanded += string(r) + string(r)
		}
		hex = expanded
	}
	if len(hex) < 6 {
		return false
	}
	var r, g, b int
	if _, err := fmt.Sscanf(hex[:6], "%02x%02x%02x", &r, &g, &b); err != nil {
		return false
	}
	max := r
	min := r
	for _, channel := range []int{g, b} {
		if channel > max {
			max = channel
		}
		if channel < min {
			min = channel
		}
	}
	return max-min < 10
}

// Slot hints, in Open Design's priority order, with zh additions for the
// packages our bundle keeps in Chinese.
var (
	swatchBgHints      = []string{"page background", "background", "canvas", "paper", "surface", "背景", "画布", "表面"}
	swatchFgHints      = []string{"heading", "foreground", "ink", "fg", "text", "navy", "graphite", "文字", "文本", "墨", "标题"}
	swatchAccentHints  = []string{"primary brand", "brand primary", "accent", "brand", "primary", "主色", "强调", "品牌"}
	swatchSupportHints = []string{"border", "divider", "rule", "muted", "secondary", "subtle", "边框", "分隔", "辅助", "次要"}
)

type swatchRow struct {
	values    []string
	filledAll bool
}

// pickSwatchRow reduces the scraped colours to Open Design's four stripe
// slots: background, support, foreground, accent.
func pickSwatchRow(colors []namedColor) swatchRow {
	pick := func(hints []string) string {
		for _, hint := range hints {
			for _, color := range colors {
				if strings.Contains(strings.ToLower(color.name), hint) {
					return color.value
				}
			}
		}
		return ""
	}
	bg := pick(swatchBgHints)
	fg := pick(swatchFgHints)
	accent := pick(swatchAccentHints)
	support := pick(swatchSupportHints)
	filledAll := bg != "" && fg != "" && accent != "" && support != ""
	if bg == "" {
		bg = "#ffffff"
	}
	if fg == "" {
		fg = "#111111"
	}
	if accent == "" {
		for _, color := range colors {
			if !isNeutralColor(color.value) {
				accent = color.value
				break
			}
		}
	}
	if accent == "" {
		if len(colors) > 0 {
			accent = colors[0].value
		} else {
			accent = "#888888"
		}
	}
	if support == "" {
		for _, color := range colors {
			if isNeutralColor(color.value) && color.value != bg && color.value != fg {
				support = color.value
				break
			}
		}
	}
	if support == "" {
		support = "#cccccc"
	}
	return swatchRow{values: []string{bg, support, fg, accent}, filledAll: filledAll}
}

// swatchesFromDesignMarkdown is Open Design's list-row palette: the front
// matter row when it fills every slot, else the row picked from the whole
// document's named colours, else whatever partial front matter row exists.
// Empty when the document names no colours at all — the view then falls back
// to its seeded stripes.
func swatchesFromDesignMarkdown(document designMarkdownDocument) []string {
	var body []string
	body = append(body, document.Preamble...)
	for _, section := range document.Sections {
		body = append(body, section.Lines...)
	}
	scraped := scrapeNamedColors(body)
	front := frontmatterColors(document.Frontmatter)
	if len(front) > 0 {
		frontRow := pickSwatchRow(front)
		if frontRow.filledAll {
			return frontRow.values
		}
		if len(scraped) == 0 {
			return frontRow.values
		}
	}
	if len(scraped) == 0 {
		return []string{}
	}
	return pickSwatchRow(scraped).values
}

// inferPaletteRole is Open Design's inferRole: ten keyword tests in priority
// order, then the cleaned lowercase label itself — the role line under a name
// like "Parchment" simply reads "parchment".
func inferPaletteRole(label string) string {
	lowered := strings.ToLower(label)
	for _, rule := range []struct {
		pattern *regexp.Regexp
		role    string
	}{
		{regexp.MustCompile(`background|canvas|page|paper`), "background"},
		{regexp.MustCompile(`surface|card|panel|elevated`), "surface"},
		{regexp.MustCompile(`foreground|text|ink|body|heading|content|on-`), "foreground"},
		{regexp.MustCompile(`muted|secondary text|metadata|caption|subtle|slate`), "muted"},
		{regexp.MustCompile(`border|hairline|divider|line|outline|stroke`), "border"},
		{regexp.MustCompile(`accent secondary|secondary|tertiary`), "accent-secondary"},
		{regexp.MustCompile(`accent|primary|brand|cta|link|highlight`), "accent"},
		{regexp.MustCompile(`success|ok|positive|green`), "success"},
		{regexp.MustCompile(`warn|amber|caution`), "warning"},
		{regexp.MustCompile(`danger|error|destructive|red|negative`), "danger"},
	} {
		if rule.pattern.MatchString(lowered) {
			return rule.role
		}
	}
	fallback := strings.TrimSpace(asciiRolePattern.ReplaceAllString(lowered, ""))
	if fallback != "" {
		return fallback
	}
	// A label with no ASCII at all (our zh packages) keeps itself as the
	// role line, the way Open Design's fallback keeps the label.
	if trimmed := strings.TrimSpace(lowered); trimmed != "" {
		return trimmed
	}
	return "color"
}

var asciiRolePattern = regexp.MustCompile(`[^a-z0-9 ]`)

// titleCaseLabel is Open Design's titleCase: dashes and underscores become
// spaces and each ASCII word is capitalised; CJK passes through unchanged.
func titleCaseLabel(value string) string {
	value = strings.NewReplacer("-", " ", "_", " ").Replace(value)
	words := strings.Fields(value)
	for i, word := range words {
		runes := []rune(word)
		if runes[0] >= 'a' && runes[0] <= 'z' {
			runes[0] = runes[0] - 'a' + 'A'
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

var trailingParens = regexp.MustCompile(`\s*[(（].*[)）]\s*$`)

// parsePalette builds the 调色板 cards from the colour section (front matter
// as fallback), following Open Design's parseColors: table rows map
// role|name|hex|usage, bullets take the bold label, the role is inferred from
// it, everything after the hex is the usage note; de-duplicated by hex with
// the first occurrence winning and capped at 12.
func parsePalette(document designMarkdownDocument) []PaletteEntry {
	lines := document.section("color", "palette", "色彩", "颜色", "调色")
	parsed := []PaletteEntry{}
	for _, line := range lines {
		if !strings.Contains(line, "#") {
			continue
		}
		hexIndex := hexPattern.FindStringIndex(line)
		if hexIndex == nil {
			continue
		}
		hex := strings.ToLower(line[hexIndex[0]:hexIndex[1]])
		if strings.HasPrefix(line, "|") {
			cells := []string{}
			for _, cell := range strings.Split(strings.Trim(line, "|"), "|") {
				if cell = strings.TrimSpace(cell); cell != "" {
					cells = append(cells, cell)
				}
			}
			if len(cells) == 0 || allSeparatorCells(cells) || isTableHeader(cells) {
				continue
			}
			role := cleanLabel(cells[0])
			name := ""
			if len(cells) > 1 {
				name = cleanLabel(cells[1])
			}
			if name == "" {
				name = titleCaseLabel(role)
			}
			usage := ""
			if len(cells) > 3 {
				usage = cleanLabel(cells[3])
			} else if len(cells) > 2 {
				usage = cleanLabel(cells[2])
			}
			if usage == "—" {
				usage = ""
			}
			if role == "" {
				role = inferPaletteRole(name)
			}
			parsed = append(parsed, PaletteEntry{Name: name, Role: role, Value: strings.ToUpper(hex), Usage: usage})
			continue
		}
		label := ""
		if match := boldLabelPattern.FindStringSubmatch(line); match != nil {
			label = cleanLabel(match[1])
		} else if match := asciiLabelForm.FindStringSubmatch(line[:hexIndex[0]]); match != nil {
			label = cleanLabel(match[1])
		}
		usage := strings.TrimSpace(line[hexIndex[1]:])
		usage = strings.TrimLeft(usage, " \t—–-:：(（`)）")
		usage = strings.TrimSuffix(strings.TrimSpace(usage), ")")
		name := titleCaseLabel(trailingParens.ReplaceAllString(label, ""))
		role := inferPaletteRole(label)
		if name == "" {
			name = titleCaseLabel(role)
		}
		parsed = append(parsed, PaletteEntry{Name: name, Role: role, Value: strings.ToUpper(hex), Usage: usage})
	}
	if len(parsed) == 0 {
		for _, color := range frontmatterColors(document.Frontmatter) {
			parsed = append(parsed, PaletteEntry{
				Name:  titleCaseLabel(color.name),
				Role:  color.name,
				Value: strings.ToUpper(color.value),
				Usage: "",
			})
		}
	}
	seen := map[string]struct{}{}
	entries := []PaletteEntry{}
	for _, entry := range parsed {
		key := strings.ToLower(entry.Value)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, entry)
		if len(entries) == 12 {
			break
		}
	}
	return entries
}

func cleanLabel(value string) string {
	value = strings.NewReplacer("*", "", "`", "").Replace(value)
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(value), ":："))
}

func allSeparatorCells(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

func isTableHeader(cells []string) bool {
	hasRole := false
	hasHex := false
	for _, cell := range cells {
		lowered := strings.ToLower(strings.TrimSpace(cell))
		if lowered == "role" {
			hasRole = true
		}
		if lowered == "hex" {
			hasHex = true
		}
	}
	return hasRole && hasHex
}

// parseTypography reads the Families line// parseTypography is Open Design's: a shared weights line first, then the
// preset "Families: primary=X, display=Y, mono=Z" line with its alias chain,
// then per-role bold bullets whose label names a role — the family being
// whatever the bullet says up to the first dash, parenthesis or backtick,
// quirks included, because the reference renders exactly that.
func parseTypography(typographySection []string) Typography {
	typography := Typography{Weights: []string{}}
	for _, line := range typographySection {
		if weightMention.MatchString(line) && weightPattern.MatchString(line) {
			typography.Weights = parseWeightScale(line)
			break
		}
	}
	for _, line := range typographySection {
		if !familiesMention.MatchString(line) || !strings.Contains(line, "=") {
			continue
		}
		families := map[string]string{}
		for _, match := range familyPairPattern.FindAllStringSubmatch(line, -1) {
			families[strings.ToLower(match[1])] = cleanLabel(match[2])
		}
		first := func(keys ...string) string {
			for _, key := range keys {
				if value := families[key]; value != "" {
					return value
				}
			}
			return ""
		}
		typography.Display = first("display", "heading", "primary")
		typography.Body = first("body", "text", "primary", "display")
		typography.Mono = first("mono", "code")
		break
	}
	for _, line := range typographySection {
		match := roleBulletPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		label := strings.ToLower(match[1])
		var slot *string
		switch {
		case strings.Contains(label, "mono") || strings.Contains(label, "code"):
			slot = &typography.Mono
		case strings.Contains(label, "body") || strings.Contains(label, "text"):
			slot = &typography.Body
		case strings.Contains(label, "display") || strings.Contains(label, "head") || strings.Contains(label, "title"):
			slot = &typography.Display
		default:
			continue
		}
		if *slot != "" {
			continue
		}
		family := familyFromLine(line)
		if family == "" {
			continue
		}
		*slot = family
	}
	return typography
}

var (
	weightMention     = regexp.MustCompile(`(?i)weights?`)
	familiesMention   = regexp.MustCompile(`(?i)families?`)
	familyPairPattern = regexp.MustCompile(`(\w+)\s*=\s*([^,]+)`)
	roleBulletPattern = regexp.MustCompile(`^[-*]\s*\*\*(.+?)\*\*`)
	quotedFamily      = regexp.MustCompile(`["'“”]([^"'“”]+)["'“”]`)
	familyStops       = regexp.MustCompile("—|–| - | or |\\(|`")
	afterRoleLabel    = regexp.MustCompile(`^[-*]\s*\*\*.+?\*\*[:：]?\s*`)
)

// parseWeightScale keeps the line's three-digit numbers, unique, 100–900.
func parseWeightScale(line string) []string {
	weights := []string{}
	seen := map[string]struct{}{}
	for _, weight := range weightPattern.FindAllString(line, -1) {
		if weight < "100" || weight > "900" {
			continue
		}
		if _, dup := seen[weight]; dup {
			continue
		}
		seen[weight] = struct{}{}
		weights = append(weights, weight)
	}
	return weights
}

// familyFromLine is Open Design's: a quoted family wins; otherwise the text
// after the bold label up to the first dash, " - ", " or ", parenthesis or
// backtick.
func familyFromLine(line string) string {
	if match := quotedFamily.FindStringSubmatch(line); match != nil {
		return cleanLabel(match[1])
	}
	afterLabel := afterRoleLabel.ReplaceAllString(line, "")
	if loc := familyStops.FindStringIndex(afterLabel); loc != nil {
		afterLabel = afterLabel[:loc[0]]
	}
	return cleanLabel(afterLabel)
}

// parseLayoutGuidelines flattens the first spacing/layout section's bullets// parseLayoutGuidelines flattens the first spacing/layout section's bullets
// into plain sentences, at most six — the kit view's 布局准则 list.
func parseLayoutGuidelines(layoutSection []string) []string {
	guidelines := []string{}
	for _, line := range layoutSection {
		if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
			continue
		}
		text := strings.TrimSpace(line[2:])
		if match := boldBulletPattern.FindStringSubmatch(text); match != nil {
			text = match[1] + ": " + strings.TrimSpace(boldBulletPattern.ReplaceAllString(text, ""))
		}
		text = strings.ReplaceAll(text, "**", "")
		if text == "" || strings.HasPrefix(strings.ToLower(text), "(none") {
			continue
		}
		guidelines = append(guidelines, text)
		if len(guidelines) == 6 {
			break
		}
	}
	return guidelines
}

var boldBulletPattern = regexp.MustCompile(`^\*\*(.+?)[:：]?\*\*[:：]?\s*`)

// parseTokenContract reads system/tokens.default.json preserving the file's
// key order, which Go's map decoding would lose.
func parseTokenContract(raw []byte) ([]TokenContractEntry, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("token contract is not an object")
	}
	entries := []TokenContractEntry{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("token contract key is not a string")
		}
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		entries = append(entries, TokenContractEntry{Name: key, Value: fmt.Sprintf("%v", value)})
	}
	return entries, nil
}
