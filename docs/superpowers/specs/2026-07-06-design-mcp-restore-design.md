# Multica Design MCP Restore Design

Status: ready for review
Date: 2026-07-06

## Goal

Provide a Multica MCP entry for real frontend engineers and their local coding agents to consume uploaded design data from Multica Design Center.

The first version should let an engineer restore from:

- a whole frame/page
- a whole Figma group
- selected layers
- a marquee selection bounds

After MCP setup, the user must not need to log in again within 30 days under normal use. The token must be persisted locally and reused by the MCP server.

## Product Position

This MCP line is for the manual/frontend-engineer path, not the UI Agent dispatch path.

The UI design Issue owner can still choose UI Agent restore. MCP exists for the other route: a real engineer opens their IDE or local coding agent, connects Multica MCP, and asks for the design context or Restore Pack for a specific design scope.

The frontend engineer must still be able to restore a single frame even when the frame belongs to a Figma group. Group restore is an additional scope, not a replacement for frame restore.

## Non-Goals

- Do not implement remote HTTP MCP with OAuth in v1.
- Do not expose raw database tables or arbitrary workspace APIs through MCP.
- Do not make MCP mutate the target repo directly. The coding agent or engineer writes code in the target repo.
- Do not make complex design-noise classification a blocking upload rule.
- Do not remove the existing Design Center copy-context and task queue features.
- Do not require UI designers to manage layer hygiene beyond naming and grouping conventions.

## Recommended Approach

Use a local stdio MCP server launched by the Multica CLI:

```bash
multica mcp serve design
```

The MCP server reads the existing CLI config:

```text
~/.multica/config.json
```

That config already carries:

- `server_url`
- `app_url`
- `workspace_id`
- `token`

This keeps v1 simple and reliable. Local editor agents connect to a command, not a remote HTTP endpoint. The token stays on the user's machine and is not copied into every MCP client config.

Remote Streamable HTTP MCP can be added later for cloud-hosted clients, but it should be a separate auth design because browser OAuth, redirect handling, token refresh, and workspace scoping are materially more complex.

## User Workflow

1. The user logs in once with the CLI:

   ```bash
   multica login
   ```

2. The user installs or prints MCP config:

   ```bash
   multica mcp setup design
   ```

   This validates the token, validates the default workspace, and prints client snippets for Codex, Claude Desktop, and OpenCode-style MCP configs.

3. In Multica Design Center, the user opens a design file or frame.

4. The user chooses a scope:

   - copy frame scope
   - copy group scope
   - copy selected layers scope
   - copy marquee bounds scope

5. In the IDE or local agent, the engineer asks:

   ```text
   Use Multica MCP to get the restore pack for this scope: <scope json or scope id>
   ```

6. The coding agent calls MCP tools, reads the Restore Pack, and implements UI code in the target repo.

## Authentication

### v1 Decision

Reuse the existing CLI PAT path.

The existing browser login flow creates a PAT and persists it in the CLI config file. That PAT currently has a longer lifetime than 30 days, so it satisfies the 30-day no-repeat-login requirement when setup succeeds.

The local MCP server should:

- read the token from CLI config
- never print the token in logs
- call `/api/me` at startup to validate auth
- call `/api/tokens/current/renew` at startup when the token is a `mul_` PAT
- continue using the token for API calls until the server returns 401
- return an actionable MCP error if re-login is required

The MCP setup command should refuse to write snippets when no valid token exists. It should tell the user to run `multica login`.

### Token Storage

Keep using `~/.multica/config.json` with file mode `0600`.

Do not write the raw token into generated MCP client snippets. The snippet should only launch:

```json
{
  "command": "multica",
  "args": ["mcp", "serve", "design"]
}
```

Profiles can be supported with:

```json
{
  "command": "multica",
  "args": ["--profile", "work", "mcp", "serve", "design"]
}
```

### Later Hardening

After v1 works, add a narrower MCP-scoped token:

- prefix: `mcp_` or a dedicated existing token family if product naming settles there
- workspace scoped
- read-only design permissions
- 30 to 90 day expiry
- revocable from Settings

This is not required for v1 because the existing PAT path is already used by local developer tooling.

## Scope Model

Introduce a stable scope object, `DesignRestoreScopeV1`.

```json
{
  "version": "1.0",
  "kind": "frame",
  "designFileId": "design-file-id",
  "revisionId": "revision-id",
  "frameId": "frame-id",
  "label": "钱包首页 - 已绑定支付宝",
  "sourcePageUrl": "http://localhost:3031/amc/designs/..."
}
```

Supported `kind` values:

- `frame`
- `figma_group`
- `selected_layers`
- `selection_bounds`

### Frame Scope

Used for a single page or a single state.

Required fields:

- `designFileId`
- `revisionId`
- `frameId`

