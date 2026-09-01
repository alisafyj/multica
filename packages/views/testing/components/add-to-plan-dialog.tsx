"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  testPlanCasesOptions,
  testPlanListOptions,
  useAddTestPlanCases,
  useCreateTestPlan,
} from "@multica/core/testing";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { NativeSelect } from "@multica/ui/components/ui/native-select";
import { useT } from "../../i18n";

const NEW_PLAN = "__new__";

/**
 * Put the selected cases into a plan.
 *
 * This is the missing half of the plan lifecycle: plans could be created and
 * executed, but nothing in the UI ever called `useAddTestPlanCases`, so a
 * hand-made plan stayed empty forever and the run button stayed disabled. The
 * case library is where multi-select already lives, so this is the surface that
 * fills a plan.
 */
export function AddToPlanDialog({
  open,
  onOpenChange,
  wsId,
  projectId,
  caseIds,
  onAdded,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  wsId: string;
  projectId: string;
  caseIds: string[];
  onAdded?: () => void;
}) {
  const { t } = useT("testing");

  const { data: plans = [] } = useQuery({
    ...testPlanListOptions(wsId, { projectId }),
    enabled: open && projectId.length > 0,
  });

  // null means "the user has not picked yet", which is not the same as picking
  // the first plan: the list arrives after the dialog opens. Deriving the
  // default instead of writing it in an effect keeps a refetch of `plans` from
  // resetting a choice the user already made.
  const [picked, setPicked] = useState<string | null>(null);
  const [newTitle, setNewTitle] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (open) return;
    setPicked(null);
    setNewTitle("");
  }, [open]);

  const planId = picked ?? plans[0]?.id ?? NEW_PLAN;
  const isNew = planId === NEW_PLAN;

  // Existing members decide the append offset and are skipped rather than
  // re-sent: the endpoint upserts on (plan, case), so re-sending a case that is
  // already in the plan would silently move it to the end of the order.
  const { data: existingCases = [] } = useQuery({
    ...testPlanCasesOptions(wsId, isNew ? "" : planId),
    enabled: open && !isNew && planId.length > 0,
  });

  const createPlan = useCreateTestPlan();
  const addCases = useAddTestPlanCases();

  async function submit() {
    if (caseIds.length === 0 || projectId.length === 0) return;
    if (isNew && newTitle.trim().length === 0) return;
    setIsSubmitting(true);
    try {
      let targetId = planId;
      let offset = existingCases.length;
      let pending = caseIds;

      if (isNew) {
        const plan = await createPlan.mutateAsync({
          project_id: projectId,
          title: newTitle.trim(),
        });
        targetId = plan.id;
        offset = 0;
      } else {
        const present = new Set(existingCases.map((planCase) => planCase.test_case_id));
        pending = caseIds.filter((id) => !present.has(id));
      }

      if (pending.length === 0) {
        toast.success(t(($) => $.toast.planCasesAlreadyAdded));
        onOpenChange(false);
        return;
      }

      await addCases.mutateAsync({
        planId: targetId,
        data: {
          cases: pending.map((id, index) => ({
            test_case_id: id,
            position: offset + index,
          })),
        },
      });
      toast.success(t(($) => $.toast.planCasesAdded, { count: pending.length }));
      onAdded?.();
      onOpenChange(false);
    } catch (err) {
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : t(($) => $.toast.planCasesAddFailed),
      );
    } finally {
      setIsSubmitting(false);
    }
  }

  const busy = isSubmitting || createPlan.isPending || addCases.isPending;

  return (
    <Dialog open={open} onOpenChange={(next) => (busy ? undefined : onOpenChange(next))}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(($) => $.addToPlan.title)}</DialogTitle>
          <DialogDescription>
            {t(($) => $.addToPlan.description, { count: caseIds.length })}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="flex flex-col gap-1">
            <Label htmlFor="add-to-plan-target">{t(($) => $.addToPlan.plan)}</Label>
            <NativeSelect
              id="add-to-plan-target"
              value={planId}
              disabled={busy}
              onChange={(event) => setPicked(event.target.value)}
            >
              {plans.map((plan) => (
                <option key={plan.id} value={plan.id}>
                  {plan.title}
                </option>
              ))}
              <option value={NEW_PLAN}>{t(($) => $.addToPlan.newPlan)}</option>
            </NativeSelect>
          </div>

          {isNew ? (
            <div className="flex flex-col gap-1">
              <Label htmlFor="add-to-plan-title">{t(($) => $.plans.createTitle)}</Label>
              <Input
                id="add-to-plan-title"
                autoFocus
                value={newTitle}
                disabled={busy}
                placeholder={t(($) => $.plans.createTitle)}
                onChange={(event) => setNewTitle(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") void submit();
                }}
              />
            </div>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="ghost" size="sm" disabled={busy} onClick={() => onOpenChange(false)}>
            {t(($) => $.actions.cancel)}
          </Button>
          <Button
            size="sm"
            disabled={busy || (isNew && newTitle.trim().length === 0)}
            onClick={() => void submit()}
          >
            {t(($) => $.addToPlan.confirm)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
