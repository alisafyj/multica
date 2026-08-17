import { useMemo } from "react";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { conflictId, TruncatedValue, type DiffFieldRow, type DiffFilter, type DiffView } from "./pmo-diff";
import type { PMOApplyChoice } from "@multica/core/types";
import { useT } from "../i18n";

interface SourceOwner {
  externalId: string;
  displayName: string;
}

interface SourceTask {
  taskId: string;
  schemeId: string;
  schemeName: string;
  title: string;
  owner: SourceOwner | null;
  startDate: string | null;
  dueDate: string | null;
  workload: number | null;
  sourceStatus: string;
  status: string;
}

interface SourceRequirement {
  key: string;
  displayNumber: string;
  title: string;
  sourceStatus: string;
  status: string;
  priority: string;
  prdUrl: string | null;
  owner: SourceOwner | null;
  startDate: string | null;
  dueDate: string | null;
  workload: number | null;
  tasks: SourceTask[];
}

interface SourceView {
  parent: SourceRequirement;
  children: SourceRequirement[];
}

export interface PMOSourcePreviewProps {
  snapshot: unknown;
  diff: DiffView | null;
  filter: DiffFilter;
  rows: DiffFieldRow[];
  selections: Record<string, PMOApplyChoice>;
  onSelectionChange: (row: DiffFieldRow, choice: PMOApplyChoice) => void;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object";
}

function readString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function readNullableString(value: unknown): string | null {
  const result = readString(value);
  return result || null;
}

function readNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function safeUrl(value: string): string | null {
  try {
    const url = new URL(value.trim());
    return url.protocol === "http:" || url.protocol === "https:" ? value.trim() : null;
  } catch {
    return null;
  }
}

