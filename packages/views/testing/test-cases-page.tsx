"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { FlaskConical, Plus } from "lucide-react";
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
import { AppLink } from "../navigation";
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

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <FlaskConical className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-body font-medium">{t(($) => $.page.title)}</h1>
        </div>
        <Button size="sm" disabled={selectedProjectId.length === 0}>
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
            {hasFilters ? (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  clearFilters();
                  setModule(null);
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
                    <CaseRow key={testCase.id} testCase={testCase} href={paths.testCaseDetail(testCase.key)} />
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

function CaseRow({ testCase, href }: { testCase: TestCase; href: string }) {
  const { t } = useT("testing");
  const status = knownEnumKey<TestCaseStatus>(testCase.status, TEST_CASE_STATUSES);
  const priority = knownEnumKey<TestCasePriority>(testCase.priority, TEST_CASE_PRIORITIES);
  const origin = knownEnumKey<TestCaseOrigin>(testCase.origin, TEST_CASE_ORIGINS);
  const repoSummary = formatRepoSummary(testCase);

  return (
    <tr className="border-b border-border hover:bg-accent">
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
