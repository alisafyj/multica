package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type DesignRepoAnalysisResponse struct {
	ID                string          `json:"id"`
	WorkspaceID       string          `json:"workspace_id"`
	ProjectID         string          `json:"project_id"`
	ProjectResourceID string          `json:"project_resource_id"`
	Status            string          `json:"status"`
	SchemaVersion     string          `json:"schema_version"`
	SourceFingerprint *string         `json:"source_fingerprint"`
	Framework         *string         `json:"framework"`
	Language          *string         `json:"language"`
	PackageManager    *string         `json:"package_manager"`
	AppType           *string         `json:"app_type"`
	Routing           json.RawMessage `json:"routing"`
	Styling           json.RawMessage `json:"styling"`
	Directories       json.RawMessage `json:"directories"`
	Commands          json.RawMessage `json:"commands"`
	Boundaries        json.RawMessage `json:"boundaries"`
	TargetCandidates  json.RawMessage `json:"target_candidates"`
	Confidence        float32         `json:"confidence"`
	Summary           *string         `json:"summary"`
	RawResult         json.RawMessage `json:"raw_result"`
	Error             *string         `json:"error"`
	AnalyzedAt        *string         `json:"analyzed_at"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

type CreateDesignRepoAnalysisRequest struct {
	ProjectID         string `json:"project_id"`
	ProjectResourceID string `json:"project_resource_id"`
}

type ListDesignRepoAnalysesResponse struct {
	Analyses []DesignRepoAnalysisResponse `json:"analyses"`
}

type repoPackageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type repoAnalysisResult struct {
	Framework        string
	Language         string
	PackageManager   string
	AppType          string
	Routing          map[string]any
	Styling          map[string]any
	Directories      map[string]any
	Commands         map[string]any
	Boundaries       map[string]any
	TargetCandidates []map[string]any
	Confidence       float32
	Summary          string
	RawResult        map[string]any
	Fingerprint      string
}

func designRepoAnalysisToResponse(a db.DesignRepoAnalysis) DesignRepoAnalysisResponse {
	return DesignRepoAnalysisResponse{
		ID:                uuidToString(a.ID),
		WorkspaceID:       uuidToString(a.WorkspaceID),
		ProjectID:         uuidToString(a.ProjectID),
		ProjectResourceID: uuidToString(a.ProjectResourceID),
		Status:            a.Status,
		SchemaVersion:     a.SchemaVersion,
		SourceFingerprint: textToPtr(a.SourceFingerprint),
		Framework:         textToPtr(a.Framework),
		Language:          textToPtr(a.Language),
		PackageManager:    textToPtr(a.PackageManager),
		AppType:           textToPtr(a.AppType),
		Routing:           json.RawMessage(a.Routing),
		Styling:           json.RawMessage(a.Styling),
		Directories:       json.RawMessage(a.Directories),
		Commands:          json.RawMessage(a.Commands),
		Boundaries:        json.RawMessage(a.Boundaries),
		TargetCandidates:  json.RawMessage(a.TargetCandidates),
		Confidence:        a.Confidence,
		Summary:           textToPtr(a.Summary),
		RawResult:         json.RawMessage(a.RawResult),
		Error:             textToPtr(a.Error),
		AnalyzedAt:        timestampToPtr(a.AnalyzedAt),
		CreatedAt:         timestampToString(a.CreatedAt),
		UpdatedAt:         timestampToString(a.UpdatedAt),
	}
}

func designRepoAnalysisPlanBlock(a db.DesignRepoAnalysis) map[string]any {
	return map[string]any{
		"status":         a.Status,
		"mode":           "production_candidate",
		"analysisId":     uuidToString(a.ID),
		"resourceId":     uuidToString(a.ProjectResourceID),
		"framework":      textToString(a.Framework),
		"language":       textToString(a.Language),
		"packageManager": textToString(a.PackageManager),
		"appType":        textToString(a.AppType),
		"routing":        jsonRawToAny(a.Routing, map[string]any{}),
		"styling":        jsonRawToAny(a.Styling, map[string]any{}),
		"directories":    jsonRawToAny(a.Directories, map[string]any{}),
		"confidence":     a.Confidence,
		"summary":        textToString(a.Summary),
	}
}

func textToString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func jsonRawToAny(raw []byte, fallback any) any {
	if len(raw) == 0 {
		return fallback
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fallback
	}
	return value
}

func jsonRawFieldToAny(raw []byte, field string, fallback any) any {
	var value map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return fallback
	}
	if result, ok := value[field]; ok {
		return result
	}
	return fallback
}

func (h *Handler) CreateDesignRepoAnalysis(w http.ResponseWriter, r *http.Request) {
	var req CreateDesignRepoAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	analysis, ok := h.createDesignRepoAnalysisFromRequest(w, r, req)
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, designRepoAnalysisToResponse(analysis))
}

func (h *Handler) ListDesignRepoAnalyses(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	projectUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(r.URL.Query().Get("project_id")), "project_id")
	if !ok {
		return
	}
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: projectUUID, WorkspaceID: wsUUID}); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	rows, err := h.Queries.ListDesignRepoAnalysesByProject(r.Context(), db.ListDesignRepoAnalysesByProjectParams{WorkspaceID: wsUUID, ProjectID: projectUUID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list design repo analyses")
		return
	}
	resp := make([]DesignRepoAnalysisResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, designRepoAnalysisToResponse(row))
	}
	writeJSON(w, http.StatusOK, ListDesignRepoAnalysesResponse{Analyses: resp})
}

func (h *Handler) GetDesignRepoAnalysis(w http.ResponseWriter, r *http.Request) {
	analysisID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "analysis id")
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	analysis, err := h.Queries.GetDesignRepoAnalysisInWorkspace(r.Context(), db.GetDesignRepoAnalysisInWorkspaceParams{ID: analysisID, WorkspaceID: wsUUID})
	if err != nil {
		writeError(w, http.StatusNotFound, "design repo analysis not found")
		return
	}
	writeJSON(w, http.StatusOK, designRepoAnalysisToResponse(analysis))
}

func (h *Handler) RerunDesignRepoAnalysis(w http.ResponseWriter, r *http.Request) {
	analysisID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "analysis id")
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	previous, err := h.Queries.GetDesignRepoAnalysisInWorkspace(r.Context(), db.GetDesignRepoAnalysisInWorkspaceParams{ID: analysisID, WorkspaceID: wsUUID})
	if err != nil {
		writeError(w, http.StatusNotFound, "design repo analysis not found")
		return
	}
	analysis, ok := h.createDesignRepoAnalysisForResource(w, r, previous.ProjectID, previous.ProjectResourceID)
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, designRepoAnalysisToResponse(analysis))
}

func (h *Handler) createDesignRepoAnalysisFromRequest(w http.ResponseWriter, r *http.Request, req CreateDesignRepoAnalysisRequest) (db.DesignRepoAnalysis, bool) {
	projectUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.ProjectID), "project_id")
	if !ok {
		return db.DesignRepoAnalysis{}, false
	}
	resourceUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.ProjectResourceID), "project_resource_id")
	if !ok {
		return db.DesignRepoAnalysis{}, false
	}
	return h.createDesignRepoAnalysisForResource(w, r, projectUUID, resourceUUID)
}

func (h *Handler) createDesignRepoAnalysisForResource(w http.ResponseWriter, r *http.Request, projectUUID, resourceUUID pgtype.UUID) (db.DesignRepoAnalysis, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return db.DesignRepoAnalysis{}, false
	}
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: projectUUID, WorkspaceID: wsUUID}); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return db.DesignRepoAnalysis{}, false
	}
	resource, err := h.Queries.GetProjectResourceInWorkspace(r.Context(), db.GetProjectResourceInWorkspaceParams{ID: resourceUUID, WorkspaceID: wsUUID})
	if err != nil || resource.ProjectID != projectUUID {
		writeError(w, http.StatusNotFound, "project resource not found")
		return db.DesignRepoAnalysis{}, false
	}
	if resource.ResourceType != "local_directory" {
		writeError(w, http.StatusBadRequest, "design repo analysis MVP supports local_directory resources only")
		return db.DesignRepoAnalysis{}, false
	}
	result, err := analyzeLocalDesignRepo(resource.ResourceRef)
	params := designRepoAnalysisCreateParams(wsUUID, projectUUID, resourceUUID, result, err)
	analysis, createErr := h.Queries.CreateDesignRepoAnalysis(r.Context(), params)
	if createErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to create design repo analysis")
		return db.DesignRepoAnalysis{}, false
	}
	return analysis, true
}

func (h *Handler) ensureDesignRepoAnalysisForProject(ctx context.Context, workspaceID, projectID pgtype.UUID) (db.DesignRepoAnalysis, error) {
	analysis, err := h.Queries.GetLatestCompletedDesignRepoAnalysisForProject(ctx, db.GetLatestCompletedDesignRepoAnalysisForProjectParams{WorkspaceID: workspaceID, ProjectID: projectID})
	if err == nil && designRepoAnalysisHasFileLevelTargets(analysis) {
		return analysis, nil
	}
	if err != nil && err != pgx.ErrNoRows {
		return db.DesignRepoAnalysis{}, err
	}
	resources, err := h.Queries.ListProjectResources(ctx, projectID)
	if err != nil {
		return db.DesignRepoAnalysis{}, err
	}
	var resource *db.ProjectResource
	for i := range resources {
		if resources[i].WorkspaceID == workspaceID && resources[i].ResourceType == "local_directory" {
			resource = &resources[i]
			break
		}
	}
	if resource == nil {
		return db.DesignRepoAnalysis{}, fmt.Errorf("no local_directory project resource available for design repo analysis")
	}
	result, analysisErr := analyzeLocalDesignRepo(resource.ResourceRef)
	created, createErr := h.Queries.CreateDesignRepoAnalysis(ctx, designRepoAnalysisCreateParams(workspaceID, projectID, resource.ID, result, analysisErr))
	if createErr != nil {
		return db.DesignRepoAnalysis{}, createErr
	}
	if analysisErr != nil {
		return created, analysisErr
	}
	return created, nil
}

func designRepoAnalysisHasFileLevelTargets(analysis db.DesignRepoAnalysis) bool {
	var candidates []map[string]any
	if err := json.Unmarshal(analysis.TargetCandidates, &candidates); err != nil {
		return false
	}
	for _, candidate := range candidates {
		kind, _ := candidate["kind"].(string)
		path, _ := candidate["path"].(string)
		if strings.HasSuffix(kind, "_file") && strings.Contains(path, ".") {
			return true
		}
	}
	return false
}

func designRepoAnalysisCreateParams(wsUUID, projectUUID, resourceUUID pgtype.UUID, result repoAnalysisResult, analysisErr error) db.CreateDesignRepoAnalysisParams {
	status := "completed"
	errorText := pgtype.Text{Valid: false}
	analyzedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	if analysisErr != nil {
		status = "failed"
		errorText = pgtype.Text{String: analysisErr.Error(), Valid: true}
		result = emptyRepoAnalysisResult()
	}
	return db.CreateDesignRepoAnalysisParams{
		WorkspaceID:       wsUUID,
		ProjectID:         projectUUID,
		ProjectResourceID: resourceUUID,
		Status:            status,
		SchemaVersion:     "1.0",
		SourceFingerprint: textValue(result.Fingerprint),
		Framework:         textValue(result.Framework),
		Language:          textValue(result.Language),
		PackageManager:    textValue(result.PackageManager),
		AppType:           textValue(result.AppType),
		Routing:           mustJSON(result.Routing, `{}`),
		Styling:           mustJSON(result.Styling, `{}`),
		Directories:       mustJSON(result.Directories, `{}`),
		Commands:          mustJSON(result.Commands, `{}`),
		Boundaries:        mustJSON(result.Boundaries, `{}`),
		TargetCandidates:  mustJSON(result.TargetCandidates, `[]`),
		Confidence:        result.Confidence,
		Summary:           textValue(result.Summary),
		RawResult:         mustJSON(result.RawResult, `{}`),
		Error:             errorText,
		AnalyzedAt:        analyzedAt,
	}
}

func emptyRepoAnalysisResult() repoAnalysisResult {
	return repoAnalysisResult{
		Routing:          map[string]any{},
		Styling:          map[string]any{},
		Directories:      map[string]any{},
		Commands:         map[string]any{},
		Boundaries:       map[string]any{},
		TargetCandidates: []map[string]any{},
		RawResult:        map[string]any{},
	}
}

func analyzeLocalDesignRepo(resourceRef []byte) (repoAnalysisResult, error) {
	var ref localDirectoryRef
	if err := json.Unmarshal(resourceRef, &ref); err != nil {
		return emptyRepoAnalysisResult(), fmt.Errorf("invalid local_directory resource_ref: %w", err)
	}
	root := strings.TrimSpace(ref.LocalPath)
	info, err := os.Stat(root)
	if err != nil {
		return emptyRepoAnalysisResult(), fmt.Errorf("local directory is not readable: %w", err)
	}
	if !info.IsDir() {
		return emptyRepoAnalysisResult(), fmt.Errorf("local path is not a directory")
	}
	pkg := readRepoPackageJSON(root)
	deps := mergeDeps(pkg)
	framework := detectRepoFramework(root, deps)
	language := detectRepoLanguage(root)
	packageManager := detectRepoPackageManager(root)
	appType := detectRepoAppType(root)
	routing := detectRepoRouting(root, framework)
	styling := detectRepoStyling(root, deps)
	directories := detectRepoDirectories(root)
	commands := detectRepoCommands(packageManager, pkg)
	boundaries := detectRepoBoundaries(root)
	targetCandidates := detectRepoTargetCandidates(root)
	confidence := repoAnalysisConfidence(framework, language, targetCandidates)
	summary := fmt.Sprintf("%s / %s / %s", fallback(framework, "unknown framework"), fallback(language, "unknown language"), fallback(packageManager, "unknown package manager"))
	return repoAnalysisResult{
		Framework:        framework,
		Language:         language,
		PackageManager:   packageManager,
		AppType:          appType,
		Routing:          routing,
		Styling:          styling,
		Directories:      directories,
		Commands:         commands,
		Boundaries:       boundaries,
		TargetCandidates: targetCandidates,
		Confidence:       confidence,
		Summary:          summary,
		Fingerprint:      repoFingerprint(root),
		RawResult: map[string]any{
			"root":                 root,
			"detectedDependencies": sortedStringKeys(deps),
			"analyzer":             "design_center_rule_mvp",
		},
	}, nil
}

func readRepoPackageJSON(root string) repoPackageJSON {
	var pkg repoPackageJSON
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return pkg
	}
	_ = json.Unmarshal(data, &pkg)
	return pkg
}

func mergeDeps(pkg repoPackageJSON) map[string]string {
	deps := map[string]string{}
	for k, v := range pkg.Dependencies {
		deps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		deps[k] = v
	}
	return deps
}

func detectRepoFramework(root string, deps map[string]string) string {
	switch {
	case deps["next"] != "" || anyPathExists(root, "next.config.js", "next.config.mjs", "next.config.ts"):
		return "Next.js"
	case deps["vue"] != "":
		return "Vue"
	case deps["react"] != "":
		return "React"
	case anyPathExists(root, "vite.config.js", "vite.config.ts", "vite.config.mjs"):
		return "Vite"
	default:
		return ""
	}
}

func detectRepoLanguage(root string) string {
	if anyPathExists(root, "tsconfig.json", "tsconfig.base.json") {
		return "TypeScript"
	}
	return "JavaScript"
}

func detectRepoPackageManager(root string) string {
	switch {
	case anyPathExists(root, "pnpm-lock.yaml"):
		return "pnpm"
	case anyPathExists(root, "yarn.lock"):
		return "yarn"
	case anyPathExists(root, "package-lock.json"):
		return "npm"
	default:
		return ""
	}
}

func detectRepoAppType(root string) string {
	if anyPathExists(root, "pnpm-workspace.yaml", "turbo.json", "packages") || anyPathExists(root, "apps") {
		return "monorepo"
	}
	return "single_app"
}

func detectRepoRouting(root, framework string) map[string]any {
	owners := existingPaths(root, "apps/web/app", "app", "src/app", "pages", "src/pages", "src/router", "src/routes")
	kind := "unknown"
	if framework == "Next.js" && len(existingPaths(root, "apps/web/app", "app", "src/app")) > 0 {
		kind = "next_app_router"
	} else if framework == "Next.js" {
		kind = "next"
	} else if len(existingPaths(root, "src/router", "src/routes")) > 0 {
		kind = "client_router"
	}
	return map[string]any{"kind": kind, "owners": owners}
}

func detectRepoStyling(root string, deps map[string]string) map[string]any {
	items := []string{}
	if deps["tailwindcss"] != "" || anyPathExists(root, "tailwind.config.js", "tailwind.config.ts") {
		items = append(items, "tailwind")
	}
	if anyPathExists(root, "components.json", "packages/ui/components.json") {
		items = append(items, "shadcn")
	}
	if deps["antd"] != "" {
		items = append(items, "antd")
	}
	if deps["@mui/material"] != "" {
		items = append(items, "mui")
	}
	return map[string]any{"systems": items}
}

func detectRepoDirectories(root string) map[string]any {
	return map[string]any{
		"appRoots":      existingPaths(root, "apps/web", "src", "app"),
		"businessViews": existingPaths(root, "packages/views", "src/views", "src/pages"),
		"uiComponents":  existingPaths(root, "packages/ui", "src/components", "components"),
		"coreLogic":     existingPaths(root, "packages/core", "src/store", "src/api"),
	}
}

func detectRepoCommands(packageManager string, pkg repoPackageJSON) map[string]any {
	pm := fallback(packageManager, "npm")
	typecheck := []string{}
	if _, ok := pkg.Scripts["typecheck"]; ok {
		typecheck = append(typecheck, pm+" run typecheck")
	} else {
		typecheck = append(typecheck, pm+" exec tsc --noEmit --pretty false")
	}
	return map[string]any{"typecheck": typecheck, "test": []string{}, "lint": []string{}}
}

func detectRepoBoundaries(root string) map[string]any {
	allowed := existingPaths(root, "packages/views", "packages/ui", "apps/web", "src", "components")
	forbidden := existingPaths(root, "server", "packages/core", "node_modules", ".next", "dist", "build")
	return map[string]any{"allowedPaths": allowed, "forbiddenPaths": forbidden}
}

func detectRepoTargetCandidates(root string) []map[string]any {
	targets := []struct {
		Path   string
		Kind   string
		Reason string
	}{
		{"packages/views", "business_view", "共享业务页面目录"},
		{"apps/web/app", "next_route", "Next.js App Router 页面入口"},
		{"src/pages", "page", "页面目录"},
		{"src/views", "view", "视图目录"},
		{"src/components", "component", "组件目录"},
	}
	out := []map[string]any{}
	seen := map[string]bool{}
	appendCandidate := func(path, kind, reason string, confidence float64) {
		if seen[path] {
			return
		}
		seen[path] = true
		out = append(out, map[string]any{"path": path, "kind": kind, "reason": reason, "confidence": confidence})
	}
	for _, item := range targets {
		if pathExists(root, item.Path) {
			appendCandidate(item.Path, item.Kind, item.Reason, 0.7)
		}
	}
	for _, item := range detectRepoTargetFiles(root) {
		appendCandidate(item["path"].(string), item["kind"].(string), item["reason"].(string), item["confidence"].(float64))
	}
	return out
}

func detectRepoTargetFiles(root string) []map[string]any {
	specs := []struct {
		Root   string
		Kind   string
		Reason string
	}{
		{"packages/views", "business_view_file", "可直接还原到共享业务视图文件"},
		{"apps/web/app", "next_route_file", "可直接还原到 Next.js 路由页面"},
		{"src/pages", "page_file", "可直接还原到页面文件"},
		{"src/views", "view_file", "可直接还原到视图文件"},
		{"src/components", "component_file", "可直接还原到组件文件"},
	}
	out := []map[string]any{}
	for _, spec := range specs {
		base := filepath.Join(root, spec.Root)
		if !pathExists(root, spec.Root) {
			continue
		}
		_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if !(strings.HasSuffix(name, ".tsx") || strings.HasSuffix(name, ".jsx") || strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")) {
				return nil
			}
			if strings.HasSuffix(name, ".test.ts") || strings.HasSuffix(name, ".test.tsx") || strings.HasSuffix(name, ".spec.ts") || strings.HasSuffix(name, ".spec.tsx") {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			out = append(out, map[string]any{"path": rel, "kind": spec.Kind, "reason": spec.Reason, "confidence": 0.82})
			if len(out) >= 30 {
				return filepath.SkipAll
			}
			return nil
		})
	}
	return out
}

func repoAnalysisConfidence(framework, language string, candidates []map[string]any) float32 {
	confidence := float32(0.35)
	if framework != "" {
		confidence += 0.25
	}
	if language != "" {
		confidence += 0.15
	}
	if len(candidates) > 0 {
		confidence += 0.2
	}
	if confidence > 0.95 {
		return 0.95
	}
	return confidence
}

func existingPaths(root string, paths ...string) []string {
	out := []string{}
	for _, path := range paths {
		if pathExists(root, path) {
			out = append(out, path)
		}
	}
	return out
}

func anyPathExists(root string, paths ...string) bool {
	for _, path := range paths {
		if pathExists(root, path) {
			return true
		}
	}
	return false
}

func pathExists(root, path string) bool {
	_, err := os.Stat(filepath.Join(root, path))
	return err == nil
}

func repoFingerprint(root string) string {
	parts := []string{}
	for _, path := range []string{"package.json", "pnpm-lock.yaml", "yarn.lock", "package-lock.json", "turbo.json", "next.config.js", "vite.config.ts"} {
		if info, err := os.Stat(filepath.Join(root, path)); err == nil {
			parts = append(parts, fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().Unix()))
		}
	}
	return strings.Join(parts, "|")
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}

func textValue(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	if value == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: value, Valid: true}
}

func mustJSON(value any, fallback string) []byte {
	out, err := json.Marshal(value)
	if err != nil || len(out) == 0 {
		return []byte(fallback)
	}
	return out
}