function firstDescriptionUrl(value: string): string | null {
  const direct = safeUrl(value);
  if (direct) return direct;
  const matches = value.match(/https?:\/\/[^\s<>"')]+/g) ?? [];
  return matches.map(safeUrl).find((url): url is string => Boolean(url)) ?? null;
}

function readOwner(value: unknown): SourceOwner | null {
  if (!isRecord(value)) return null;
  const externalId = readString(value.external_id);
  if (!externalId) return null;
  return { externalId, displayName: readString(value.display_name) };
}

function readTask(value: unknown): SourceTask | null {
  if (!isRecord(value)) return null;
  const taskId = readString(value.task_id);
  const title = readString(value.title);
  if (!taskId || !title) return null;
  return {
    taskId,
    schemeId: readString(value.scheme_id),
    schemeName: readString(value.scheme_name),
    title,
    owner: readOwner(value.owner),
    startDate: readNullableString(value.start_date),
    dueDate: readNullableString(value.due_date),
    workload: readNumber(value.workload),
    sourceStatus: readString(value.source_status),
    status: readString(value.status),
  };
}

function readRequirement(value: unknown, extraTasks: unknown[] = []): SourceRequirement | null {
  if (!isRecord(value)) return null;
  const key = readString(value.key);
  const title = readString(value.title);
  if (!key || !title) return null;
  const ownTasks = Array.isArray(value.tasks) ? value.tasks : [];
  const tasksById = new Map<string, SourceTask>();
  for (const task of [...ownTasks, ...extraTasks].map(readTask)) {
    if (task && !tasksById.has(task.taskId)) tasksById.set(task.taskId, task);
  }
  const tasks = [...tasksById.values()];
  const prdUrl = safeUrl(readString(value.prd_url)) ?? firstDescriptionUrl(readString(value.description));
  return {
    key: key || title,
    displayNumber: readString(value.display_number),
    title: title || key,
    sourceStatus: readString(value.source_status),
    status: readString(value.status),
    priority: readString(value.priority),
    prdUrl,
    owner: readOwner(value.owner),
    startDate: readNullableString(value.start_date),
    dueDate: readNullableString(value.due_date),
    workload: readNumber(value.workload),
    tasks,
  };
}

export function parsePMOSourceView(snapshot: unknown): SourceView | null {
  if (!isRecord(snapshot)) return null;
  const topLevelTasks = Array.isArray(snapshot.tasks) ? snapshot.tasks : [];
  const parent = readRequirement(snapshot.parent_requirement, topLevelTasks);
  if (!parent) return null;
  const children = (Array.isArray(snapshot.child_requirements) ? snapshot.child_requirements : [])
    .map((child) => readRequirement(child))
    .filter((child): child is SourceRequirement => Boolean(child));
  return { parent, children };
}

function taskRows(task: SourceTask, rows: DiffFieldRow[]): DiffFieldRow[] {
  return rows.filter((row) => row.externalType === "task" && row.entityKey === task.taskId);
}

function decisionCell(
  row: DiffFieldRow | undefined,
  selections: Record<string, PMOApplyChoice>,
  onSelectionChange: (row: DiffFieldRow, choice: PMOApplyChoice) => void,
  labels: {
    chooseExternal: string;
    chooseLocal: string;
    incoming: string;
    localOnly: string;
    converged: string;
    local: string;
  },
) {
  if (!row || row.decision === "unchanged") return null;
  if (row.decision === "conflict") {
    const selected = selections[conflictId(row)];
    return (
      <div className="flex flex-col items-start gap-1">
        <span className="max-w-48 truncate text-micro text-muted-foreground" title={String(row.local ?? "—")}>
          {labels.local}: {String(row.local ?? "—")}
        </span>
        <div className="flex items-center gap-1.5">
          <Button
            size="xs"
            variant={selected === "external" ? "default" : "outline"}
            onClick={() => onSelectionChange(row, "external")}
            aria-label={`${labels.chooseExternal} ${row.entityKey} ${row.field}`}
          >
            {labels.chooseExternal}
          </Button>
          <Button
            size="xs"
            variant={selected === "local" ? "default" : "outline"}
            onClick={() => onSelectionChange(row, "local")}
            aria-label={`${labels.chooseLocal} ${row.entityKey} ${row.field}`}
          >
            {labels.chooseLocal}
          </Button>
        </div>
      </div>
    );
  }
  const label = row.decision === "incoming"
    ? labels.incoming
    : row.decision === "local_only"
      ? labels.localOnly
      : labels.converged;
  return (
    <div className="flex flex-col items-start gap-1">
      {row.decision === "local_only" ? (
        <span className="max-w-48 truncate text-micro text-muted-foreground" title={String(row.local ?? "—")}>
          {labels.local}: {String(row.local ?? "—")}
        </span>
      ) : null}
      <Badge variant={row.decision === "local_only" ? "outline" : "secondary"}>{label}</Badge>
    </div>
  );
}

function RequirementSummary({
  requirement,
  rows,
  selections,
  onSelectionChange,
}: {
  requirement: SourceRequirement;
  rows: DiffFieldRow[];
  selections: Record<string, PMOApplyChoice>;
  onSelectionChange: (row: DiffFieldRow, choice: PMOApplyChoice) => void;
}) {
  const { t } = useT("pmo");
  const requirementRows = rows.filter(
    (row) => row.externalType === "requirement" && row.entityKey === requirement.key,
  );
  const labels = {
    chooseExternal: t(($) => $.preview.choose_external),
    chooseLocal: t(($) => $.preview.choose_local),
    incoming: t(($) => $.preview.decision_incoming),
    localOnly: t(($) => $.preview.decision_local_only),
    converged: t(($) => $.preview.decision_converged),
    local: t(($) => $.preview.local),
  };
  return (
    <section className="space-y-2 py-4">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <h2 className="text-title font-medium">{requirement.title}</h2>
        <span className="font-mono text-caption text-muted-foreground">{requirement.displayNumber || requirement.key}</span>
      </div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-caption text-muted-foreground">
        {requirement.priority ? <span>{requirement.priority}</span> : null}
        {requirement.sourceStatus ? <span>{requirement.sourceStatus}</span> : null}
        {requirement.status && requirement.status !== requirement.sourceStatus ? <span>{requirement.status}</span> : null}
        {requirement.owner ? <span>{requirement.owner.displayName || requirement.owner.externalId}</span> : null}
        {requirement.startDate ? <span>{requirement.startDate}</span> : null}
        {requirement.dueDate ? <span>{requirement.dueDate}</span> : null}
        {requirement.workload !== null ? <span>{requirement.workload}</span> : null}
        {requirement.prdUrl ? (
          <a className="text-primary underline underline-offset-2" href={requirement.prdUrl} target="_blank" rel="noreferrer">
            PRD
          </a>
        ) : null}
      </div>
      {requirementRows.length > 0 ? (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-caption">
          {requirementRows.map((row) => (
            <div key={conflictId(row)} className="flex items-center gap-1.5">
              <TruncatedValue value={row.field} />
              {decisionCell(row, selections, onSelectionChange, labels)}
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}

function ScheduleTable({
  requirement,
  rows,
  selections,
  onSelectionChange,
  filter,
}: {
  requirement: SourceRequirement;
  rows: DiffFieldRow[];
  selections: Record<string, PMOApplyChoice>;
  onSelectionChange: (row: DiffFieldRow, choice: PMOApplyChoice) => void;
  filter: DiffFilter;
}) {
  const { t } = useT("pmo");
  const labels = {
    chooseExternal: t(($) => $.preview.choose_external),
    chooseLocal: t(($) => $.preview.choose_local),
    incoming: t(($) => $.preview.decision_incoming),
    localOnly: t(($) => $.preview.decision_local_only),
    converged: t(($) => $.preview.decision_converged),
    local: t(($) => $.preview.local),
  };
  const visibleTasks = requirement.tasks.filter((task) => filter === "all" || taskRows(task, rows).length > 0);
  const groups = useMemo(() => {
    const result = new Map<string, SourceTask[]>();
    for (const task of visibleTasks) {
      const key = `${task.schemeId}\0${task.schemeName}`;
      result.set(key, [...(result.get(key) ?? []), task]);
    }
    return result;
  }, [visibleTasks]);
  if (visibleTasks.length === 0) return null;
  return (
    <section className="space-y-3">
      {[...groups.entries()].map(([key, tasks]) => {
        const [schemeId, schemeName] = key.split("\0");
        return (
          <section key={key} className="space-y-2">
            <h3 className="text-body font-medium">{schemeName || schemeId || t(($) => $.fields.milestone)}</h3>
            <div data-testid="pmo-schedule-scroll" className="overflow-x-auto">
              <table className="min-w-[860px] w-full border-collapse text-left">
                <thead>
                  <tr className="border-b text-caption text-muted-foreground">
                    <th className="px-3 py-2 font-medium">{t(($) => $.entities.task)}</th>
                    <th className="px-3 py-2 font-medium">{t(($) => $.assignees.external_owner)}</th>
                    <th className="px-3 py-2 font-medium">{t(($) => $.fields.start_date)}</th>
                    <th className="px-3 py-2 font-medium">{t(($) => $.fields.due_date)}</th>
                    <th className="px-3 py-2 font-medium">{t(($) => $.fields.workload)}</th>
                    <th className="px-3 py-2 font-medium">{t(($) => $.fields.milestone)}</th>
                    <th className="px-3 py-2 font-medium">{t(($) => $.fields.status)}</th>
                  </tr>
                </thead>
                <tbody>
                  {tasks.map((task) => {
                    const relatedRows = taskRows(task, rows);
                    const rowByField = new Map(relatedRows.map((row) => [row.field, row]));
                    const hiddenRows = relatedRows.filter(
                      (row) => !["title", "assignee_id", "start_date", "due_date", "workload", "status"].includes(row.field)
                        && row.decision !== "unchanged",
                    );
                    return (
                      <tr key={task.taskId} className="border-b last:border-b-0">
                        <td className="px-3 py-2 align-top">
                          <TruncatedValue value={task.title} />
                          <span className="block font-mono text-micro text-muted-foreground">{task.taskId}</span>
                          <div className="mt-1 space-y-1">
                            {decisionCell(rowByField.get("title"), selections, onSelectionChange, labels)}
                            {hiddenRows.map((row) => (
                              <div key={conflictId(row)} className="flex items-center gap-1.5 text-micro text-muted-foreground">
                                <span>{row.field}</span>
                                {decisionCell(row, selections, onSelectionChange, labels)}
                              </div>
                            ))}
                          </div>
                        </td>
                        <td className="px-3 py-2 align-top">
                          <div className="flex flex-col items-start gap-1.5">
                            <span>{task.owner?.displayName || task.owner?.externalId || "—"}</span>
                            {decisionCell(rowByField.get("assignee_id"), selections, onSelectionChange, labels)}
                          </div>
                        </td>
                        <td className="px-3 py-2 align-top">
                          <div className="flex flex-col items-start gap-1.5">
                            <span>{task.startDate || "—"}</span>
                            {decisionCell(rowByField.get("start_date"), selections, onSelectionChange, labels)}
                          </div>
                        </td>
                        <td className="px-3 py-2 align-top">
                          <div className="flex flex-col items-start gap-1.5">
                            <span>{task.dueDate || "—"}</span>
                            {decisionCell(rowByField.get("due_date"), selections, onSelectionChange, labels)}
                          </div>
                        </td>
                        <td className="px-3 py-2 align-top">
                          <div className="flex flex-col items-start gap-1.5">
                            <span>{task.workload ?? "—"}</span>
                            {decisionCell(rowByField.get("workload"), selections, onSelectionChange, labels)}
                          </div>
                        </td>
                        <td className="px-3 py-2 align-top">{task.schemeName || task.schemeId || "—"}</td>
                        <td className="px-3 py-2 align-top">
                          <div className="flex flex-col items-start gap-1.5">
                            <span>{task.sourceStatus || task.status || "—"}</span>
                            {decisionCell(rowByField.get("status"), selections, onSelectionChange, labels)}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>
        );
      })}
    </section>
  );
}

export function PMOSourcePreview({ snapshot, diff: _diff, filter, rows, selections, onSelectionChange }: PMOSourcePreviewProps) {
  const { t } = useT("pmo");
  const source = useMemo(() => parsePMOSourceView(snapshot), [snapshot]);
  if (!source) return null;
  const isVisible = (requirement: SourceRequirement) => filter === "all" || rows.some((row) =>
    (row.externalType === "requirement" && row.entityKey === requirement.key)
      || (row.externalType === "task" && requirement.tasks.some((task) => task.taskId === row.entityKey)),
  );
  const parentVisible = isVisible(source.parent);
  const visibleChildren = source.children.filter(isVisible);
  if (!parentVisible && visibleChildren.length === 0) {
    return <p className="px-4 py-10 text-center text-caption text-muted-foreground">{t(($) => $.preview.filter_empty)}</p>;
  }
  return (
    <div className="space-y-6 py-4">
      {parentVisible ? (
        <>
          <RequirementSummary requirement={source.parent} rows={rows} selections={selections} onSelectionChange={onSelectionChange} />
          <ScheduleTable requirement={source.parent} rows={rows} filter={filter} selections={selections} onSelectionChange={onSelectionChange} />
        </>
      ) : null}
      {visibleChildren.map((child) => (
        <section key={child.key} className="space-y-3">
          <RequirementSummary requirement={child} rows={rows} selections={selections} onSelectionChange={onSelectionChange} />
          <ScheduleTable requirement={child} rows={rows} filter={filter} selections={selections} onSelectionChange={onSelectionChange} />
        </section>
      ))}
    </div>
  );
}
