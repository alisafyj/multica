"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ClipboardList, Play, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  testPlanDetailOptions,
  testPlanCasesOptions,
  useCreateTestRun,
  useRemoveTestPlanCase,
} from "@multica/core/testing";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { BreadcrumbHeader } from "../layout/breadcrumb-header";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";

export function TestPlanDetail({ planId }: { planId: string }) {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();

  const [runTitle, setRunTitle] = useState("");
  const [environment, setEnvironment] = useState("");
  const [buildRef, setBuildRef] = useState("");
  const [isCreatingRun, setIsCreatingRun] = useState(false);

  const {
    data: plan,
    isLoading,
    error,
    refetch,
  } = useQuery(testPlanDetailOptions(wsId, planId));

  const { data: cases = [] } = useQuery(testPlanCasesOptions(wsId, planId));

  const createRun = useCreateTestRun();
  const removeCase = useRemoveTestPlanCase();

  function planStatusVariant(status: string): "secondary" | "outline" | "destructive" {
    if (status === "active") return "secondary";
    if (status === "archived") return "destructive";
    return "outline";
  }

  async function handleCreateRun() {
    if (!plan) return;
    const title = runTitle.trim() || plan.title;
    setIsCreatingRun(true);
    try {
      const run = await createRun.mutateAsync({
        plan_id: planId,
        title,
        environment: environment.trim() || undefined,
        build_ref: buildRef.trim() || undefined,
      });
      toast.success(t(($) => $.toast.runCreated));
      navigation.push(paths.testRunDetail(run.id));
    } catch {
      toast.error(t(($) => $.toast.runCreateFailed));
    } finally {
      setIsCreatingRun(false);
    }
  }

  async function handleRemoveCase(caseId: string) {
    try {
      await removeCase.mutateAsync({ planId, caseId });
    } catch {
      // silent — optimistic update already rolled back
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-muted/20">
      <BreadcrumbHeader
        segments={[{ href: paths.testPlans(), label: t(($) => $.plans.title) }]}
        leaf={
          <span className="truncate font-medium">
            {plan?.title ?? "…"}
          </span>
        }
        actions={
          <Button
            size="sm"
            variant="outline"
            onClick={() => navigation.push(paths.testPlans())}
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            {t(($) => $.plans.detail.back)}
          </Button>
        }
      />

      {isLoading ? (
        <div className="grid gap-4 p-4 lg:grid-cols-[1fr_300px]">
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
        </div>
      ) : error || !plan ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
          <p className="text-body font-medium">{t(($) => $.run.error)}</p>
          <Button size="sm" variant="outline" onClick={() => void refetch()}>
            {t(($) => $.run.retry)}
          </Button>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto p-4">
          <div className="grid gap-4 lg:grid-cols-[1fr_300px]">
            {/* Cases list */}
            <section className="rounded-lg border bg-background">
              <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
                <div className="flex items-center gap-2">
                  <ClipboardList className="h-4 w-4 text-muted-foreground" />
                  <span className="text-body font-medium">
                    {t(($) => $.plans.detail.cases)}
                  </span>
                  {cases.length > 0 ? (
                    <span className="text-caption text-muted-foreground">
                      {t(($) => $.plans.detail.caseCount, { count: cases.length })}
                    </span>
                  ) : null}
                </div>
                <Badge variant={planStatusVariant(plan.status)}>
                  {t(($) => $.plans.status[plan.status as keyof typeof $.plans.status]) ?? plan.status}
                </Badge>
              </div>

              {cases.length === 0 ? (
                <div className="flex flex-col items-center gap-1 p-8 text-center">
                  <p className="text-body font-medium">
                    {t(($) => $.plans.detail.empty)}
                  </p>
                  <p className="text-caption text-muted-foreground">
                    {t(($) => $.plans.detail.emptyHint)}
                  </p>
                </div>
              ) : (
                <div className="divide-y divide-border">
                  {cases.map((planCase, index) => (
                    <div
                      key={planCase.test_case_id}
                      className="flex items-center gap-3 px-4 py-2"
                    >
                      <span className="w-6 shrink-0 text-caption text-muted-foreground tabular-nums">
                        {index + 1}
                      </span>
                      <span className="min-w-0 flex-1 truncate text-body">
                        {planCase.test_case_id}
                      </span>
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-7 shrink-0 px-2 text-muted-foreground hover:text-destructive"
                        aria-label={t(($) => $.plans.detail.remove)}
                        onClick={() => void handleRemoveCase(planCase.test_case_id)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </section>

            {/* Create run panel */}
            <aside className="space-y-3">
              <section className="rounded-lg border bg-background p-4">
                <div className="flex items-center gap-2 text-body font-medium">
                  <Play className="h-4 w-4 text-muted-foreground" />
                  {t(($) => $.plans.detail.createRun)}
                </div>

                <div className="mt-4 space-y-3">
                  <div>
                    <label className="mb-1 block text-caption font-medium text-muted-foreground">
                      {t(($) => $.plans.detail.runTitle)}
                    </label>
                    <Input
                      value={runTitle}
                      onChange={(e) => setRunTitle(e.target.value)}
                      placeholder={plan.title}
                      className="h-8 text-caption"
                    />
                  </div>

                  <div>
                    <label className="mb-1 block text-caption font-medium text-muted-foreground">
                      {t(($) => $.plans.detail.environment)}
                    </label>
                    <Input
                      value={environment}
                      onChange={(e) => setEnvironment(e.target.value)}
                      placeholder="staging"
                      className="h-8 text-caption"
                    />
                  </div>

                  <div>
                    <label className="mb-1 block text-caption font-medium text-muted-foreground">
                      {t(($) => $.plans.detail.buildRef)}
                    </label>
                    <Input
                      value={buildRef}
                      onChange={(e) => setBuildRef(e.target.value)}
                      placeholder="v1.2.3"
                      className="h-8 text-caption"
                    />
                  </div>

                  <Button
                    className="w-full"
                    disabled={isCreatingRun || createRun.isPending || cases.length === 0}
                    onClick={() => void handleCreateRun()}
                  >
                    <Play className="h-3.5 w-3.5" />
                    {t(($) => $.plans.detail.createRun)}
                  </Button>
                </div>
              </section>
            </aside>
          </div>
        </div>
      )}
    </div>
  );
}
