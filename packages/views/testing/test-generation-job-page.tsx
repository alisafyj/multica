"use client";

import { useEffect, useMemo, useState } from "react";
import {
  ArrowLeft,
  Bot,
  CheckCircle2,
  ClipboardList,
  Save,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { agentTaskSnapshotOptions } from "@multica/core/agents/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { TestGenerationPlanPayload } from "@multica/core/types";
import {
  testGenerationJobDetailOptions,
  testGenerationPlanOptions,
  useApproveTestGenerationPlan,
  useDispatchTestGenerationJob,
  useGenerateTestGenerationPlan,
  useUpdateTestGenerationPlan,
} from "@multica/core/testing";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { BreadcrumbHeader } from "../layout/breadcrumb-header";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";
import {
  planInstructions,
  planModules,
  planRepos,
  validatePlanJson,
} from "./test-generation-job-logic";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function planStatusVariant(status: string): "secondary" | "outline" | "destructive" {
  if (status === "approved" || status === "dispatched") return "secondary";
  if (status === "draft") return "outline";
  return "destructive";
}

function jobStatusVariant(status: string): "secondary" | "outline" | "destructive" {
  if (status === "completed") return "secondary";
  if (status === "failed" || status === "cancelled") return "destructive";
  return "outline";
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export function TestGenerationJobPage({ jobId }: { jobId: string }) {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();

  const [selectedAgentId, setSelectedAgentId] = useState("");
  const [planDraft, setPlanDraft] = useState("");
  const [reviewNotes, setReviewNotes] = useState("");

  const {
    data: job,
    isLoading,
    error,
    refetch,
  } = useQuery(testGenerationJobDetailOptions(wsId, jobId));
  const { data: plan, isError: planMissing } = useQuery(
    testGenerationPlanOptions(wsId, jobId),
  );
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: agentTasks = [] } = useQuery(agentTaskSnapshotOptions(wsId));

  const availableAgents = useMemo(
    () => agents.filter((agent) => !agent.archived_at && agent.runtime_id),
    [agents],
  );
  const dispatchAgentId = selectedAgentId || availableAgents[0]?.id || "";

  const generatePlan = useGenerateTestGenerationPlan();
  const savePlanMutation = useUpdateTestGenerationPlan();
  const approvePlanMutation = useApproveTestGenerationPlan();
  const dispatchJob = useDispatchTestGenerationJob();

  useEffect(() => {
    if (!plan) return;
    setPlanDraft(JSON.stringify(plan.plan, null, 2));
    setReviewNotes(plan.review_notes ?? "");
  }, [plan]);

  const hasApprovedPlan =
    plan?.status === "approved" || plan?.status === "dispatched";
  const canEditPlan = plan?.status === "draft";
  const planDraftError = canEditPlan ? validatePlanJson(planDraft) : null;
  const canApprovePlan = !!plan && canEditPlan && planDraftError === null;

  const agentTask = agentTasks.find((item) => item.id === job?.agent_task_id);
  const taskAgent = agents.find((agent) => agent.id === agentTask?.agent_id);
  const agentTaskName = job?.agent_task_id
    ? `${taskAgent?.name ?? "Agent"} · ${agentTask?.status ?? job.status}`
    : t(($) => $.job.meta.notDispatched);

  const currentPlanRepos = plan ? planRepos(plan.plan) : [];
  const currentPlanModules = plan ? planModules(plan.plan) : [];
  const currentPlanInstructions = plan ? planInstructions(plan.plan) : "";

  function handleGeneratePlan() {
    generatePlan.mutate(jobId, {
      onSuccess: (generated) => {
        setPlanDraft(JSON.stringify(generated.plan, null, 2));
        setReviewNotes(generated.review_notes ?? "");
        toast.success(t(($) => $.toast.planGenerated));
      },
      onError: (err) =>
        toast.error(
          err instanceof Error ? err.message : t(($) => $.toast.planFailed),
        ),
    });
  }

  function handleSavePlan() {
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(planDraft) as Record<string, unknown>;
    } catch {
      toast.error(t(($) => $.toast.planSaveFailed));
      return;
    }
    savePlanMutation.mutate(
      { jobId, data: { plan: parsed as unknown as TestGenerationPlanPayload, review_notes: reviewNotes } },
      {
        onSuccess: () => toast.success(t(($) => $.toast.planSaved)),
        onError: (err) =>
          toast.error(
            err instanceof Error ? err.message : t(($) => $.toast.planSaveFailed),
          ),
      },
    );
  }

  function handleApprovePlan() {
    approvePlanMutation.mutate(jobId, {
      onSuccess: () => toast.success(t(($) => $.toast.planApproved)),
      onError: (err) =>
        toast.error(
          err instanceof Error
            ? err.message
            : t(($) => $.toast.planApproveFailed),
        ),
    });
  }

  function handleDispatch() {
    if (!dispatchAgentId) return;
    dispatchJob.mutate(
      { id: jobId, data: { agent_id: dispatchAgentId } },
      {
        onSuccess: () => toast.success(t(($) => $.toast.dispatched)),
        onError: (err) =>
          toast.error(
            err instanceof Error
              ? err.message
              : t(($) => $.toast.dispatchFailed),
          ),
      },
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-muted/20">
      <BreadcrumbHeader
        segments={[{ href: paths.tests(), label: t(($) => $.page.title) }]}
        leaf={
          <span className="truncate font-medium">
            {t(($) => $.job.title)}
          </span>
        }
        actions={
          <Button
            size="sm"
            variant="outline"
            onClick={() => navigation.push(paths.tests())}
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            {t(($) => $.job.back)}
          </Button>
        }
      />

      {isLoading ? (
        <div className="grid gap-4 p-4 lg:grid-cols-[1fr_340px]">
          <Skeleton className="h-96" />
          <Skeleton className="h-96" />
        </div>
      ) : error || !job ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
          <p className="text-body font-medium">{t(($) => $.job.error)}</p>
          <Button size="sm" variant="outline" onClick={() => void refetch()}>
            {t(($) => $.job.retry)}
          </Button>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto p-4">
          <div className="grid gap-4 lg:grid-cols-[1fr_340px]">
            {/* Left column — job meta + stats */}
            <div className="space-y-4">
              <section className="rounded-lg border bg-background p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-2 text-body font-medium">
                    <ClipboardList className="h-4 w-4 text-muted-foreground" />
                    {t(($) => $.job.title)}
                  </div>
                  <Badge variant={jobStatusVariant(job.status)}>
                    {t(($) => $.job.jobStatus[job.status as keyof typeof $.job.jobStatus]) ?? job.status}
                  </Badge>
                </div>

                <div className="mt-4 grid gap-2 text-caption text-muted-foreground sm:grid-cols-2">
                  <div>
                    {t(($) => $.job.meta.status)}:{" "}
                    <span className="text-foreground">{job.status}</span>
                  </div>
                  <div>
                    {t(($) => $.job.meta.createdAt)}:{" "}
                    <span className="text-foreground">{job.created_at}</span>
                  </div>
                  <div className="sm:col-span-2">
                    {t(($) => $.job.meta.agentTask)}:{" "}
                    <span className="text-foreground">{agentTaskName}</span>
                  </div>
                </div>
              </section>
            </div>

            {/* Right column — plan, dispatch, result */}
            <aside className="space-y-4">
              {/* Plan panel */}
              <section className="rounded-lg border bg-background p-3">
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <div className="flex items-center gap-2 text-body font-medium">
                      <ClipboardList className="h-4 w-4 text-muted-foreground" />
                      {t(($) => $.job.plan.title)}
                    </div>
                  </div>
                  <Badge
                    variant={
                      plan
                        ? planStatusVariant(plan.status)
                        : planMissing
                          ? "destructive"
                          : "outline"
                    }
                  >
                    {plan
                      ? (t(($) => $.job.planStatus[plan.status as keyof typeof $.job.planStatus]) ?? plan.status)
                      : planMissing
                        ? t(($) => $.job.plan.noPlan)
                        : "…"}
                  </Badge>
                </div>

                <div className="mt-3 grid grid-cols-2 gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={
                      generatePlan.isPending ||
                      (!!plan && plan.status !== "draft")
                    }
                    onClick={handleGeneratePlan}
                  >
                    <ClipboardList className="h-3.5 w-3.5" />
                    {plan
                      ? t(($) => $.job.plan.regenerate)
                      : t(($) => $.job.plan.generate)}
                  </Button>
                  <Button
                    size="sm"
                    disabled={!canApprovePlan || approvePlanMutation.isPending}
                    onClick={handleApprovePlan}
                  >
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    {t(($) => $.job.plan.approve)}
                  </Button>
                </div>

                {plan ? (
                  <div className="mt-3 space-y-3">
                    <div className="grid gap-2 text-caption text-muted-foreground">
                      {currentPlanRepos.length > 0 ? (
                        <div>
                          {t(($) => $.job.plan.repos)}:{" "}
                          <span className="text-foreground">
                            {currentPlanRepos.join(", ")}
                          </span>
                        </div>
                      ) : null}
                      {currentPlanModules.length > 0 ? (
                        <div>
                          {t(($) => $.job.plan.modules)}:{" "}
                          <span className="text-foreground">
                            {currentPlanModules.join(", ")}
                          </span>
                        </div>
                      ) : null}
                      {currentPlanInstructions ? (
                        <div>
                          {t(($) => $.job.plan.instructions)}:{" "}
                          <span className="text-foreground line-clamp-2">
                            {currentPlanInstructions}
                          </span>
                        </div>
                      ) : null}
                      {plan.approved_at ? (
                        <div>
                          {t(($) => $.job.plan.approvedAt)}:{" "}
                          <span className="text-foreground">
                            {plan.approved_at}
                          </span>
                        </div>
                      ) : null}
                    </div>

                    <div>
                      <label className="mb-1 block text-caption font-medium text-muted-foreground">
                        {t(($) => $.job.plan.reviewNotes)}
                      </label>
                      <Input
                        value={reviewNotes}
                        onChange={(event) =>
                          setReviewNotes(event.target.value)
                        }
                        disabled={!canEditPlan}
                        className="h-8 text-caption"
                        placeholder={t(
                          ($) => $.job.plan.reviewNotesPlaceholder,
                        )}
                      />
                    </div>

                    <details className="rounded-md border" open={false}>
                      <summary className="flex cursor-pointer list-none items-center justify-between px-3 py-2 text-caption font-medium hover:bg-muted/50">
                        <span>{t(($) => $.job.plan.rawJson)}</span>
                      </summary>
                      <div className="max-h-60 overflow-auto border-t p-2">
                        <pre className="text-caption leading-relaxed text-muted-foreground">
                          {JSON.stringify(plan.plan, null, 2)}
                        </pre>
                      </div>
                    </details>

                    <details className="rounded-md border" open={canEditPlan}>
                      <summary className="flex cursor-pointer list-none items-center justify-between px-3 py-2 text-caption font-medium hover:bg-muted/50">
                        <span>{t(($) => $.job.plan.editJson)}</span>
                        <span className="text-muted-foreground">
                          {t(($) => $.job.plan.editJsonHint)}
                        </span>
                      </summary>
                      <div className="border-t p-2">
                        <Textarea
                          value={planDraft}
                          onChange={(event) => setPlanDraft(event.target.value)}
                          disabled={!canEditPlan}
                          className="min-h-40 font-mono text-caption"
                        />
                        {planDraftError ? (
                          <p className="mt-1 text-caption text-destructive">
                            {planDraftError}
                          </p>
                        ) : null}
                      </div>
                    </details>

                    <Button
                      size="sm"
                      variant="outline"
                      className="w-full"
                      disabled={
                        !canEditPlan ||
                        savePlanMutation.isPending ||
                        planDraftError !== null
                      }
                      onClick={handleSavePlan}
                    >
                      <Save className="h-3.5 w-3.5" />
                      {t(($) => $.job.plan.save)}
                    </Button>
                  </div>
                ) : (
                  <div className="mt-3 rounded-md border border-dashed p-3 text-caption leading-relaxed text-muted-foreground">
                    {t(($) => $.job.plan.noPlanHint)}
                  </div>
                )}
              </section>

              {/* Dispatch panel */}
              <section className="rounded-lg border bg-background p-3">
                <div className="flex items-center gap-2 text-body font-medium">
                  <Bot className="h-4 w-4 text-muted-foreground" />
                  {t(($) => $.job.dispatch.title)}
                </div>
                <p className="mt-1 text-caption text-muted-foreground">
                  {t(($) => $.job.dispatch.hint)}
                </p>

                <div className="mt-3 space-y-3">
                  <div>
                    <label className="mb-1 block text-caption font-medium text-muted-foreground">
                      {t(($) => $.job.dispatch.agent)}
                    </label>
                    <select
                      value={dispatchAgentId}
                      onChange={(event) =>
                        setSelectedAgentId(event.target.value)
                      }
                      className="h-8 w-full rounded-md border bg-background px-2 text-caption"
                      disabled={!availableAgents.length}
                    >
                      {availableAgents.length ? (
                        availableAgents.map((agent) => (
                          <option key={agent.id} value={agent.id}>
                            {agent.name} · {agent.status}
                          </option>
                        ))
                      ) : (
                        <option value="">
                          {t(($) => $.job.dispatch.noAgent)}
                        </option>
                      )}
                    </select>
                  </div>

                  {!hasApprovedPlan ? (
                    <div className="rounded-md border border-amber-200 bg-amber-50 p-2 text-caption text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200">
                      {t(($) => $.job.dispatch.needsPlan)}
                    </div>
                  ) : null}

                  <Button
                    className="w-full"
                    disabled={
                      !dispatchAgentId ||
                      dispatchJob.isPending ||
                      job.status === "running" ||
                      !hasApprovedPlan
                    }
                    onClick={handleDispatch}
                  >
                    <Bot className="h-3.5 w-3.5" />
                    {job.status === "running"
                      ? t(($) => $.job.dispatch.running)
                      : job.agent_task_id
                        ? t(($) => $.job.dispatch.redispatch)
                        : dispatchJob.isPending
                          ? t(($) => $.job.dispatch.dispatching)
                          : t(($) => $.job.dispatch.button)}
                  </Button>
                </div>
              </section>

              {/* Result panel — shown once job has a result */}
              {job.result && Object.keys(job.result).length > 0 ? (
                <section className="rounded-lg border bg-background p-3">
                  <div className="text-body font-medium">
                    {t(($) => $.job.result.title)}
                  </div>
                  <div className="mt-3 text-caption text-muted-foreground">
                    <pre className="max-h-48 overflow-auto whitespace-pre-wrap rounded-md bg-muted p-2 text-caption leading-relaxed">
                      {JSON.stringify(job.result, null, 2)}
                    </pre>
                  </div>
                </section>
              ) : null}
            </aside>
          </div>
        </div>
      )}
    </div>
  );
}
