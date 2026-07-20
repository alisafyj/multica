package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type designMCPToolCaller interface {
	callTool(ctx context.Context, name string, arguments map[string]any) (any, error)
}

type designMCPServer struct {
	adapter designMCPToolCaller
}

func newDesignMCPServer(adapter designMCPToolCaller) *designMCPServer {
	return &designMCPServer{adapter: adapter}
}

type mcpJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	hasID   bool
}

type mcpJSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *mcpJSONRPCError `json:"error,omitempty"`
}

type mcpJSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *designMCPServer) serve(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		req, err := decodeMCPJSONRPCRequest(scanner.Bytes())
		if err != nil {
			resp := mcpJSONRPCResponse{JSONRPC: "2.0", Error: &mcpJSONRPCError{Code: -32700, Message: "parse error"}}
			if err := encoder.Encode(resp); err != nil {
				return err
			}
			continue
		}
		if !req.hasID {
			continue
		}
		resp := s.handleRequest(context.Background(), req)
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func decodeMCPJSONRPCRequest(data []byte) (mcpJSONRPCRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return mcpJSONRPCRequest{}, err
	}
	var req mcpJSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return mcpJSONRPCRequest{}, err
	}
	_, req.hasID = raw["id"]
	return req, nil
}

func (s *designMCPServer) handleRequest(ctx context.Context, req mcpJSONRPCRequest) mcpJSONRPCResponse {
	resp := mcpJSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "multica-design", "version": version},
		}
	case "tools/list":
		resp.Result = map[string]any{"tools": designMCPToolDescriptors()}
	case "tools/call":
		result, err := s.handleToolCall(ctx, req.Params)
		if err != nil {
			resp.Error = &mcpJSONRPCError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = result
	default:
		resp.Error = &mcpJSONRPCError{Code: -32601, Message: "method not found"}
	}
	return resp
}

func (s *designMCPServer) handleToolCall(ctx context.Context, params json.RawMessage) (any, error) {
	if s.adapter == nil {
		return nil, fmt.Errorf("design MCP adapter is not configured")
	}
	var req struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid tools/call params")
	}
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}
	content, err := s.adapter.callTool(ctx, req.Name, req.Arguments)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": mcpTextContent(content),
		}},
	}, nil
}

func mcpTextContent(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}
