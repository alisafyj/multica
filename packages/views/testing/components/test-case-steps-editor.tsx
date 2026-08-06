"use client";

import { Plus, X } from "lucide-react";
import type { TestCaseStep } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { NativeSelect } from "@multica/ui/components/ui/native-select";
import { useT } from "../../i18n";
import { normalizeStepIndexes } from "../case-summary";

interface TestCaseStepsEditorProps {
  value: TestCaseStep[];
  onChange: (next: TestCaseStep[]) => void;
  /** Aliases from the case's linked repositories; empty on single-repo cases. */
  repoAliases: string[];
  disabled?: boolean;
}

/**
 * Controlled structured-step editor. Steps are rows, not markdown, because an
 * agent executes them by index. Deleting a row renumbers the rest so the index
 * always reads as the running order.
 */
export function TestCaseStepsEditor({
  value,
  onChange,
  repoAliases,
  disabled = false,
}: TestCaseStepsEditorProps) {
  const { t } = useT("testing");

  function patchStep(position: number, patch: Partial<TestCaseStep>) {
    onChange(value.map((step, index) => (index === position ? { ...step, ...patch } : step)));
  }

  function removeStep(position: number) {
    onChange(normalizeStepIndexes(value.filter((_step, index) => index !== position)));
  }

  function addStep() {
    onChange(normalizeStepIndexes([...value, { index: 0, action: "", expected: "" }]));
  }

  return (
    <div className="flex flex-col gap-2">
      {value.length === 0 ? (
        <p className="text-caption text-muted-foreground">{t(($) => $.steps.empty)}</p>
      ) : null}

      {value.map((step, position) => (
        <div
          key={position}
          className="flex items-start gap-2 rounded-md border border-border p-2"
        >
          <span className="mt-2 w-6 shrink-0 text-caption text-muted-foreground tabular-nums">
            {step.index}
          </span>
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <Input
              value={step.action}
              disabled={disabled}
              aria-label={t(($) => $.steps.action)}
              placeholder={t(($) => $.steps.actionPlaceholder)}
              onChange={(event) => patchStep(position, { action: event.target.value })}
            />
            <Input
              value={step.expected}
              disabled={disabled}
              aria-label={t(($) => $.steps.expected)}
              placeholder={t(($) => $.steps.expectedPlaceholder)}
              onChange={(event) => patchStep(position, { expected: event.target.value })}
            />
            {repoAliases.length > 0 ? (
              <NativeSelect
                value={step.repo ?? ""}
                disabled={disabled}
                aria-label={t(($) => $.steps.repo)}
                onChange={(event) =>
                  patchStep(position, { repo: event.target.value || undefined })
                }
              >
                <option value="">{t(($) => $.steps.noRepo)}</option>
                {repoAliases.map((alias) => (
                  <option key={alias} value={alias}>
                    {alias}
                  </option>
                ))}
              </NativeSelect>
            ) : null}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            disabled={disabled}
            aria-label={t(($) => $.steps.remove)}
            onClick={() => removeStep(position)}
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
        disabled={disabled}
        onClick={addStep}
      >
        <Plus className="size-4" />
        {t(($) => $.steps.add)}
      </Button>
    </div>
  );
}
