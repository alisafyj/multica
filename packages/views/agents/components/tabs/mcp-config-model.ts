export type McpConfigContainer = "mcpServers" | "mcp";

export type ManagedMcpServer = {
  name: string;
  config: Record<string, unknown>;
  container: McpConfigContainer;
  transport: string;
  enabled: boolean;
};

export type RuntimeMcpPolicy =
  | { mode: "inherit"; allow: [] }
  | { mode: "deny_all"; allow: [] }
  | { mode: "allowlist"; allow: string[] };

export function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

export function mcpTransport(config: Record<string, unknown>): string {
  const type = typeof config.type === "string" ? config.type.toLowerCase() : "";
  if (config.command || type === "local" || type === "stdio") return "stdio";
  if (type === "sse") return "sse";
  if (
    config.url ||
    type === "remote" ||
    type === "http" ||
    type === "streamable-http"
  ) {
    return "http";
  }
  return "unknown";
}

export function listManagedMcpServers(value: unknown): ManagedMcpServer[] {
  if (!isRecord(value)) return [];

  const out: ManagedMcpServer[] = [];
  const seen = new Set<string>();
  for (const container of ["mcpServers", "mcp"] as const) {
    const raw = value[container];
    if (!isRecord(raw)) continue;
    for (const [name, entry] of Object.entries(raw)) {
      if (seen.has(name) || !isRecord(entry)) continue;
      seen.add(name);
      out.push({
        name,
        config: entry,
        container,
        transport: mcpTransport(entry),
        enabled: entry.enabled !== false && entry.disabled !== true,
      });
    }
  }
  return out.sort((a, b) => a.name.localeCompare(b.name));
}

export function upsertManagedMcpServer(
  value: unknown,
  previous: ManagedMcpServer | null,
  name: string,
  config: Record<string, unknown>,
): Record<string, unknown> {
  const document = isRecord(value) ? { ...value } : {};
  const container: McpConfigContainer = previous?.container ?? "mcpServers";

  if (previous) {
    const previousMap = isRecord(document[previous.container])
      ? { ...(document[previous.container] as Record<string, unknown>) }
      : {};
    delete previousMap[previous.name];
    document[previous.container] = previousMap;
  }

  const target = isRecord(document[container])
    ? { ...(document[container] as Record<string, unknown>) }
    : {};
  target[name] = config;
  document[container] = target;
  return document;
}

export function removeManagedMcpServer(
  value: unknown,
  server: ManagedMcpServer,
): Record<string, unknown> | null {
  if (!isRecord(value)) return null;
  const document = { ...value };
  const container = isRecord(document[server.container])
    ? { ...(document[server.container] as Record<string, unknown>) }
    : {};
  delete container[server.name];

  if (Object.keys(container).length > 0) document[server.container] = container;
  else delete document[server.container];

  return Object.keys(document).length > 0 ? document : null;
}

export function getRuntimeMcpPolicy(value: unknown): RuntimeMcpPolicy {
  if (!isRecord(value)) return { mode: "inherit", allow: [] };
  const multica = value._multica;
  if (!isRecord(multica)) return { mode: "inherit", allow: [] };
  const runtimeMcp = multica.runtimeMcp;
  if (!isRecord(runtimeMcp)) return { mode: "inherit", allow: [] };

  if (runtimeMcp.mode === "deny_all") return { mode: "deny_all", allow: [] };
  if (runtimeMcp.mode !== "allowlist") return { mode: "inherit", allow: [] };

  const allow = Array.isArray(runtimeMcp.allow)
    ? Array.from(
        new Set(
          runtimeMcp.allow.filter(
            (name): name is string => typeof name === "string" && name.length > 0,
          ),
        ),
      )
    : [];
  return { mode: "allowlist", allow };
}

export function setRuntimeMcpPolicy(
  value: unknown,
  policy: RuntimeMcpPolicy,
): Record<string, unknown> {
  const document = isRecord(value) ? { ...value } : {};
  const multica = isRecord(document._multica)
    ? { ...document._multica }
    : {};

  multica.runtimeMcp =
    policy.mode === "allowlist"
      ? { mode: "allowlist", allow: Array.from(new Set(policy.allow)) }
      : { mode: policy.mode };
  document._multica = multica;
  return document;
}
