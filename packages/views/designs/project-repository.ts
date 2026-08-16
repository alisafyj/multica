import type { ProjectResource } from "@multica/core/types";

/**
 * Repository identity helpers shared by the design-centre surfaces that let a
 * user pick one repository out of a project (DC-052). Repository names repeat
 * across hosts and get truncated in narrow columns, so callers pair the label
 * with the URL as a title.
 */
export function repositoryUrl(resource: ProjectResource): string {
  const ref = resource.resource_ref as { url?: unknown } | undefined;
  return typeof ref?.url === "string" ? ref.url.trim() : "";
}

export function repositoryLabel(resource: ProjectResource): string {
  const label = resource.label?.trim();
  if (label) return label;
  const url = repositoryUrl(resource);
  if (!url) return "未命名仓库";
  const normalized = url.replace(/\.git$/, "").replace(/\/+$/, "");
  return normalized.split("/").pop() || normalized;
}
