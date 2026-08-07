"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ClipboardList, Plus } from "lucide-react";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  testPlanListOptions,
  useCreateTestPlan,
} from "@multica/core/testing";
import type { TestPlan } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { PageHeader } from "../layout/page-header";
import { AppLink, useNavigation } from "../navigation";
import { useT } from "../i18n";

export function TestPlansPage() {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();

  const [newTitle, setNewTitle] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const { data: plans = [], isLoading } = useQuery(testPlanListOptions(wsId));

  const createPlan = useCreateTestPlan();

  async function handleCreate() {
    const title = newTitle.trim() || t(($) => $.plans.title);
    setIsCreating(true);
    try {
      const plan = await createPlan.mutateAsync({
        project_id: "",
        title,
      });
      toast.success(t(($) => $.toast.planCreated));
      navigation.push(paths.testPlanDetail(plan.id));
    } catch {
      toast.error(t(($) => $.toast.planCreateFailed));
    } finally {
      setIsCreating(false);
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <ClipboardList className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-body font-medium">
            {t(($) => $.plans.title)}
          </h1>
        </div>
        <div className="flex items-center gap-2">
          <Input
            className="h-8 w-48 text-caption"
            placeholder={t(($) => $.plans.createTitle)}
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleCreate();
            }}
          />
          <Button
            size="sm"
            disabled={isCreating || createPlan.isPending}
            onClick={() => void handleCreate()}
          >
            <Plus className="size-4" />
            {t(($) => $.plans.new)}
          </Button>
        </div>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-auto">
        {isLoading ? null : plans.length === 0 ? (
          <div className="flex flex-col items-center gap-1 p-12 text-center">
            <p className="text-body font-medium">{t(($) => $.plans.empty)}</p>
            <p className="text-caption text-muted-foreground">
              {t(($) => $.plans.emptyHint)}
            </p>
          </div>
        ) : (
          <table className="w-full text-body">
            <thead>
              <tr className="border-b border-border text-caption text-muted-foreground">
                <Th>{t(($) => $.plans.columns.title)}</Th>
                <Th>{t(($) => $.plans.columns.status)}</Th>
                <Th>{t(($) => $.plans.columns.created)}</Th>
              </tr>
            </thead>
            <tbody>
              {plans.map((plan) => (
                <PlanRow
                  key={plan.id}
                  plan={plan}
                  href={paths.testPlanDetail(plan.id)}
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

function planStatusVariant(status: string): "secondary" | "outline" | "destructive" {
  if (status === "active") return "secondary";
  if (status === "archived") return "destructive";
  return "outline";
}

function PlanRow({ plan, href }: { plan: TestPlan; href: string }) {
  const { t } = useT("testing");
  const navigation = useNavigation();

  return (
    <tr
      className="cursor-pointer border-b border-border hover:bg-accent"
      onClick={() => navigation.push(href)}
    >
      <td className="px-3 py-2">
        <AppLink href={href} className="font-medium">
          {plan.title}
        </AppLink>
        {plan.description ? (
          <p className="mt-0.5 truncate text-caption text-muted-foreground">
            {plan.description}
          </p>
        ) : null}
      </td>
      <td className="px-3 py-2">
        <Badge variant={planStatusVariant(plan.status)}>
          {t(($) => $.plans.status[plan.status as keyof typeof $.plans.status]) ?? plan.status}
        </Badge>
      </td>
      <td className="px-3 py-2 text-caption text-muted-foreground">
        {plan.created_at.slice(0, 10)}
      </td>
    </tr>
  );
}