The MCP server calls the Restore Pack API for one frame.

### Figma Group Scope

Used when one Figma group expresses one business page with states, result states, and modals.

Required fields:

- `designFileId`
- `revisionId`
- `groupId` or `groupPath`
- `frameIds`

Recommended shape:

```json
{
  "version": "1.0",
  "kind": "figma_group",
  "designFileId": "design-file-id",
  "revisionId": "revision-id",
  "groupId": "4-189",
  "groupName": "钱包首页",
  "groupPath": ["钱包首页"],
  "frameIds": ["frame-1", "frame-2", "frame-3"],
  "label": "钱包首页"
}
```

The server expands the group into ordered frame contexts using existing native JSON `restoreHints.figmaGroups` and frame `source.group*` metadata.

Important rule: group restore must not hide individual frame restore. The design page should still let users open and copy scope for a single frame inside the group.

### Selected Layers Scope

Used for precise partial restore inside one frame.

Required fields:

- `designFileId`
- `revisionId`
- `frameId`
- `layerIds`

The server uses existing selection context logic and returns the selected layer tree, referenced assets, text, colors, and bounds.

### Selection Bounds Scope

Used for marquee/box selection.

Required fields:

- `designFileId`
- `revisionId`
- `frameId`
- `selectionBounds`

Recommended shape:

```json
{
  "version": "1.0",
  "kind": "selection_bounds",
  "designFileId": "design-file-id",
  "revisionId": "revision-id",
  "frameId": "frame-1",
  "selectionBounds": {
    "x": 120,
    "y": 240,
    "width": 360,
    "height": 180
  },
  "includeIntersectingLayers": true
}
```

Default `includeIntersectingLayers` is `true`, matching the existing backend selection context behavior.

## Restore Pack

The MCP server should not hand raw Figma/native JSON to agents as the primary payload.

It should request a server-built Restore Pack that is clean enough for implementation:

```json
{
  "version": "1.0",
  "designFile": {
    "id": "design-file-id",
    "title": "钱包",
    "sourceTool": "figma"
  },
  "revision": {
    "id": "revision-id",
    "number": 3
  },
  "scope": {},
  "frames": [],
  "designStructure": {},
  "assets": {},
  "texts": [],
  "colors": [],
  "implementationHints": {},
  "warnings": [],
  "provenance": {}
}
```

The pack should include:

- selected frame or grouped frames
- normalized page/state/modal interpretation from frame names
- core visible text
- core colors
- key image/export assets
- layer bounds and layout hints
- interaction cues for `请选择`, `请输入`, buttons, tabs, switches, and bottom sheets
- warnings when the selection is empty, huge, stale, or ambiguous
- source provenance so engineers can open the exact Design Center URL

The pack should avoid:

- hidden layers
- full raw native JSON by default
- full-frame preview as the implementation source
- complex speculative noise removal

## Design Noise Policy

Keep noise processing conservative.

Rules:

- Hidden layers should not be included in Restore Pack.
- A visible layer with an exported image or image fill is high priority and must be preserved as an asset candidate.
- If a layer has a concrete image asset, the agent should render that asset rather than redrawing it from imagination.
- Text like `请选择` should become a project-appropriate select/popover interaction when surrounding UI implies a picker.
- Text like `请输入` should become a project-appropriate input/form interaction when surrounding UI implies an input.
- Suspected annotation or draft layers can be warnings, not blockers.
- Noise analysis should be internal to the pack. Do not add user-facing complexity in v1.

## MCP Tools

Tool names should be prefixed to avoid collisions.

### `multica_design_list_files`

Lists accessible design files in the configured workspace.

Arguments:

- `projectId` optional
- `folderId` optional
- `query` optional

Returns file IDs, titles, current revision IDs, project/folder metadata, and Design Center URLs.

### `multica_design_list_frames`

Lists frames for one design file revision.

Arguments:

- `designFileId`
- `revisionId` optional

Returns frames, names, dimensions, group metadata, and per-frame scope snippets.

### `multica_design_list_groups`

Lists Figma groups from a design file revision.

Arguments:

- `designFileId`
- `revisionId` optional

Returns group ID, group path, frame count, ordered frame IDs, child frame names, and group scope snippets.

### `multica_design_get_restore_pack`

Returns the server-built Restore Pack for a scope.

Arguments:

- `scope`: `DesignRestoreScopeV1`
- `detailLevel`: `compact | normal | full`, default `normal`

`compact` is for first-pass planning. `normal` is for implementation. `full` may include more raw context but should still avoid dumping unnecessary native JSON.

### `multica_design_get_selection_context`

Returns selected layer context directly for debugging or precise partial restore.

Arguments:

