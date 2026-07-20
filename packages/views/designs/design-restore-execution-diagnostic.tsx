import type { DesignRestoreTask } from "@multica/core/types";

export function restoreExecutionDiagnosticCopy(task: DesignRestoreTask | null | undefined) {
  const status = task?.execution_status;
  if (!status) return null;
  switch (status.reason) {
    case "runtime_offline":
      return { label: "运行时离线", hint: "Agent 所在运行时当前离线，任务会继续等待守护进程恢复。", tone: "warning" as const };
    case "runtime_missing":
      return { label: "运行时缺失", hint: "任务关联的运行时不存在，请检查 Agent 配置。", tone: "warning" as const };
    case "runtime_stale":
      return { label: "运行时心跳超时", hint: "运行时最近没有心跳，任务可能正在等待本地守护进程恢复。", tone: "warning" as const };
    case "queued_over_threshold":
      return { label: "等待领取", hint: "任务已进入队列，但 Agent 暂时还没有领取。", tone: "warning" as const };
    case "running_no_recent_output":
      return { label: "暂无新输出", hint: "Agent task 仍在运行，但最近一段时间没有新的执行输出。", tone: "warning" as const };
    case "waiting_local_directory":
      return { label: "等待本地目录", hint: "本地仓库目录正在被其他 task 占用，释放后会继续执行。", tone: "warning" as const };
    case "agent_task_failed":
      return { label: "Agent task 失败", hint: status.agent_task_error || "Agent task 已失败，可打开任务详情查看错误。", tone: "error" as const };
    case "agent_task_cancelled":
      return { label: "Agent task 已取消", hint: "这次 Agent task 已取消，可重新派发。", tone: "warning" as const };
    default:
      return null;
  }
}

export function RestoreExecutionDiagnostic({ task, className = "" }: { task: DesignRestoreTask | null | undefined; className?: string }) {
  const diagnostic = restoreExecutionDiagnosticCopy(task);
  if (!diagnostic) return null;
  const toneClass = diagnostic.tone === "error"
    ? "border-destructive/30 bg-destructive/10 text-destructive"
    : "border-amber-200 bg-amber-50 text-amber-900";
  return (
    <div className={`rounded-md border p-2 ${toneClass} ${className}`}>
      <div className="font-medium">{diagnostic.label}</div>
      <div className="mt-1">{diagnostic.hint}</div>
    </div>
  );
}
