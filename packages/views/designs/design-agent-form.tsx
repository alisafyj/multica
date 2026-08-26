"use client";

import { useState } from "react";
import { Check, ClipboardList, ListChecks, ShieldCheck, Sparkles } from "lucide-react";
import {
  formatAgentFormAnswers,
  type AgentCard,
  type AgentFormDirectionCard,
  type AgentFormQuestion,
  type AgentQuestionForm,
} from "@multica/core/designs/agent-ui";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";

/** Free-text controls that map straight onto an `<input type>`. */
const NATIVE_INPUT_TYPES: Record<string, string> = {
  text: "text",
  number: "number",
  date: "date",
  time: "time",
  "datetime-local": "datetime-local",
  color: "color",
  url: "url",
  email: "email",
  tel: "tel",
};

function isAnswered(value: string | string[] | undefined): boolean {
  if (Array.isArray(value)) return value.length > 0;
  return typeof value === "string" && value.trim().length > 0;
}

function DirectionCard({
  card,
  selected,
  onSelect,
}: {
  card: AgentFormDirectionCard;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={cn(
        "flex min-w-0 cursor-pointer flex-col gap-2 rounded-lg border p-3 text-left transition-colors",
        // Selection lives on border and weight, dimensions hover does not
        // touch, so a picked card cannot read as merely hovered.
        selected ? "border-primary bg-accent" : "hover:bg-accent/50",
      )}
    >
      <div className="flex min-w-0 items-center justify-between gap-2">
        <span className={cn("min-w-0 truncate text-body", selected && "font-medium text-primary")}>
          {card.label}
        </span>
        {selected ? <Check className="size-3.5 shrink-0 text-primary" /> : null}
      </div>
      {/* The point of this control: the direction is judged by looking at it. */}
      {card.palette.length > 0 ? (
        <span className="flex gap-1">
          {card.palette.slice(0, 6).map((swatch: string, index: number) => (
            <span
              key={`${swatch}-${index}`}
              className="size-4 rounded-full border"
              style={{ backgroundColor: swatch }}
            />
          ))}
        </span>
      ) : null}
      {card.displayFont || card.bodyFont ? (
        <span className="flex items-baseline gap-2">
          {card.displayFont ? (
            <span className="text-title" style={{ fontFamily: card.displayFont }}>Aa</span>
          ) : null}
          {card.bodyFont ? (
            <span className="min-w-0 truncate text-caption text-muted-foreground" style={{ fontFamily: card.bodyFont }}>
              正文样式
            </span>
          ) : null}
        </span>
      ) : null}
      {card.mood ? <span className="text-caption text-muted-foreground">{card.mood}</span> : null}
      {card.references.length > 0 ? (
        <span className="truncate text-micro text-muted-foreground">
          参考：{card.references.join(" · ")}
        </span>
      ) : null}
    </button>
  );
}

