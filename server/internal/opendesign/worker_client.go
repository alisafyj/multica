package opendesign

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxWorkerResponseBytes int64 = 2 << 20
	maxWorkerEventBytes          = 1 << 20
	maxWorkerArchiveBytes        = RunArchiveMaxBytes
)

type WorkerWorkspaceProvenance struct {
	SourceLabel  string `json:"sourceLabel,omitempty"`
	SourceRef    string `json:"sourceRef,omitempty"`
	BaseRevision string `json:"baseRevision,omitempty"`
}

type WorkerWorkspaceRequest struct {
	ScratchRoot string
	Name        string
	Provenance  WorkerWorkspaceProvenance
}

type WorkerWorkspace struct {
	ProjectID      string
	ConversationID string
}

type WorkerStartRunRequest struct {
	Workspace WorkerWorkspace
	Agent     AgentIdentity
	Prompt    string
}

type WorkerRunStatus struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ExitCode        *int   `json:"exitCode,omitempty"`
	Signal          string `json:"signal,omitempty"`
	Error           string `json:"error,omitempty"`
	ErrorCode       string `json:"errorCode,omitempty"`
	FailureCategory string `json:"failureCategory,omitempty"`
	FailureDetail   string `json:"failureDetail,omitempty"`
	CancelRequested bool   `json:"cancelRequested,omitempty"`
	Resumable       bool   `json:"resumable,omitempty"`
	UnfinishedWork  bool   `json:"endedWithUnfinishedWork,omitempty"`
}

type workerPackageAudit struct {
	PackageAudit
	ProjectPath string `json:"projectPath"`
}

type workerPreviewURLResponse struct {
	URL           string `json:"url"`
	File          string `json:"file"`
	CSP           string `json:"csp"`
	IframeSandbox string `json:"iframeSandbox"`
	OpaqueOrigin  bool   `json:"opaqueOrigin"`
}

type workerImportRequest struct {
	BaseDir               string                      `json:"baseDir"`
	Name                  string                      `json:"name"`
	OrchestratorWorkspace workerOrchestratorWorkspace `json:"orchestratorWorkspace"`
}

type workerOrchestratorWorkspace struct {
	Kind         string `json:"kind"`
	Writeback    string `json:"writeback"`
	SourceLabel  string `json:"sourceLabel,omitempty"`
	SourceRef    string `json:"sourceRef,omitempty"`
	BaseRevision string `json:"baseRevision,omitempty"`
}

type workerRunRequest struct {
	ProjectID      string `json:"projectId"`
	ConversationID string `json:"conversationId"`
	AgentID        string `json:"agentId"`
	Model          string `json:"model,omitempty"`
	Message        string `json:"message"`
	CurrentPrompt  string `json:"currentPrompt"`
	SessionMode    string `json:"sessionMode"`
}

type WorkerClient struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

type workerHTTPStatusError struct {
	statusCode int
	code       string
	message    string
}

func (e *workerHTTPStatusError) Error() string {
	detail := strings.TrimSpace(e.message)
	if e.code != "" {
		if detail != "" {
			detail = e.code + ": " + detail
		} else {
			detail = e.code
		}
	}
	if detail == "" {
		detail = http.StatusText(e.statusCode)
	}
	return fmt.Sprintf("Open Design worker returned HTTP %d: %s", e.statusCode, detail)
}

func (e *workerHTTPStatusError) HTTPStatusCode() int {
	return e.statusCode
}

func NewWorkerClient(rawBaseURL, token string, httpClient *http.Client) (*WorkerClient, error) {
	baseURL, err := parseWorkerBaseURL(rawBaseURL)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		return nil, errors.New("Open Design worker HTTP client is required")
	}
	return &WorkerClient{
		baseURL:    baseURL,
		token:      strings.TrimSpace(token),
		httpClient: httpClient,
	}, nil
}

func parseWorkerBaseURL(rawBaseURL string) (*url.URL, error) {
	baseURL, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return nil, fmt.Errorf("parse Open Design worker URL: %w", err)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, errors.New("Open Design worker URL must use http or https")
	}
	if baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("Open Design worker URL must not contain credentials, query, or fragment")
	}
	hostname := baseURL.Hostname()
	address := net.ParseIP(hostname)
	if hostname != "localhost" && (address == nil || !address.IsLoopback()) {
		return nil, errors.New("Open Design worker URL must use a loopback host")
	}
	if strings.Trim(baseURL.Path, "/") != "" {
		return nil, errors.New("Open Design worker URL must not contain a path")
	}
	baseURL.Path = ""
	return baseURL, nil
}

