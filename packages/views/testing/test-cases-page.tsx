"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FlaskConical, Plus, Sparkles } from "lucide-react";
import { toast } from "sonner";
import { projectListOptions } from "@multica/core/projects/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  TEST_CASE_ORIGINS,
  TEST_CASE_PRIORITIES,
  TEST_CASE_PRIORITY_TONE,
  TEST_CASE_STATUSES,
  TEST_CASE_STATUS_TONE,
  testCaseListOptions,
  testCaseModulesOptions,
  useApproveTestCase,
  useCreateTestCase,
  useCreateTestGenerationJob,
  useTestCaseViewStore,
} from "@multica/core/testing";
import type {
  TestCase,
  TestCaseOrigin,
  TestCasePriority,
  TestCaseStatus,
  TestCaseType,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { NativeSelect } from "@multica/ui/components/ui/native-select";
import { PageHeader } from "../layout/page-header";
import { AppLink, useNavigation } from "../navigation";
import { useT } from "../i18n";
import { formatRepoSummary, knownEnumKey } from "./case-summary";

/**
 * Test case library for one project. The project is the unit of generation and
 * of repository context, so it is a required selection rather than a filter —
 * a workspace-wide case list would have no coherent repo column.
 */
export function TestCasesPage() {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [isBatchApproving, setIsBatchApproving] = useState(false);
  const approveCase = useApproveTestCase();

  const projectId = useTestCaseViewStore((state) => state.projectId);
  const activeModule = useTestCaseViewStore((state) => state.module);
  const filters = useTestCaseViewStore((state) => state.filters);
  const setProjectId = useTestCaseViewStore((state) => state.setProjectId);
  const setModule = useTestCaseViewStore((state) => state.setModule);
  const setFilter = useTestCaseViewStore((state) => state.setFilter);
  const clearFilters = useTestCaseViewStore((state) => state.clearFilters);

  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const selectedProjectId = projectId ?? projects[0]?.id ?? "";

  const { data: modules = [] } = useQuery(testCaseModulesOptions(wsId, selectedProjectId));
  const { data: cases = [], isLoading } = useQuery({
    ...testCaseListOptions(wsId, {
      projectId: selectedProjectId,
      module: activeModule ?? undefined,
      status: filters.statuses[0],
      priority: filters.priorities[0],
      caseType: filters.caseTypes[0],
      origin: filters.origins[0],
    }),
    enabled: selectedProjectId.length > 0,
  });

  const hasFilters = useMemo(
    () => Object.values(filters).some((values) => values.length > 0) || activeModule !== null,
    [filters, activeModule],
  );

  const allVisibleSelected =
    cases.length > 0 && cases.every((c) => selectedIds.has(c.id));

  function toggleSelectAll() {
    if (allVisibleSelected) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(cases.map((c) => c.id)));
    }
  }

  function toggleSelect(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  const navigation = useNavigation();
  const createCase = useCreateTestCase();
  const createGenerationJob = useCreateTestGenerationJob();
  const [isStartingGeneration, setIsStartingGeneration] = useState(false);

  // Create-then-edit: the detail page already owns the full editor, so the
  // list only has to mint a case and hand over. Navigation waits for the
  // server because the new case's key comes back with the response.
  async function createAndOpenCase() {
    if (selectedProjectId.length === 0) return;
    try {
      const created = await createCase.mutateAsync({
        project_id: selectedProjectId,
        title: t(($) => $.page.untitled),
      });
      navigation.push(paths.testCaseDetail(created.key));
    } catch {
      toast.error(t(($) => $.toast.createFailed));
    }
  }

  // A generation run is not dispatched here: creating the job opens its scope
  // for review first, which is the whole point of the plan gate.
  async function startGeneration() {
    if (selectedProjectId.length === 0) return;
    setIsStartingGeneration(true);
    try {
      const job = await createGenerationJob.mutateAsync({ project_id: selectedProjectId });
      toast.success(t(($) => $.toast.generationStarted));
      navigation.push(paths.testGenerationJobDetail(job.id));
    } catch {
      toast.error(t(($) => $.toast.generationFailed));
    } finally {
      setIsStartingGeneration(false);
    }
  }

  async function batchApprove() {
    setIsBatchApproving(true);
    const ids = [...selectedIds];
    let count = 0;
    for (const id of ids) {
      try {
        await approveCase.mutateAsync(id);
        count++;
      } catch {
        // individual failures are silent here; the cache rolls back per case
      }
    }
    setIsBatchApproving(false);
    if (count > 0) {
      toast.success(t(($) => $.toast.batchApproved, { count }));
      setSelectedIds(new Set());
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <FlaskConical className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-body font-medium">{t(($) => $.page.title)}</h1>
        </div>
        <Button
          size="sm"
          variant="outline"
          disabled={selectedProjectId.length === 0 || isStartingGeneration}
          onClick={() => void startGeneration()}
        >
          <Sparkles className="size-4" />
          {isStartingGeneration ? t(($) => $.page.generating) : t(($) => $.page.generate)}
        </Button>
        <Button
          size="sm"
          disabled={selectedProjectId.length === 0 || createCase.isPending}
          onClick={() => void createAndOpenCase()}
        >
          <Plus className="size-4" />
          {t(($) => $.page.new)}
        </Button>
      </PageHeader>

      <div className="flex min-h-0 flex-1">
        <aside className="flex w-56 shrink-0 flex-col gap-3 border-r border-border p-3">
          <label className="flex flex-col gap-1">
            <span className="text-caption text-muted-foreground">
              {t(($) => $.page.selectProject)}
            </span>
            <NativeSelect
              value={selectedProjectId}
              onChange={(event) => setProjectId(event.target.value)}
            >
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.title}
                </option>
              ))}
            </NativeSelect>
          </label>

          <nav className="flex min-h-0 flex-col gap-0.5 overflow-y-auto">
            <ModuleRow
              label={t(($) => $.filters.all)}
              active={activeModule === null}
              onSelect={() => setModule(null)}
            />
            {modules.map((module) => (
              <ModuleRow
                key={module.module}
                label={module.module === "" ? t(($) => $.filters.module) : module.module}
                count={module.case_count}
                active={activeModule === module.module}
                onSelect={() => setModule(module.module)}
              />
            ))}
          </nav>
        </aside>

        <section className="flex min-w-0 flex-1 flex-col">
          <div className="flex flex-wrap items-center gap-2 border-b border-border p-3">
            <FilterSelect
              label={t(($) => $.filters.status)}
              allLabel={t(($) => $.filters.all)}
              value={filters.statuses[0] ?? ""}
              options={TEST_CASE_STATUSES.map((status) => ({
                value: status,
                label: t(($) => $.status[status]),
              }))}
              onChange={(value) => setFilter("statuses", value ? [value] : [])}
            />
            <FilterSelect
              label={t(($) => $.filters.priority)}
              allLabel={t(($) => $.filters.all)}
              value={filters.priorities[0] ?? ""}
              options={TEST_CASE_PRIORITIES.map((priority) => ({
                value: priority,
                label: t(($) => $.priority[priority]),
              }))}
              onChange={(value) => setFilter("priorities", value ? [value] : [])}
            />
            <FilterSelect
              label={t(($) => $.filters.origin)}
              allLabel={t(($) => $.filters.all)}
              value={filters.origins[0] ?? ""}
              options={TEST_CASE_ORIGINS.map((origin) => ({
                value: origin,
                label: t(($) => $.origin[origin]),
              }))}
              onChange={(value) => setFilter("origins", value ? [value] : [])}
            />
            <Button
              size="sm"
              variant="outline"
              onClick={() => setFilter("statuses", ["draft"])}
            >
              {t(($) => $.filters.reviewQueue)}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                setFilter("statuses", ["draft"]);
                setFilter("origins", ["ai"]);
              }}
            >
              {t(($) => $.filters.aiReviewQueue)}
            </Button>
            {selectedIds.size > 0 ? (
              <Button
                size="sm"
                disabled={isBatchApproving}
                onClick={() => void batchApprove()}
              >
                {t(($) => $.actions.batchApprove)} ({selectedIds.size})
              </Button>
            ) : null}
            {hasFilters ? (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  clearFilters();
                  setModule(null);
                  setSelectedIds(new Set());
                }}
              >
                {t(($) => $.filters.clear)}
              </Button>
            ) : null}
          </div>

          <div className="min-h-0 flex-1 overflow-auto">
            {isLoading ? null : cases.length === 0 ? (
              <div className="flex flex-col items-center gap-1 p-12 text-center">
                <p className="text-body font-medium">{t(($) => $.page.empty)}</p>
                <p className="text-caption text-muted-foreground">{t(($) => $.page.emptyHint)}</p>
              </div>
            ) : (
              <table className="w-full text-body">
                <thead>
                  <tr className="border-b border-border text-caption text-muted-foreground">
                    <th className="w-8 px-3 py-2">
                      <input
                        type="checkbox"
                        aria-label={t(($) => $.actions.selectAll)}
                        checked={allVisibleSelected}
                        onChange={toggleSelectAll}
                      />
                    </th>
                    <Th>{t(($) => $.columns.key)}</Th>
                    <Th>{t(($) => $.columns.title)}</Th>
                    <Th>{t(($) => $.columns.module)}</Th>
                    <Th>{t(($) => $.columns.type)}</Th>
                    <Th>{t(($) => $.columns.priority)}</Th>
                    <Th>{t(($) => $.columns.status)}</Th>
                    <Th>{t(($) => $.columns.origin)}</Th>
                    <Th>{t(($) => $.columns.repos)}</Th>
                  </tr>
                </thead>
                <tbody>
                  {cases.map((testCase) => (
                    <CaseRow
                      key={testCase.id}
                      testCase={testCase}
                      href={paths.testCaseDetail(testCase.key)}
                      selected={selectedIds.has(testCase.id)}
                      onToggleSelect={toggleSelect}
                    />
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return <th className="px-3 py-2 text-left font-normal">{children}</th>;
}

function ModuleRow({
  label,
  count,
  active,
  onSelect,
}: {
  label: string;
  count?: number;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      data-active={active || undefined}
      onClick={onSelect}
      // The active row stays identifiable while hovered because selection is
      // carried by font weight and text color, dimensions hover does not touch.
      className="flex items-center justify-between rounded-md px-2 py-1 text-left text-body text-muted-foreground hover:bg-accent data-active:font-medium data-active:text-foreground"
    >
      <span className="truncate">{label}</span>
      {count === undefined ? null : (
        <span className="ml-2 shrink-0 text-caption tabular-nums">{count}</span>
      )}
    </button>
  );
}

function FilterSelect({
  label,
  allLabel,
  value,
  options,
  onChange,
}: {
  label: string;
  allLabel: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}) {
  return (
    <label className="flex items-center gap-1">
      <span className="text-caption text-muted-foreground">{label}</span>
      <NativeSelect value={value} onChange={(event) => onChange(event.target.value)}>
        <option value="">{allLabel}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </NativeSelect>
    </label>
  );
}

function CaseRow({
  testCase,
  href,
  selected,
  onToggleSelect,
}: {
  testCase: TestCase;
  href: string;
  selected: boolean;
  onToggleSelect: (id: string) => void;
}) {
  const { t } = useT("testing");
  const status = knownEnumKey<TestCaseStatus>(testCase.status, TEST_CASE_STATUSES);
  const priority = knownEnumKey<TestCasePriority>(testCase.priority, TEST_CASE_PRIORITIES);
  const origin = knownEnumKey<TestCaseOrigin>(testCase.origin, TEST_CASE_ORIGINS);
  const repoSummary = formatRepoSummary(testCase);

  return (
    <tr className="border-b border-border hover:bg-accent" data-active={selected || undefined}>
      <td className="w-8 px-3 py-2">
        <input
          type="checkbox"
          checked={selected}
          onChange={() => onToggleSelect(testCase.id)}
          aria-label={testCase.key}
        />
      </td>
      <td className="px-3 py-2 text-muted-foreground tabular-nums">
        <AppLink href={href}>{testCase.key}</AppLink>
      </td>
      <td className="max-w-xs px-3 py-2">
        <AppLink href={href} className="block truncate" title={testCase.title}>
          {testCase.title}
        </AppLink>
      </td>
      <td className="px-3 py-2 text-muted-foreground">{testCase.module}</td>
      <td className="px-3 py-2 text-muted-foreground">
        {t(($) => $.caseType[testCase.case_type as TestCaseType]) || testCase.case_type}
      </td>
      <td className={`px-3 py-2 ${priority ? TEST_CASE_PRIORITY_TONE[priority] : ""}`}>
        {priority ? t(($) => $.priority[priority]) : testCase.priority}
      </td>
      <td className={`px-3 py-2 ${status ? TEST_CASE_STATUS_TONE[status] : ""}`}>
        {status ? t(($) => $.status[status]) : testCase.status}
      </td>
      <td className="px-3 py-2 text-muted-foreground">
        {origin ? t(($) => $.origin[origin]) : testCase.origin}
      </td>
      <td className="max-w-xs px-3 py-2 text-muted-foreground">
        <span className="block truncate" title={repoSummary}>
          {repoSummary}
        </span>
      </td>
    </tr>
  );
}
