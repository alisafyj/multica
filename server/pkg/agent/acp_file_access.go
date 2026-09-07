package agent

import "encoding/json"

// acpFileAccessParams excludes literal replacement text only for the explicit
// write_file edit shape. The destination and all other request metadata still
// pass through the filesystem guard. Unknown tools/shapes retain the generic
// conservative scan; a shell command must never acquire a content exemption.
func acpFileAccessParams(raw json.RawMessage) json.RawMessage {
	var params map[string]json.RawMessage
	if json.Unmarshal(raw, &params) != nil {
		return raw
	}
	var call map[string]json.RawMessage
	if json.Unmarshal(params["toolCall"], &call) != nil {
		return raw
	}
	var kind string
	if json.Unmarshal(call["kind"], &kind) != nil || kind != "edit" {
		return raw
	}
	var input map[string]json.RawMessage
	if json.Unmarshal(call["rawInput"], &input) != nil {
		return raw
	}
	var tool string
	if json.Unmarshal(input["tool"], &tool) != nil || tool != "write_file" {
		return raw
	}
	var args map[string]json.RawMessage
	if json.Unmarshal(input["arguments"], &args) != nil {
		return raw
	}
	var path, body string
	if json.Unmarshal(args["path"], &path) != nil || path == "" || json.Unmarshal(args["content"], &body) != nil {
		return raw
	}
	var blocks []map[string]json.RawMessage
	if json.Unmarshal(call["content"], &blocks) != nil || len(blocks) != 1 {
		return raw
	}
	var blockType, blockPath, newText string
	block := blocks[0]
	if json.Unmarshal(block["type"], &blockType) != nil || blockType != "diff" ||
		json.Unmarshal(block["path"], &blockPath) != nil || blockPath != path ||
		json.Unmarshal(block["newText"], &newText) != nil || newText != body {
		return raw
	}
	// Both raw input and preview describe the same literal write. Retain the
	// two path fields and any unfamiliar fields for the ordinary guard.
	delete(args, "content")
	delete(block, "newText")
	delete(block, "oldText")
	input["arguments"], _ = json.Marshal(args)
	call["rawInput"], _ = json.Marshal(input)
	call["content"], _ = json.Marshal(blocks)
	params["toolCall"], _ = json.Marshal(call)
	filtered, err := json.Marshal(params)
	if err != nil {
		return raw
	}
	return filtered
}
