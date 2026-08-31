"use client";

import { Folder, GitBranch } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";

export type DesignMvpViewMode = "project" | "repository";

const modes = [
  { value: "project", label: "项目视角", icon: Folder },
  { value: "repository", label: "仓库视角", icon: GitBranch },
] as const;

export function DesignMvpViewSwitcher({
  mode,
  onModeChange,
}: {
  mode: DesignMvpViewMode;
  onModeChange: (mode: DesignMvpViewMode) => void;
}) {
  return (
    <div
      role="group"
      aria-label="设计中心视角"
      className="inline-flex items-center gap-1 rounded-lg border bg-muted/30 p-1"
    >
      {modes.map(({ value, label, icon: Icon }) => {
        const selected = mode === value;
        return (
          <Tooltip key={value}>
            <TooltipTrigger
              render={
                <Button
                  type="button"
                  variant={selected ? "brand" : "ghost"}
                  size="icon-sm"
                  aria-label={label}
                  aria-pressed={selected}
                  onClick={() => onModeChange(value)}
                >
                  <Icon aria-hidden="true" className="size-4" />
                </Button>
              }
            />
            <TooltipContent>
              {label}
              {selected ? "（已选择）" : ""}
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}
