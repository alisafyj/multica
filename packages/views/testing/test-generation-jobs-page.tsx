"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Sparkles } from "lucide-react";
import { toast } from "sonner";
import { projectListOptions } from "@multica/core/projects/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  testGenerationJobListOptions,
  useCreateTestGenerationJob,
  useTestCaseViewStore,
} from "@multica/core/testing";
import type { TestGenerationJob } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { NativeSelect } from "@multica/ui/components/ui/native-select";
import { PageHeader } from "../layout/page-header";
import { AppLink, useNavigation } from "../navigation";
import { useT } from "../i18n";
import { TestsTabs } from "./components/tests-tabs";
import { resolveSelectedProjectId } from "./project-selection";

/**
 * Past and in-flight AI generation runs for the selected project.
 *
 * The job detail page was only ever reachable in the seconds after starting a
 * job, so a run whose proposals were not reviewed on the spot was lost. This is
 * the way back to it.
 */
export function TestGenerationJobsPage() {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();

  const projectId = useTestCaseViewStore((state) => state.projectId);
  const setProjectId = useTestCaseViewStore((state) => state.setProjectId);
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const selectedProjectId = resolveSelectedProjectId(projects, projectId);

  const { data: jobs = [], isLoading } = useQuery({
    ...testGenerationJobListOptions(wsId, { projectId: selectedProjectId }),
    enabled: selectedProjectId.length > 0,
  });

  const createGenerationJob = useCreateTestGenerationJob();
  const [isStarting, setIsStarting] = useState(false);

  async function startGeneration() {
    if (selectedProjectId.length === 0) return;
    setIsStarting(true);
    try {
      const job = await createGenerationJob.mutateAsync({ project_id: selectedProjectId });
      toast.success(t(($) => $.toast.generationStarted));
      navigation.push(paths.testGenerationJobDetail(job.id));
    } catch {
      toast.error(t(($) => $.toast.generationFailed));
    } finally {
      setIsStarting(false);
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <Sparkles className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-body font-medium">{t(($) => $.jobs.title)}</h1>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <NativeSelect
            className="h-8 w-40 text-caption"
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
          <Button
            size="sm"
            disabled={selectedProjectId.length === 0 || isStarting}
            onClick={() => void startGeneration()}
          >
            <Sparkles className="size-4" />
            {isStarting ? t(($) => $.page.generating) : t(($) => $.page.generate)}
          </Button>
        </div>
      </PageHeader>

      <TestsTabs active="jobs" />

      <div className="min-h-0 flex-1 overflow-auto">
        {isLoading ? null : jobs.length === 0 ? (
          <div className="flex flex-col items-center gap-1 p-12 text-center">
            <p className="text-body font-medium">{t(($) => $.jobs.empty)}</p>
            <p className="text-caption text-muted-foreground">{t(($) => $.jobs.emptyHint)}</p>
          </div>
        ) : (
          <table className="w-full text-body">
            <thead>
              <tr className="border-b border-border text-caption text-muted-foreground">
                <Th>{t(($) => $.jobs.columns.job)}</Th>
                <Th>{t(($) => $.jobs.columns.status)}</Th>
                <Th>{t(($) => $.jobs.columns.created)}</Th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job) => (
                <JobRow
                  key={job.id}
                  job={job}
                  href={paths.testGenerationJobDetail(job.id)}
                />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return <th className="px-3 py-2 text-left font-normal">{children}</th>;
}

function jobStatusVariant(status: string): "secondary" | "outline" | "destructive" {
  if (status === "completed") return "secondary";
  if (status === "failed" || status === "cancelled") return "destructive";
  return "outline";
}

function JobRow({ job, href }: { job: TestGenerationJob; href: string }) {
  const { t } = useT("testing");

  return (
    <tr className="border-b border-border hover:bg-accent">
      <td className="max-w-md px-3 py-2">
        <AppLink href={href} className="block truncate font-medium">
          {t(($) => $.job.title)}
        </AppLink>
        {job.error ? (
          <p className="mt-0.5 truncate text-caption text-destructive" title={job.error}>
            {job.error}
          </p>
        ) : null}
      </td>
      <td className="px-3 py-2">
        <Badge variant={jobStatusVariant(job.status)}>
          {t(($) => $.job.jobStatus[job.status as keyof typeof $.job.jobStatus]) ?? job.status}
        </Badge>
      </td>
      <td className="px-3 py-2 text-caption text-muted-foreground tabular-nums">
        {job.created_at.slice(0, 10)}
      </td>
    </tr>
  );
}
