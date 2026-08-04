package opendesign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const maxAgentInventoryBytes int64 = 2 << 20

type ProbeClient struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

type workerAgentInventory struct {
	Agents []workerAgent `json:"agents"`
}

type workerAgent struct {
	ID           string             `json:"id"`
	Available    bool               `json:"available"`
	AuthStatus   *string            `json:"authStatus"`
	AuthMessage  string             `json:"authMessage"`
	Version      *string            `json:"version"`
	Models       []workerAgentModel `json:"models"`
	ModelsSource string             `json:"modelsSource"`
	Diagnostics  []struct {
		Message string `json:"message"`
	} `json:"diagnostics"`
}

type workerAgentModel struct {
	ID      string `json:"id"`
	Enabled *bool  `json:"enabled"`
}

func NewProbeClient(rawBaseURL, token string, httpClient *http.Client) (*ProbeClient, error) {
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
	if httpClient == nil {
		return nil, errors.New("Open Design worker HTTP client is required")
	}
	baseURL.Path = ""
	return &ProbeClient{
		baseURL:    baseURL,
		token:      strings.TrimSpace(token),
		httpClient: httpClient,
	}, nil
}

func (c *ProbeClient) Probe(ctx context.Context, expected ExpectedPreflight, pluginPolicy string) (PreflightReport, error) {
	if err := expected.Engine.Validate(); err != nil {
		return PreflightReport{}, fmt.Errorf("validate expected Open Design engine: %w", err)
	}
	if strings.TrimSpace(expected.AdapterID) == "" {
		return PreflightReport{}, errors.New("expected Open Design adapter_id is required")
	}
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: "/api/agents"})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return PreflightReport{}, fmt.Errorf("build Open Design agent inventory request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PreflightReport{}, fmt.Errorf("load Open Design agent inventory: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return PreflightReport{}, fmt.Errorf("load Open Design agent inventory: status %d", resp.StatusCode)
	}
	var inventory workerAgentInventory
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxAgentInventoryBytes+1))
	if err := decoder.Decode(&inventory); err != nil {
		return PreflightReport{}, fmt.Errorf("decode Open Design agent inventory: %w", err)
	}
	if len(inventory.Agents) == 0 {
		return PreflightReport{}, errors.New("Open Design agent inventory is empty")
	}
	var selected *workerAgent
	for index := range inventory.Agents {
		if inventory.Agents[index].ID != expected.AdapterID {
			continue
		}
		if selected != nil {
			return PreflightReport{}, fmt.Errorf("Open Design agent inventory contains duplicate adapter_id %q", expected.AdapterID)
		}
		selected = &inventory.Agents[index]
	}
	if selected == nil {
		return PreflightReport{}, fmt.Errorf("Open Design adapter_id %q is not registered", expected.AdapterID)
	}

	report := PreflightReport{
		Schema:    PreflightSchema,
		Engine:    expected.Engine,
		AdapterID: expected.AdapterID,
		Model:     expected.Model,
		Binary: ProbeResult{
			Status:  ProbeFailed,
			Message: workerAgentMessage(*selected),
		},
		Auth: ProbeResult{
			Status: ProbeUnknown,
		},
		ModelProbe: ProbeResult{
			Status: ProbeFailed,
		},
		Plugins: PluginPreflight{Policy: pluginPolicy},
	}
	if selected.Version != nil {
		report.Binary.Version = strings.TrimSpace(*selected.Version)
	}
	if selected.Available {
		report.Binary.Status = ProbePassed
	}
	report.Auth = authProbeResult(*selected)
	report.ModelProbe = modelProbeResult(*selected, expected.Model)
	return report, nil
}

func authProbeResult(agent workerAgent) ProbeResult {
	if agent.AuthStatus == nil {
		return ProbeResult{Status: ProbeUnknown, Required: false}
	}
	message := strings.TrimSpace(agent.AuthMessage)
	switch *agent.AuthStatus {
	case "ok":
		return ProbeResult{Status: ProbePassed, Required: true, Message: message}
	case "missing":
		return ProbeResult{Status: ProbeFailed, Required: true, Message: message}
	default:
		return ProbeResult{Status: ProbeUnknown, Required: true, Message: message}
	}
}

func modelProbeResult(agent workerAgent, expectedModel string) ProbeResult {
	if strings.TrimSpace(expectedModel) == "" {
		return ProbeResult{Status: ProbePassed, Message: "worker default model"}
	}
	if agent.ModelsSource != "live" {
		return ProbeResult{Status: ProbeFailed, Message: "model inventory is not live"}
	}
	for _, model := range agent.Models {
		if model.ID != expectedModel {
			continue
		}
		if model.Enabled != nil && !*model.Enabled {
			return ProbeResult{Status: ProbeFailed, Message: "selected model is disabled"}
		}
		return ProbeResult{Status: ProbePassed}
	}
	return ProbeResult{Status: ProbeFailed, Message: "selected model was not reported by the worker"}
}

func workerAgentMessage(agent workerAgent) string {
	messages := make([]string, 0, len(agent.Diagnostics))
	for _, diagnostic := range agent.Diagnostics {
		message := strings.TrimSpace(diagnostic.Message)
		if message != "" {
			messages = append(messages, message)
		}
		if len(messages) == 3 {
			break
		}
	}
	return strings.Join(messages, "; ")
}
