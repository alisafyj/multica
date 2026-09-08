"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Loader2 } from "lucide-react";
import type { PendingInput, PendingInputAnswer } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@multica/ui/components/ui/radio-group";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

interface PendingInputFormProps {
  pendingInput: PendingInput;
  onAnswer: (answers: Record<string, PendingInputAnswer>) => Promise<void>;
}

interface DraftAnswer {
  selected: string[];
  other: string;
}

function initialDraft(pendingInput: PendingInput): Record<string, DraftAnswer> {
  return Object.fromEntries(
    pendingInput.questions.map((question) => {
      const stored = pendingInput.answers?.[question.id]?.answers ?? [];
      if (question.options.length === 0) {
        return [question.id, { selected: [], other: stored.join("\n") }];
      }
      const labels = new Set(question.options.map((option) => option.label));
      return [question.id, {
        selected: stored.filter((answer) => labels.has(answer)),
        other: stored.find((answer) => !labels.has(answer)) ?? "",
      }];
    }),
  );
}

export function PendingInputForm({ pendingInput, onAnswer }: PendingInputFormProps) {
  const { t } = useT("issues");
  const [draft, setDraft] = useState(() => initialDraft(pendingInput));
  const [submitting, setSubmitting] = useState(false);
  const [failed, setFailed] = useState(false);
  const submittingRef = useRef(false);
  const supported = pendingInput.version === 1;
  const open = pendingInput.state === "open" && supported;

  useEffect(() => {
    if (pendingInput.answers) setDraft(initialDraft(pendingInput));
  }, [pendingInput]);

  const prepared = useMemo(() => {
    let valid = pendingInput.questions.length > 0 && pendingInput.questions.length <= 3;
    const answers = Object.fromEntries(pendingInput.questions.map((question) => {
      const value = draft[question.id] ?? { selected: [], other: "" };
      const rawItems = question.options.length === 0
        ? [value.other.trim()].filter(Boolean)
        : [...value.selected, value.other.trim()].filter(Boolean);
      const items = [...new Set(rawItems)];
      if (items.length === 0 || items.length > 8) valid = false;
      return [question.id, { answers: items }];
    }));
    return { answers, valid };
  }, [draft, pendingInput.questions]);

  const complete = supported && prepared.valid;

  const updateDraft = (questionId: string, update: (current: DraftAnswer) => DraftAnswer) => {
    setFailed(false);
    setDraft((current) => ({
      ...current,
      [questionId]: update(current[questionId] ?? { selected: [], other: "" }),
    }));
  };

  const submit = async () => {
    if (!open || !complete || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    setFailed(false);
    try {
      await onAnswer(prepared.answers);
    } catch {
      setFailed(true);
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const stateLabel = !supported
    ? t(($) => $.pending_input.unsupported_version)
    : pendingInput.state === "answered"
    ? t(($) => $.pending_input.answered)
    : pendingInput.state === "cancelled"
      ? t(($) => $.pending_input.cancelled)
      : pendingInput.state === "expired"
        ? t(($) => $.pending_input.expired)
        : t(($) => $.pending_input.waiting);

  return (
    <div className="mt-3 ml-10 max-md:ml-0 border-l-2 border-brand/40 pl-3">
      <div className="mb-2 flex items-center justify-between gap-3">
        <span className="text-caption font-medium text-foreground">
          {t(($) => $.pending_input.title)}
        </span>
        <span className={cn("text-caption", open ? "text-brand" : "text-muted-foreground")}>
          {stateLabel}
        </span>
      </div>

      <div className="space-y-3">
        {pendingInput.questions.map((question) => {
          const value = draft[question.id] ?? { selected: [], other: "" };
          return (
            <fieldset key={question.id} disabled={!open || submitting} className="space-y-1.5">
              <legend className="min-w-0 break-words text-body font-medium">{question.header}</legend>
              <p className="min-w-0 break-words text-body text-muted-foreground">{question.question}</p>

              {question.options.length === 0 ? (
                <Textarea
                  value={value.other}
                  onChange={(event) => updateDraft(question.id, (current) => ({
                    ...current,
                    other: event.target.value,
                  }))}
                  aria-label={question.question}
                  className="min-h-20"
                  maxLength={2000}
                />
              ) : question.multi_select ? (
                <div className="space-y-1.5">
                  {question.options.map((option) => (
                    <label key={option.label} className="flex min-w-0 items-start gap-2 text-body">
                      <Checkbox
                        checked={value.selected.includes(option.label)}
                        onCheckedChange={(checked) => updateDraft(question.id, (current) => ({
                          ...current,
                          selected: checked
                            ? [...current.selected, option.label]
                            : current.selected.filter((item) => item !== option.label),
                        }))}
                      />
                      <span className="min-w-0">
                        <span className="block break-words text-foreground">{option.label}</span>
                        {option.description && (
                          <span className="block break-words text-caption text-muted-foreground">
                            {option.description}
                          </span>
                        )}
                      </span>
                    </label>
                  ))}
                </div>
              ) : (
                <RadioGroup
                  value={value.selected[0] ?? ""}
                  onValueChange={(selected) => updateDraft(question.id, () => ({
                    selected: [selected],
                    other: "",
                  }))}
                >
                  {question.options.map((option) => (
                    <label key={option.label} className="flex min-w-0 items-start gap-2 text-body">
                      <RadioGroupItem value={option.label} />
                      <span className="min-w-0">
                        <span className="block break-words text-foreground">{option.label}</span>
                        {option.description && (
                          <span className="block break-words text-caption text-muted-foreground">
                            {option.description}
                          </span>
                        )}
                      </span>
                    </label>
                  ))}
                </RadioGroup>
              )}

              {question.options.length > 0 && question.allow_other && (
                <Input
                  value={value.other}
                  onChange={(event) => updateDraft(question.id, (current) => ({
                    selected: question.multi_select ? current.selected : [],
                    other: event.target.value,
                  }))}
                  placeholder={t(($) => $.pending_input.other_placeholder)}
                  aria-label={t(($) => $.pending_input.other_placeholder)}
                  maxLength={2000}
                />
              )}
            </fieldset>
          );
        })}
      </div>

      {failed && (
        <p role="alert" className="mt-2 text-caption text-destructive">
          {t(($) => $.pending_input.answer_failed)}
        </p>
      )}

      {open && (
        <div className="mt-3 flex justify-end">
          <Button
            type="button"
            size="sm"
            onClick={() => void submit()}
            disabled={!complete || submitting}
          >
            {submitting && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            {t(($) => $.pending_input.send)}
          </Button>
        </div>
      )}
    </div>
  );
}
