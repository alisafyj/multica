// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  getRuntimeMcpPolicy,
  listManagedMcpServers,
  mcpTransportLabel,
  removeManagedMcpServer,
  setRuntimeMcpPolicy,
  upsertManagedMcpServer,
} from "./mcp-config-model";

describe("mcp config compatibility model", () => {
  it("uses the same user-facing transport names across MCP surfaces", () => {
    expect(mcpTransportLabel("stdio")).toBe("STDIO");
    expect(mcpTransportLabel("LOCAL")).toBe("STDIO");
    expect(mcpTransportLabel("http")).toBe("Streamable HTTP");
    expect(mcpTransportLabel("remote")).toBe("Streamable HTTP");
    expect(mcpTransportLabel("streamable-http")).toBe("Streamable HTTP");
    expect(mcpTransportLabel("sse")).toBe("SSE");
    expect(mcpTransportLabel(" websocket ")).toBe("websocket");
    expect(mcpTransportLabel("   ")).toBe("Unknown");
  });

  it("reads canonical and historical OpenCode-native containers without duplication", () => {
    const servers = listManagedMcpServers({
      mcpServers: { shared: { command: "canonical" } },
      mcp: {
        shared: { type: "local", command: ["native"] },
        legacy: { type: "remote", url: "https://example.test/mcp" },
      },
    });

    expect(servers.map(({ name, container }) => ({ name, container }))).toEqual([
      { name: "legacy", container: "mcp" },
      { name: "shared", container: "mcpServers" },
    ]);
  });

  it("updates a single legacy entry while preserving unknown document fields", () => {
    const value = {
      provider: "opencode",
      mcp: {
        legacy: { type: "remote", url: "https://old.test/mcp" },
        sibling: { type: "local", command: ["node", "server.js"] },
      },
    };
    const legacy = listManagedMcpServers(value).find(
      (server) => server.name === "legacy",
    );
    expect(legacy).toBeDefined();

    expect(
      upsertManagedMcpServer(
        value,
        legacy!,
        "legacy",
        { type: "remote", url: "https://new.test/mcp" },
      ),
    ).toEqual({
      provider: "opencode",
      mcp: {
        legacy: { type: "remote", url: "https://new.test/mcp" },
        sibling: { type: "local", command: ["node", "server.js"] },
      },
    });
  });

  it("returns null only when deleting the last value from an otherwise empty document", () => {
    const value = { mcpServers: { fetch: { command: "uvx" } } };
    const [fetch] = listManagedMcpServers(value);
    expect(removeManagedMcpServer(value, fetch!)).toBeNull();

    const withMetadata = { version: 1, ...value };
    const [withMetadataFetch] = listManagedMcpServers(withMetadata);
    expect(removeManagedMcpServer(withMetadata, withMetadataFetch!)).toEqual({
      version: 1,
    });
  });

  it("defaults a missing runtime MCP policy to inherit", () => {
    expect(getRuntimeMcpPolicy(null)).toEqual({ mode: "inherit", allow: [] });
    expect(getRuntimeMcpPolicy({ _multica: { future: true } })).toEqual({
      mode: "inherit",
      allow: [],
    });
  });

  it("reads only valid saved allowlist names", () => {
    expect(
      getRuntimeMcpPolicy({
        _multica: {
          runtimeMcp: {
            mode: "allowlist",
            allow: ["linear", "docs", "linear"],
          },
        },
      }),
    ).toEqual({ mode: "allowlist", allow: ["linear", "docs"] });
  });

  it("updates runtime MCP policy without replacing MCP servers or unknown metadata", () => {
    const value = {
      version: 1,
      mcpServers: { fetch: { command: "uvx" } },
      _multica: {
        future: { enabled: true },
        runtimeMcp: { mode: "inherit" },
      },
    };

    expect(
      setRuntimeMcpPolicy(value, {
        mode: "allowlist",
        allow: ["linear", "docs", "linear"],
      }),
    ).toEqual({
      version: 1,
      mcpServers: { fetch: { command: "uvx" } },
      _multica: {
        future: { enabled: true },
        runtimeMcp: { mode: "allowlist", allow: ["linear", "docs"] },
      },
    });
  });

  it("preserves runtime MCP policy while editing and deleting managed servers", () => {
    const value = {
      _multica: {
        runtimeMcp: { mode: "allowlist", allow: ["linear"] },
        future: "keep",
      },
      mcpServers: { fetch: { command: "uvx" } },
    };
    const [fetch] = listManagedMcpServers(value);

    const updated = upsertManagedMcpServer(
      value,
      fetch!,
      "fetch",
      { command: "npx" },
    );
    expect(updated._multica).toEqual(value._multica);
    expect(removeManagedMcpServer(value, fetch!)?._multica).toEqual(
      value._multica,
    );
  });
});