func (c *WorkerClient) PrepareWorkspace(ctx context.Context, request WorkerWorkspaceRequest) (WorkerWorkspace, error) {
	scratchRoot, err := filepath.Abs(strings.TrimSpace(request.ScratchRoot))
	if err != nil {
		return WorkerWorkspace{}, fmt.Errorf("resolve Open Design scratch root: %w", err)
	}
	info, err := os.Lstat(scratchRoot)
	if err != nil {
		return WorkerWorkspace{}, fmt.Errorf("inspect Open Design scratch root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return WorkerWorkspace{}, errors.New("Open Design scratch root must be a real directory")
	}

	var imported struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		ConversationID string `json:"conversationId"`
	}
	err = c.doJSON(ctx, http.MethodPost, "/api/import/folder", workerImportRequest{
		BaseDir: scratchRoot,
		Name:    strings.TrimSpace(request.Name),
		OrchestratorWorkspace: workerOrchestratorWorkspace{
			Kind:         "scratch",
			Writeback:    "external",
			SourceLabel:  strings.TrimSpace(request.Provenance.SourceLabel),
			SourceRef:    strings.TrimSpace(request.Provenance.SourceRef),
			BaseRevision: strings.TrimSpace(request.Provenance.BaseRevision),
		},
	}, &imported)
	if err != nil {
		return WorkerWorkspace{}, fmt.Errorf("import Open Design scratch workspace: %w", err)
	}
	if strings.TrimSpace(imported.Project.ID) == "" || strings.TrimSpace(imported.ConversationID) == "" {
		return WorkerWorkspace{}, errors.New("Open Design worker returned an incomplete imported workspace")
	}

	// Folder import assigns kind=prototype, which silently activates the
	// example-web-prototype scenario. Phase 0 runs intentionally have no
	// scenario plugin; clearing mutable metadata leaves the worker-preserved
	// folder and orchestrator provenance fields intact while disabling fallback.
	projectPath := "/api/projects/" + url.PathEscape(imported.Project.ID)
	if err := c.doJSON(ctx, http.MethodPatch, projectPath, map[string]any{
		"metadata": map[string]any{},
	}, nil); err != nil {
		return WorkerWorkspace{}, fmt.Errorf("disable Open Design scenario fallback: %w", err)
	}
	return WorkerWorkspace{
		ProjectID:      imported.Project.ID,
		ConversationID: imported.ConversationID,
	}, nil
}

