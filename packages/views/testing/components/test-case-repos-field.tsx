"use client";

import { useQuery } from "@tanstack/react-query";
import { Plus, X } from "lucide-react";
import { projectResourcesOptions } from "@multica/core/projects";
import { TEST_CASE_REPO_ROLES } from "@multica/core/testing";
import type { TestCaseRepo, TestCaseRepoRole } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { NativeSelect } from "@multica/ui/components/ui/native-select";
import { useT } from "../../i18n";

interface TestCaseReposFieldProps {
  wsId: string;
  projectId: string;
  value: TestCaseRepo[];
  onChange: (next: TestCaseRepo[]) => void;
  disabled?: boolean;
}

/** Only these resource types carry code a test can be run against. */
const REPO_RESOURCE_TYPES = new Set(["github_repo", "local_directory"]);

function defaultAlias(url: string): string {
  const trimmed = url.replace(/\.git$/, "").replace(/\/+$/, "");
  const last = trimmed.split("/").pop() ?? "";
  return last;
}

/**
 * Edits which repositories of the project a case touches. Bindings reference
 * `project_resource_id`, so the picker offers exactly what the project has
 * attached — there is no free-text repo entry to drift from the project.
 */
export function TestCaseReposField({
  wsId,
  projectId,
  value,
  onChange,
  disabled = false,
}: TestCaseReposFieldProps) {
  const { t } = useT("testing");
  const { data: resources = [] } = useQuery(projectResourcesOptions(wsId, projectId));

  const repoResources = resources.filter((resource) =>
    REPO_RESOURCE_TYPES.has(resource.resource_type),
  );
  const available = repoResources.filter(
    (resource) => !value.some((repo) => repo.project_resource_id === resource.id),
  );

  function patchRepo(position: number, patch: Partial<TestCaseRepo>) {
    onChange(value.map((repo, index) => (index === position ? { ...repo, ...patch } : repo)));
  }

  function addRepo() {
    const next = available[0];
    if (!next) return;
    const ref = next.resource_ref as { url?: string; local_path?: string } | undefined;
    const alias = next.label ?? defaultAlias(ref?.url ?? ref?.local_path ?? "");
    onChange([
      ...value,
      {
        project_resource_id: next.id,
        alias,
        role: "under_test",
        path_globs: [],
      },
    ]);
  }

  if (repoResources.length === 0) {
    return <p className="text-caption text-muted-foreground">{t(($) => $.repos.noResources)}</p>;
  }

  return (
    <div className="flex flex-col gap-2">
      {value.length === 0 ? (
        <p className="text-caption text-muted-foreground">{t(($) => $.repos.empty)}</p>
      ) : null}

      {value.map((repo, position) => (
        <div
          key={repo.project_resource_id}
          className="flex items-start gap-2 rounded-md border border-border p-2"
        >
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <NativeSelect
              value={repo.project_resource_id}
              disabled={disabled}
              aria-label={t(($) => $.repos.add)}
              onChange={(event) => patchRepo(position, { project_resource_id: event.target.value })}
            >
              {repoResources.map((resource) => {
                const ref = resource.resource_ref as { url?: string; local_path?: string } | undefined;
                return (
                  <option key={resource.id} value={resource.id}>
                    {resource.label ?? ref?.url ?? ref?.local_path ?? resource.id}
                  </option>
                );
              })}
            </NativeSelect>
            <Input
              value={repo.alias}
              disabled={disabled}
              aria-label={t(($) => $.repos.alias)}
              placeholder={t(($) => $.repos.aliasPlaceholder)}
              onChange={(event) => patchRepo(position, { alias: event.target.value })}
            />
            <NativeSelect
              value={repo.role}
              disabled={disabled}
              aria-label={t(($) => $.repos.role)}
              onChange={(event) =>
                patchRepo(position, { role: event.target.value as TestCaseRepoRole })
              }
            >
              {TEST_CASE_REPO_ROLES.map((role) => (
                <option key={role} value={role}>
                  {t(($) => $.role[role])}
                </option>
              ))}
            </NativeSelect>
            <Input
              value={repo.path_globs.join(", ")}
              disabled={disabled}
              aria-label={t(($) => $.repos.pathGlobs)}
              placeholder={t(($) => $.repos.pathGlobsPlaceholder)}
              onChange={(event) =>
                patchRepo(position, {
                  path_globs: event.target.value
                    .split(",")
                    .map((glob) => glob.trim())
                    .filter((glob) => glob.length > 0),
                })
              }
            />
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            disabled={disabled}
            aria-label={t(($) => $.repos.remove)}
            onClick={() => onChange(value.filter((_repo, index) => index !== position))}
          >
            <X className="size-4" />
          </Button>
        </div>
      ))}

      <Button
        type="button"
        variant="outline"
        size="sm"
        className="self-start"
        disabled={disabled || available.length === 0}
        onClick={addRepo}
      >
        <Plus className="size-4" />
        {t(($) => $.repos.add)}
      </Button>
    </div>
  );
}