function QuestionField({
  question,
  value,
  onChange,
  disabled,
}: {
  question: AgentFormQuestion;
  value: string | string[] | undefined;
  onChange: (next: string | string[]) => void;
  disabled: boolean;
}) {
  // A finite-choice question tracks its custom entry separately: switching
  // back to a listed option must not destroy what the user typed.
  const [custom, setCustom] = useState("");
  const single = typeof value === "string" ? value : "";
  const many = Array.isArray(value) ? value : [];
  const listed = question.options.map((option) => option.value);
  // Custom mode is explicit state, not inferred from the value being unlisted:
  // an empty custom answer is exactly the state right after the user picks
  // "other", and inferring would hide the field they just asked for.
  const [customMode, setCustomMode] = useState(false);
  const onCustom = question.allowCustom && (customMode || (single.length > 0 && !listed.includes(single)));

  if (question.type === "direction-cards" && question.cards.length > 0) {
    return (
      <div role="radiogroup" aria-label={question.label} className="grid gap-2 sm:grid-cols-2">
        {question.cards.map((card) => (
          <DirectionCard
            key={card.id}
            card={card}
            selected={single === card.id}
            onSelect={() => !disabled && onChange(card.id)}
          />
        ))}
      </div>
    );
  }

  if (question.type === "radio" || question.type === "select") {
    return (
      <div className="flex flex-col gap-1.5">
        <div role="radiogroup" aria-label={question.label} className="flex flex-wrap gap-1.5">
          {question.options.map((option) => (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={!onCustom && single === option.value}
              disabled={disabled}
              onClick={() => {
                setCustomMode(false);
                onChange(option.value);
              }}
              className={cn(
                "flex h-7 cursor-pointer items-center rounded-full border px-2.5 text-caption transition-colors disabled:cursor-not-allowed disabled:opacity-60",
                single === option.value
                  ? "border-primary bg-accent font-medium text-primary"
                  : "text-muted-foreground hover:bg-accent hover:text-foreground",
              )}
            >
              {option.label}
            </button>
          ))}
          {question.allowCustom ? (
            <button
              type="button"
              role="radio"
              aria-checked={onCustom}
              disabled={disabled}
              onClick={() => {
                setCustomMode(true);
                onChange(custom);
              }}
              className={cn(
                "flex h-7 cursor-pointer items-center rounded-full border border-dashed px-2.5 text-caption transition-colors disabled:cursor-not-allowed disabled:opacity-60",
                onCustom ? "border-primary bg-accent font-medium text-primary" : "text-muted-foreground hover:bg-accent",
              )}
            >
              {question.customLabel || "其他"}
            </button>
          ) : null}
        </div>
        {question.allowCustom && onCustom ? (
          <Input
            value={customMode ? custom : single}
            disabled={disabled}
            aria-label={`${question.label}（自定义）`}
            onChange={(event) => {
              setCustom(event.target.value);
              onChange(event.target.value);
            }}
            className="h-8 max-w-sm text-body"
          />
        ) : null}
      </div>
    );
  }

  if (question.type === "checkbox") {
    return (
      <div className="flex flex-wrap gap-1.5">
        {question.options.map((option) => {
          const picked = many.includes(option.value);
          return (
            <button
              key={option.value}
              type="button"
              role="checkbox"
              aria-checked={picked}
              disabled={disabled}
              onClick={() =>
                onChange(picked ? many.filter((item) => item !== option.value) : [...many, option.value])
              }
              className={cn(
                "flex h-7 cursor-pointer items-center gap-1 rounded-full border px-2.5 text-caption transition-colors disabled:cursor-not-allowed disabled:opacity-60",
                picked
                  ? "border-primary bg-accent font-medium text-primary"
                  : "text-muted-foreground hover:bg-accent hover:text-foreground",
              )}
            >
              {picked ? <Check className="size-3" /> : null}
              {option.label}
            </button>
          );
        })}
      </div>
    );
  }

  if (question.type === "switch") {
    const on = single === "是";
    return (
      <button
        type="button"
        role="switch"
        aria-checked={on}
        aria-label={question.label}
        disabled={disabled}
        onClick={() => onChange(on ? "否" : "是")}
        className={cn(
          "flex h-7 w-fit cursor-pointer items-center rounded-full border px-2.5 text-caption transition-colors disabled:cursor-not-allowed disabled:opacity-60",
          on ? "border-primary bg-accent font-medium text-primary" : "text-muted-foreground hover:bg-accent",
        )}
      >
        {on ? "是" : "否"}
      </button>
    );
  }

  if (question.type === "textarea") {
    return (
      <Textarea
        value={single}
        disabled={disabled}
        aria-label={question.label}
        placeholder={question.placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="min-h-20 resize-none text-body"
      />
    );
  }

  if (question.type === "range") {
    const min = question.min ?? 0;
    const max = question.max ?? 100;
    return (
      <div className="flex items-center gap-2">
        <input
          type="range"
          value={single || String(min)}
          min={min}
          max={max}
          step={question.step ?? 1}
          disabled={disabled}
          aria-label={question.label}
          onChange={(event) => onChange(event.target.value)}
          className="max-w-sm flex-1 accent-primary"
        />
        <span className="w-10 shrink-0 text-caption tabular-nums text-muted-foreground">
          {single || min}
        </span>
      </div>
    );
  }

  return (
    <Input
      type={NATIVE_INPUT_TYPES[question.type] ?? "text"}
      value={single}
      disabled={disabled}
      aria-label={question.label}
      placeholder={question.placeholder}
      {...(question.min !== null ? { min: question.min } : {})}
      {...(question.max !== null ? { max: question.max } : {})}
      {...(question.step !== null ? { step: question.step } : {})}
      onChange={(event) => onChange(event.target.value)}
      className="h-8 max-w-sm text-body"
    />
  );
}

/**
 * A form the agent asked for, rendered in place of the block it wrote.
 *
 * Submitting does not reach a running agent — our design runs are one-shot
 * tasks with no input channel, unlike Open Design's live chat session — so the
 * answers are handed to the caller as the text the agent would have received,
 * and the caller decides where they go. The workspace seeds the adjustment
 * composer with them, which is the one place a reply genuinely reaches the
 * agent: as the brief for the next turn.
 */
export function DesignAgentForm({
  form,
  onSubmit,
  submittedAnswers,
}: {
  form: AgentQuestionForm;
  /** Receives the formatted answer text. Absent renders the form read-only. */
  onSubmit?: (text: string, answers: Record<string, string | string[]>) => void;
  /** Already answered: render the picks, locked. */
  submittedAnswers?: Record<string, string | string[]>;
}) {
  const [answers, setAnswers] = useState<Record<string, string | string[]>>({});
  const locked = submittedAnswers !== undefined || onSubmit === undefined;
  const current = submittedAnswers ?? answers;
  const missing = form.questions.filter(
    (question) => question.required && !isAnswered(current[question.id]),
  );

  return (
    <section
      aria-label={form.title || "智能体提问"}
      className="flex flex-col gap-3 rounded-lg border bg-card p-3"
    >
      <div className="flex min-w-0 items-start gap-2">
        <Sparkles className="size-3.5 shrink-0 translate-y-0.5 text-primary" />
        <div className="min-w-0 flex-1">
          <p className="text-body font-medium">{form.title || "智能体想确认几件事"}</p>
          {form.description ? (
            <p className="mt-0.5 text-caption text-muted-foreground">{form.description}</p>
          ) : null}
        </div>
        {submittedAnswers ? (
          <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-micro text-muted-foreground">
            已回答
          </span>
        ) : null}
      </div>

      {form.questions.map((question) => (
        <div key={question.id} className="flex min-w-0 flex-col gap-1.5">
          <span className="text-caption font-medium">
            {question.label}
            {question.required ? <span className="ml-0.5 text-destructive">*</span> : null}
          </span>
          {question.help ? (
            <span className="text-caption text-muted-foreground">{question.help}</span>
          ) : null}
          <QuestionField
            question={question}
            value={current[question.id]}
            disabled={locked}
            onChange={(next) => setAnswers((prev) => ({ ...prev, [question.id]: next }))}
          />
        </div>
      ))}

      {locked ? null : (
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
          <p role="status" className="text-caption text-muted-foreground">
            {missing.length > 0 ? `还需回答：${missing.map((item) => item.label).join("、")}` : ""}
          </p>
          <Button
            type="button"
            size="sm"
            disabled={missing.length > 0}
            onClick={() => onSubmit?.(formatAgentFormAnswers(form, answers), answers)}
          >
            填入调整
          </Button>
        </div>
      )}
    </section>
  );
}

const CARD_ICONS: Record<AgentCard["kind"], typeof Sparkles> = {
  "task-brief": ClipboardList,
  "memory-applied": ListChecks,
  "verify-scorecard": ShieldCheck,
  "rule-proposal": Sparkles,
};

const CARD_TITLES: Record<AgentCard["kind"], string> = {
  "task-brief": "已改写的需求",
  "memory-applied": "用到的记忆",
  "verify-scorecard": "自检结果",
  "rule-proposal": "建议的规则",
};

/**
 * The display-only blocks — Open Design's `<od-card>`. They carry no input, so
 * unlike the form they need nothing from the architecture: the agent is
 * showing its work, and the card just has to render it.
 */
export function DesignAgentCard({ card }: { card: AgentCard }) {
  const Icon = CARD_ICONS[card.kind];
  return (
    <section
      aria-label={card.title || CARD_TITLES[card.kind]}
      className="flex flex-col gap-2 rounded-lg border bg-muted/30 p-3"
    >
      <div className="flex min-w-0 items-center gap-1.5 text-caption font-medium">
        <Icon className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="min-w-0 truncate">{card.title || CARD_TITLES[card.kind]}</span>
        {card.status ? (
          <span
            className={cn(
              "ml-auto shrink-0 rounded-full px-2 py-0.5 text-micro",
              card.status === "pass" ? "bg-primary/10 text-primary" : "bg-destructive/10 text-destructive",
            )}
          >
            {card.status}
          </span>
        ) : null}
      </div>
      {card.body ? (
        <p className="whitespace-pre-wrap break-words text-caption text-muted-foreground">{card.body}</p>
      ) : null}
      {card.items.length > 0 ? (
        <ul className="flex flex-col gap-0.5">
          {card.items.map((item, index) => (
            <li key={`${item}-${index}`} className="truncate text-caption text-muted-foreground">
              · {item}
            </li>
          ))}
        </ul>
      ) : null}
      {card.rows.length > 0 ? (
        <dl className="flex flex-col gap-1">
          {card.rows.map((row) => (
            <div key={row.label} className="flex min-w-0 items-baseline justify-between gap-2">
              <dt className="min-w-0 truncate text-caption text-muted-foreground">{row.label}</dt>
              <dd
                className={cn(
                  "shrink-0 text-caption",
                  row.verdict === "pass" ? "text-muted-foreground" : "text-destructive",
                )}
              >
                {row.note || row.verdict}
              </dd>
            </div>
          ))}
        </dl>
      ) : null}
    </section>
  );
}
