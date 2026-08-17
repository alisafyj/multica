"use client";

import { useState } from "react";
import { Terminal } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";

type Step = {
  title: string;
  body: string;
  code?: string;
};

const STEPS: Step[] = [
  {
    title: "登录 Multica CLI 并选择 workspace",
    body: "本 MCP 读取 Multica 云端设计数据，需要先登录并确认当前 workspace。",
    code: "multica login\nmultica workspace switch <workspace-slug>",
  },
  {
    title: "生成 MCP 配置",
    body: "该命令会校验登录态和 workspace，并输出可直接粘贴的 MCP 配置片段。",
    code: "multica mcp setup design",
  },
  {
    title: "把配置写入你的 MCP 客户端",
    body: "例如 Claude Desktop、Codex 或 OpenCode 的 mcpServers 配置。",
    code: `{
  "mcpServers": {
    "multica-design": {
      "command": "multica",
      "args": ["mcp", "serve", "design"]
    }
  }
}`,
  },
  {
    title: "在设计中心复制还原范围",
    body: "打开设计文件或 frame，复制“MCP 还原 Prompt”。支持单个 frame、Figma group、选中图层和框选范围。",
  },
  {
    title: "在本地 IDE / coding agent 中使用",
    body: "把复制的 Prompt 交给本地 coding agent，它会调用 multica-design MCP 获取 Restore Pack 并实现 UI。",
  },
  {
    title: "验证 MCP 状态",
    body: "确认 server、workspace 和 auth 都正常。",
    code: "multica mcp status design",
  },
];

export function FigmaMCPGuide() {
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button variant="outline" size="sm" aria-label="Figma MCP 配置" onClick={() => setOpen(true)}>
        <Terminal className="h-3.5 w-3.5" />
        <span className="hidden sm:inline">Figma MCP</span>
        <span className="sm:hidden">MCP</span>
      </Button>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Figma MCP 配置</DialogTitle>
          <DialogDescription>
            使用 Multica Design MCP 在本地 IDE / coding agent 中还原设计稿。
          </DialogDescription>
        </DialogHeader>
        <ol className="space-y-4">
          {STEPS.map((step, index) => (
            <li key={step.title} className="rounded-lg border bg-muted/20 p-4">
              <div className="flex items-start gap-3">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary text-caption font-medium text-primary-foreground">
                  {index + 1}
                </span>
                <div className="min-w-0 flex-1">
                  <h3 className="text-body font-medium">{step.title}</h3>
                  <p className="mt-1 text-body text-muted-foreground">{step.body}</p>
                  {step.code ? (
                    <pre className="mt-3 overflow-x-auto rounded-md bg-background p-3 font-mono text-caption leading-relaxed">
                      <code>{step.code}</code>
                    </pre>
                  ) : null}
                </div>
              </div>
            </li>
          ))}
        </ol>
      </DialogContent>
    </Dialog>
  );
}