func (c *WorkerClient) StartRun(ctx context.Context, request WorkerStartRunRequest) (string, error) {
	if strings.TrimSpace(request.Workspace.ProjectID) == "" || strings.TrimSpace(request.Workspace.ConversationID) == "" {
		return "", errors.New("Open Design worker workspace is incomplete")
	}
	if strings.TrimSpace(request.Agent.AdapterID) == "" {
		return "", errors.New("Open Design worker adapter_id is required")
	}
	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" {
		return "", errors.New("Open Design worker prompt is required")
	}
	var response struct {
		RunID string `json:"runId"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/api/runs", workerRunRequest{
		ProjectID:      request.Workspace.ProjectID,
		ConversationID: request.Workspace.ConversationID,
		AgentID:        request.Agent.AdapterID,
		Model:          request.Agent.Model,
		Message:        prompt,
		CurrentPrompt:  prompt,
		SessionMode:    "design",
	}, &response)
	if err != nil {
		return "", fmt.Errorf("start Open Design worker run: %w", err)
	}
	if strings.TrimSpace(response.RunID) == "" {
		return "", errors.New("Open Design worker returned an empty run id")
	}
	return response.RunID, nil
}

func (c *WorkerClient) StreamRunEvents(ctx context.Context, runID string, after int64, consume func(RunEvent) error) error {
	if consume == nil {
		return errors.New("Open Design run event consumer is required")
	}
	endpoint := c.endpoint("/api/runs/" + url.PathEscape(runID) + "/events")
	query := endpoint.Query()
	if after > 0 {
		query.Set("after", strconv.FormatInt(after, 10))
		endpoint.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	if after > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatInt(after, 10))
	}
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream Open Design run events: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return workerHTTPError(resp)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), maxWorkerEventBytes)
	var eventID, eventName string
	dataLines := make([]string, 0, 1)
	dispatch := func() error {
		if eventID == "" && eventName == "" && len(dataLines) == 0 {
			return nil
		}
		id, err := strconv.ParseInt(strings.TrimSpace(eventID), 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("Open Design run event has invalid id %q", eventID)
		}
		if strings.TrimSpace(eventName) == "" {
			return fmt.Errorf("Open Design run event %d has no event name", id)
		}
		data := json.RawMessage(strings.Join(dataLines, "\n"))
		if len(data) == 0 || !json.Valid(data) {
			return fmt.Errorf("Open Design run event %d has invalid JSON data", id)
		}
		return consume(RunEvent{ID: id, Event: eventName, Data: data})
	}
	reset := func() {
		eventID = ""
		eventName = ""
		dataLines = dataLines[:0]
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			reset()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			eventID = value
		case "event":
			eventName = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Open Design run events: %w", err)
	}
	return dispatch()
}

func (c *WorkerClient) GetRun(ctx context.Context, runID string) (WorkerRunStatus, error) {
	var status WorkerRunStatus
	if err := c.doJSON(ctx, http.MethodGet, "/api/runs/"+url.PathEscape(runID), nil, &status); err != nil {
		return WorkerRunStatus{}, fmt.Errorf("get Open Design worker run: %w", err)
	}
	return status, nil
}

func (c *WorkerClient) GetResultPackage(ctx context.Context, runID string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/runs/"+url.PathEscape(runID)+"/result-package", nil, &result); err != nil {
		return nil, fmt.Errorf("get Open Design result package: %w", err)
	}
	return result, nil
}

func (c *WorkerClient) GetProjectExportManifest(ctx context.Context, projectID string) (json.RawMessage, error) {
	var manifest json.RawMessage
	path := "/api/projects/" + url.PathEscape(projectID) + "/export/manifest"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &manifest); err != nil {
		return nil, fmt.Errorf("get Open Design project export manifest: %w", err)
	}
	return manifest, nil
}

func (c *WorkerClient) GetProjectArchive(ctx context.Context, projectID string) ([]byte, error) {
	endpoint := c.endpoint("/api/projects/" + url.PathEscape(projectID) + "/archive")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/zip")
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get Open Design project archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, workerHTTPError(resp)
	}
	archive, err := io.ReadAll(io.LimitReader(resp.Body, maxWorkerArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Open Design project archive: %w", err)
	}
	if int64(len(archive)) > maxWorkerArchiveBytes {
		return nil, errors.New("Open Design project archive exceeds size limit")
	}
	if len(archive) == 0 {
		return nil, errors.New("Open Design worker returned an empty project archive")
	}
	return archive, nil
}

func (c *WorkerClient) GetProjectPackageAudit(ctx context.Context, projectID string) (PackageAudit, error) {
	var response struct {
		Audit workerPackageAudit `json:"audit"`
	}
	path := "/api/projects/" + url.PathEscape(projectID) + "/design-system-package-audit"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return PackageAudit{}, fmt.Errorf("get Open Design project package audit: %w", err)
	}
	if strings.TrimSpace(response.Audit.ProjectPath) == "" {
		return PackageAudit{}, errors.New("Open Design worker package audit omitted projectPath")
	}
	audit := response.Audit.PackageAudit
	if audit.OK != (len(audit.Errors) == 0) {
		return PackageAudit{}, errors.New("Open Design worker package audit ok does not match its errors")
	}
	audit.OK = audit.OK && len(audit.Warnings) == 0
	if err := ValidatePackageAudit(audit); err != nil {
		return PackageAudit{}, fmt.Errorf("validate Open Design project package audit: %w", err)
	}
	return audit, nil
}

func (c *WorkerClient) GetProjectPreviewURL(ctx context.Context, projectID string, target PreviewTarget) (PreviewURL, error) {
	if strings.TrimSpace(projectID) == "" {
		return PreviewURL{}, errors.New("Open Design worker project id is required for Preview")
	}
	if err := validatePreviewTarget(target); err != nil {
		return PreviewURL{}, err
	}

	endpoint := c.endpoint("/api/projects/" + url.PathEscape(projectID) + "/preview-url")
	query := endpoint.Query()
	query.Set("file", target.Path)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return PreviewURL{}, err
	}
	req.Header.Set("Accept", "application/json")
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PreviewURL{}, fmt.Errorf("get Open Design project Preview URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return PreviewURL{}, workerHTTPError(resp)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxWorkerResponseBytes+1))
	if err != nil {
		return PreviewURL{}, fmt.Errorf("read Open Design project Preview URL: %w", err)
	}
	if int64(len(data)) > maxWorkerResponseBytes {
		return PreviewURL{}, errors.New("Open Design worker Preview URL response exceeds size limit")
	}
	var response workerPreviewURLResponse
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&response); err != nil {
		return PreviewURL{}, fmt.Errorf("decode Open Design project Preview URL: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return PreviewURL{}, errors.New("Open Design worker Preview URL response must contain one JSON value")
	}

	previewURL, err := c.validateProjectPreviewURL(projectID, target, response)
	if err != nil {
		return PreviewURL{}, err
	}
	return PreviewURL{Target: target, URL: previewURL}, nil
}

func (c *WorkerClient) validateProjectPreviewURL(projectID string, target PreviewTarget, response workerPreviewURLResponse) (string, error) {
	if response.File != target.Path {
		return "", errors.New("Open Design worker Preview URL file does not match the requested target")
	}
	if !response.OpaqueOrigin || response.IframeSandbox != "allow-scripts allow-forms" ||
		strings.Contains(strings.ToLower(response.CSP), "allow-same-origin") ||
		!strings.Contains(response.CSP, "connect-src 'none'") ||
		!strings.Contains(response.CSP, "sandbox allow-scripts allow-forms") {
		return "", errors.New("Open Design worker Preview URL does not enforce the pinned sandbox policy")
	}

	parsed, err := url.Parse(strings.TrimSpace(response.URL))
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/") {
		return "", errors.New("Open Design worker returned an invalid scoped Preview URL")
	}
	prefix := "/api/projects/" + url.PathEscape(projectID) + "/preview/"
	escapedPath := parsed.EscapedPath()
	if !strings.HasPrefix(escapedPath, prefix) {
		return "", errors.New("Open Design worker Preview URL is outside the requested project scope")
	}
	scope, encodedFile, found := strings.Cut(strings.TrimPrefix(escapedPath, prefix), "/")
	if !found || !validPreviewScope(scope) || encodedFile != escapePreviewPath(target.Path) {
		return "", errors.New("Open Design worker returned an invalid Preview scope or file path")
	}

	resolved := c.baseURL.ResolveReference(parsed)
	if resolved.Scheme != c.baseURL.Scheme || resolved.Host != c.baseURL.Host {
		return "", errors.New("Open Design worker Preview URL is not same-origin")
	}
	return resolved.String(), nil
}

func validPreviewScope(scope string) bool {
	if len(scope) < 8 || len(scope) > 128 {
		return false
	}
	for _, char := range scope {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func escapePreviewPath(value string) string {
	segments := strings.Split(value, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	return strings.Join(segments, "/")
}

func (c *WorkerClient) CancelRun(ctx context.Context, runID string) (WorkerRunStatus, error) {
	var response struct {
		Run WorkerRunStatus `json:"run"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/runs/"+url.PathEscape(runID)+"/cancel", map[string]any{}, &response); err != nil {
		return WorkerRunStatus{}, fmt.Errorf("cancel Open Design worker run: %w", err)
	}
	return response.Run, nil
}

