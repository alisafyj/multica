"use client";

import { cn } from "@multica/ui/lib/utils";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";

export interface DesignFilterSelectOption {
  value: string;
  label: string;
  count: number;
}

/**
 * A single-pick facet as a dropdown: the "all" row first, then every value
 * with how many entries it holds. Used where the facet has too many values
 * for a row of pills — the official design-system catalogue carries twenty
 * categories, which as pills pushed the list they filter below the fold.
 *
 * Counts ride inside the option text so the trigger and the list read the
 * same way ("AI 与大模型 15"), and so a screen reader announces the count
 * with the label.
 */
export function DesignFilterSelect({
  label,
  value,
  allValue,
  allLabel,
  allCount,
  options,
  onChange,
  className,
}: {
  /** Accessible name of the control, e.g. "官方设计体系分类". */
  label: string;
  value: string;
  allValue: string;
  allLabel: string;
  allCount: number;
  options: DesignFilterSelectOption[];
  onChange: (value: string) => void;
  className?: string;
}) {
  const items: DesignFilterSelectOption[] = [{ value: allValue, label: allLabel, count: allCount }, ...options];
  const current = items.find((item) => item.value === value) ?? items[0];
  return (
    <Select
      items={items.map((item) => ({ value: item.value, label: item.label }))}
      value={current?.value ?? allValue}
      onValueChange={(next) => onChange(typeof next === "string" ? next : allValue)}
    >
      <SelectTrigger size="sm" aria-label={label} className={cn("w-full", className)}>
        <SelectValue>
          {() => (
            <>
              <span className="truncate">{current?.label ?? allLabel}</span>
              <span className="tabular-nums text-muted-foreground">{current?.count ?? allCount}</span>
            </>
          )}
        </SelectValue>
      </SelectTrigger>
      <SelectContent align="start">
        {items.map((item) => (
          <SelectItem key={item.value || "unlabelled"} value={item.value}>
            <span className="truncate">{item.label}</span>
            <span className="tabular-nums text-muted-foreground">{item.count}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
