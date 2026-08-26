"use client";

import { Sparkles } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";

/**
 * A follow-up the user is likely to want next, as a ready-made instruction.
 *
 * Deliberately NOT an agent capability: picking one seeds the adjustment box
 * with editable text, so the user always sees and owns what gets sent. Open
 * Design's toolbox dispatches its own actions; here the next turn is an
 * ordinary adjustment, which keeps one path — and one audit trail — for every
 * change a document goes through.
 */
export interface NextStep {
  id: string;
  label: string;
  instruction: string;
}

/**
 * The passes a page design most often needs after a first draft, in the order
 * a designer would run them: structure is settled, so what is left is craft.
 * Kept short on purpose — a long menu is a worse prompt than a blank box.
 */
export const NEXT_STEPS: NextStep[] = [
  {
    id: "polish",
    label: "设计润色",
    instruction: "在不改变信息架构和页面数量的前提下，统一排版节奏、间距与对比度，让层级更清晰。",
  },
  {
    id: "states",
    label: "补齐交互状态",
    instruction: "为可交互元素补齐悬停、按下、禁用、加载与错误状态，并说明每种状态的触发条件。",
  },
  {
    id: "responsive",
    label: "响应式检查",
    instruction: "检查 360px 到 1920px 的表现，消除横向溢出与折行异常，必要时调整断点下的布局。",
  },
  {
    id: "a11y",
    label: "可访问性",
    instruction: "检查文本对比度、焦点顺序与键盘可达性，为图标按钮补上可读名称。",
  },
  // Last, and only here: the panel is the one follow-up that adds tooling
  // rather than refining the design. It used to hold a permanent chip above
  // the composer, which spent a fixed slot on a niche request and offered it
  // from the moment any revision existed. Now that every design is already
  // token-driven, this asks for the control surface alone.
  {
    id: "tweaks",
    label: "调整面板",
    instruction:
      "为原型添加一个调整面板（tweaks）：一个默认收起、可从浮动标签打开的包内侧栏控件，绑定已有的 --accent / --scale / --density / --motion，并补上明暗切换（--mode），含色板与取色、缩放与密度滑杆、动效开关和重置。选择保存在 localStorage 并在加载时恢复。整个包继续离线可运行，其余设计保持不变。",
  },
];

export function DesignNextSteps({
  onPick,
  disabled,
  className,
}: {
  /** Receives the instruction text; the caller decides where to put it. */
  onPick: (instruction: string) => void;
  disabled: boolean;
  className?: string;
}) {
  return (
    <div className={cn("min-w-0", className)}>
      <div className="flex items-baseline gap-1.5 text-caption text-muted-foreground">
        <Sparkles className="size-3 shrink-0 translate-y-0.5" />
        <span>下一步</span>
      </div>
      <div className="mt-1.5 flex flex-wrap gap-1.5">
        {NEXT_STEPS.map((step) => (
          <button
            key={step.id}
            type="button"
            disabled={disabled}
            title={step.instruction}
            onClick={() => onPick(step.instruction)}
            className={cn(
              "cursor-pointer rounded-full border px-2.5 py-1 text-caption text-muted-foreground",
              "hover:bg-accent hover:text-foreground",
              "disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-transparent",
            )}
          >
            {step.label}
          </button>
        ))}
      </div>
    </div>
  );
}