func (c *WorkerClient) doJSON(ctx context.Context, method, path string, body, destination any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path).String(), requestBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return workerHTTPError(resp)
	}
	if destination == nil {
		_, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxWorkerResponseBytes+1))
		return err
	}
	limited := io.LimitReader(resp.Body, maxWorkerResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(data)) > maxWorkerResponseBytes {
		return errors.New("Open Design worker response exceeds size limit")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("Open Design worker returned an empty response")
	}
	if raw, ok := destination.(*json.RawMessage); ok {
		if !json.Valid(data) {
			return errors.New("Open Design worker returned invalid JSON")
		}
		*raw = append((*raw)[:0], data...)
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Open Design worker response must contain one JSON value")
	}
	return nil
}

func (c *WorkerClient) endpoint(path string) *url.URL {
	return c.baseURL.ResolveReference(&url.URL{Path: path})
}

func (c *WorkerClient) authorize(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func workerHTTPError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &payload)
	code := strings.TrimSpace(payload.Error.Code)
	if code == "" {
		code = strings.TrimSpace(payload.Code)
	}
	message := strings.TrimSpace(payload.Error.Message)
	if message == "" {
		message = strings.TrimSpace(payload.Message)
	}
	if message == "" {
		message = strings.TrimSpace(string(data))
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return &workerHTTPStatusError{
		statusCode: resp.StatusCode,
		code:       code,
		message:    message,
	}
}