- `designFileId`
- `revisionId`
- `frameId`
- `layerIds` optional
- `selectionBounds` optional
- `includeIntersectingLayers` optional

This maps to the existing backend selection context endpoint.

### `multica_design_get_ui_restore_artifact`

Reads the UI restore artifact document created by UI Agent, when a frontend issue receives a completed UI restore delivery.

Arguments:

- `artifactDocPath`
- `projectLocalPath` optional

The first implementation can return instructions and path metadata. Reading the file itself happens in the target repo by the coding agent. Later, if Multica stores artifact contents, this tool can return the content too.

## Server API Boundary

Add a server-side Restore Pack API instead of building the pack entirely inside the CLI.

Recommended endpoint:

```http
POST /api/design-files/{id}/restore-pack?workspace_id=...
```

Request:

```json
{
  "scope": {},
  "detailLevel": "normal"
}
```

Reasons:

- Backend already has revision access, native JSON parsing, group metadata, and selection context logic.
- UI Agent and MCP should share restore semantics.
- Future frontend UI can preview or copy the same scope contracts without duplicating pack generation.
- CLI stays a thin local MCP adapter.

Existing endpoints remain valid:

- `GET /api/design-files/{id}/context`
- `GET /api/design-files/{id}/frames/{frameId}/context`
- `POST /api/design-files/{id}/frames/{frameId}/selection-context`

## CLI Boundary

Add CLI commands:

```bash
multica mcp setup design
multica mcp serve design
multica mcp status design
```

`setup`:

- validates CLI config
- validates token with `/api/me`
- validates workspace with `/api/workspaces`
- prints config snippets
- does not write raw token into snippets

`serve`:

- starts stdio MCP JSON-RPC server
- validates config on first tool call
- calls token renew on startup
- exposes design tools only
- redacts tokens from logs and errors

`status`:

- shows configured server URL, app URL, workspace ID, and auth state
- never prints token

## Error Handling

MCP errors should be actionable:

- `not_authenticated`: run `multica login`
- `workspace_missing`: run `multica workspace switch <workspace>`
- `design_file_not_found`: open Design Center and verify the copied scope
- `revision_not_found`: scope points to a deleted or inaccessible revision
- `frame_not_found`: frame no longer exists in the revision
- `group_not_found`: group metadata is absent or stale
- `empty_selection`: selection resolved to no visible layers
- `restore_pack_too_large`: retry with `detailLevel=compact` or a smaller scope

Do not return raw Go/SQL errors through MCP.

## Security

- MCP v1 is read-only for design data.
- Token remains in CLI config, not generated client snippets.
- File permissions stay `0600`.
- Logs must redact token-like values.
- Task/agent tokens should not be accepted for user-level MCP setup.
- Later MCP-scoped tokens should be workspace-scoped and design-read-only.

## Testing Plan

Backend tests:

- Restore Pack for frame scope.
- Restore Pack for Figma group scope using `restoreHints.figmaGroups`.
- Restore Pack for selected layer scope.
- Restore Pack for selection bounds scope.
- Hidden layers are excluded when present.
- Image/export asset layers are preserved in pack asset hints.
- `请选择` and `请输入` produce interaction hints.

CLI/MCP tests:

- `multica mcp setup design` refuses missing token.
- `multica mcp setup design` prints snippets without token.
- `multica mcp serve design` lists tools through JSON-RPC.
- `multica_design_get_restore_pack` calls server API with Bearer token.
- 401 maps to `not_authenticated`.

Frontend tests:

- Design file page can copy group scope.
- Design frame page can copy frame scope.
- Multi-select can copy selected layer scope.
- Marquee can copy selection bounds scope.
- Group UI still exposes child frame scope.

Manual validation:

- Configure one local MCP client.
- Fetch a whole frame restore pack.
- Fetch a whole group restore pack.
- Fetch selected layer and marquee restore packs.
- Confirm no re-login is required after restarting editor and MCP server.

## Rollout

Phase 1: local MCP server and Restore Pack API.

Phase 2: Design Center copy-scope actions for frame, group, selected layers, and marquee bounds.

Phase 3: documentation for frontend engineers, including examples for Codex, Claude Desktop, and OpenCode.

Phase 4: optional remote HTTP MCP with OAuth and narrower MCP-scoped token.

## Acceptance Criteria

- A user who has run setup can restart the editor and use MCP without logging in again within 30 days.
- The MCP config snippets do not contain raw tokens.
- MCP can fetch restore packs for frame, group, selected layers, and selection bounds.
- Group restore preserves child frames and does not remove single-frame restore.
- Restore Pack preserves visible exported/image layers as asset candidates.
- Restore Pack includes interaction hints for select/input-like design text.
- Restore Pack avoids full raw native JSON as the default agent payload.
- The first implementation is read-only from MCP's perspective.
