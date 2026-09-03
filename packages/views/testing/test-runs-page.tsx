"use client";

import { useQuery } from "@tanstack/react-query";
import { Play } from "lucide-react";
import { projectListOptions } from "@multica/core/projects/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { testRunListOptions, useTestCaseViewStore } from "@multica/core/testing";
import type { TestRun } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { NativeSelect } from "@multica/ui/components/ui/native-select";
import { PageHeader } from "../layout/page-header";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { TestsTabs } from "./components/tests-tabs";
import { resolveSelectedProjectId } from "./project-selection";

/** The list endpoint's own ceiling; it has no cursor to page past it. */
const RUN_PAGE_LIMIT = 200;

/**
 * Every execution round of the selected project.
 *
 * Runs used to have detail pages and no index: once you navigated away from a
 * round, the only way back was a link on a case that happened to be in it. A
 * regression record you cannot enumerate is not a record.
 */
export function TestRunsPage() {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();

  const projectId = useTestCaseViewStore((state) => state.projectId);
  const setProjectId = useTestCaseViewStore((state) => state.setProjectId);
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const selectedProjectId = resolveSelectedProjectId(projects, projectId);

  // The endpoint defaults to 50 and caps at 200, with no cursor. Ask for the
  // cap rather than the default, and tell the user when the answer is truncated
  // — an index that silently stops at the newest N reads as "this is all of
  // them", which is the one thing a history surface must not imply.
  const { data: runs = [], isLoading } = useQuery({
    ...testRunListOptions(wsId, { projectId: selectedProjectId, limit: RUN_PAGE_LIMIT }),
    enabled: selectedProjectId.length > 0,
  });
  const truncated = runs.length >= RUN_PAGE_LIMIT;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <Play className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-body font-medium">{t(($) => $.runs.title)}</h1>
        </div>
        <NativeSelect
          className="h-8 w-40 shrink-0 text-caption"
          aria-label={t(($) => $.page.selectProject)}
          value={selectedProjectId}
          onChange={(event) => setProjectId(event.target.value)}
        >
          {projects.map((project) => (
            <option key={project.id} value={project.id}>
              {project.title}
            </option>
          ))}
        </NativeSelect>
      </PageHeader>

      <TestsTabs active="runs" />

      <div className="min-h-0 flex-1 overflow-auto">
        {isLoading ? null : runs.length === 0 ? (
          <div className="flex flex-col items-center gap-1 p-12 text-center">
            <p className="text-body font-medium">{t(($) => $.runs.empty)}</p>
            <p className="text-caption text-muted-foreground">{t(($) => $.runs.emptyHint)}</p>
          </div>
        ) : (
          <table className="w-full text-body">
            <thead>
              <tr className="border-b border-border text-caption text-muted-foreground">
                <Th>{t(($) => $.runs.columns.title)}</Th>
                <Th>{t(($) => $.runs.columns.status)}</Th>
                <Th>{t(($) => $.runs.columns.executor)}</Th>
                <Th>{t(($) => $.runs.columns.environment)}</Th>
                <Th>{t(($) => $.runs.columns.buildRef)}</Th>
                <Th>{t(($) => $.runs.columns.created)}</Th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run) => (
                <RunRow key={run.id} run={run} href={paths.testRunDetail(run.id)} />
              ))}
            </tbody>
          </table>
        )}
        {truncated ? (
          <p className="px-3 py-2 text-caption text-muted-foreground">
            {t(($) => $.runs.truncated, { count: RUN_PAGE_LIMIT })}
          </p>
        ) : null}
      </div>
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return <th className="px-3 py-2 text-left font-normal">{children}</th>;
}

function runStatusVariant(status: string): "secondary" | "outline" | "destructive" {
  if (status === "completed") return "secondary";
  if (status === "aborted" || status === "blocked") return "destructive";
  return "outline";
}

function RunRow({ run, href }: { run: TestRun; href: string }) {
  const { t } = useT("testing");

  return (
    <tr className="border-b border-border hover:bg-accent">
      <td className="max-w-xs px-3 py-2">
        <AppLink href={href} className="block truncate font-medium" title={run.title}>
          {run.title}
        </AppLink>
      </td>
      <td className="px-3 py-2">
        <Badge variant={runStatusVariant(run.status)}>
          {t(($) => $.run.status[run.status as keyof typeof $.run.status]) ?? run.status}
        </Badge>
      </td>
      <td className="px-3 py-2 text-caption text-muted-foreground">
        {run.executor_type === "agent"
          ? t(($) => $.runs.executor.agent)
          : t(($) => $.runs.executor.member)}
      </td>
      <td className="px-3 py-2 text-caption text-muted-foreground">{run.environment || "—"}</td>
      <td className="px-3 py-2 text-caption text-muted-foreground">{run.build_ref || "—"}</td>
      <td className="px-3 py-2 text-caption text-muted-foreground tabular-nums">
        {run.created_at.slice(0, 10)}
      </td>
    </tr>
  );
}
