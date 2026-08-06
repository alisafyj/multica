"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  TEST_CASE_EXECUTION_MODES,
  TEST_CASE_PRIORITIES,
  TEST_CASE_SCOPES,
  TEST_CASE_TYPES,
  testCaseDetailOptions,
  testCaseRevisionsOptions,
  useApproveTestCase,
  useDeleteTestCase,
  useUpdateTestCase,
} from "@multica/core/testing";
import type {
  TestCase,
  TestCaseChangeKind,
  TestCaseExecutionMode,
  TestCasePriority,
  TestCaseRepo,
  TestCaseScope,
  TestCaseStep,
  TestCaseType,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { NativeSelect } from "@multica/ui/components/ui/native-select";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { PageHeader } from "../layout/page-header";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";
import { crossRepoWarning, repoAliases } from "./case-summary";
import { TestCaseStepsEditor } from "./components/test-case-steps-editor";
import { TestCaseReposField } from "./components/test-case-repos-field";

interface TestCaseDetailProps {
  /** A TC-<n> key or a UUID; the server resolves both. */
  refId: string;
}

interface DraftState {
  title: string;
  module: string;
  preconditions: string;
  expectedResult: string;
  steps: TestCaseStep[];
  repos: TestCaseRepo[];
  priority: string;
  caseType: string;
  scope: string;
  executionMode: string;
}

function toDraft(testCase: TestCase): DraftState {
  return {
    title: testCase.title,
    module: testCase.module,
    preconditions: testCase.preconditions,
    expectedResult: testCase.expected_result,
    steps: testCase.steps,
    repos: testCase.repos,
    priority: testCase.priority,
    caseType: testCase.case_type,
    scope: testCase.scope,
    executionMode: testCase.execution_mode,
  };
}

export function TestCaseDetail({ refId }: TestCaseDetailProps) {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();

  const { data: testCase, isLoading } = useQuery(testCaseDetailOptions(wsId, refId));
  const { data: revisions = [] } = useQuery(testCaseRevisionsOptions(wsId, refId));
  const updateCase = useUpdateTestCase();
  const approveCase = useApproveTestCase();
  const deleteCase = useDeleteTestCase();

  const [draft, setDraft] = useState<DraftState | null>(null);
  // Version is the server's change counter, so re-seeding on it picks up both
  // our own saves and someone else's edit arriving over the websocket.
  // Deliberately NOT depending on `testCase` itself: a cache invalidation hands
  // back a new object identity with the same version, and re-seeding on that
  // would discard whatever the user is currently typing.
  useEffect(() => {
    if (testCase) setDraft(toDraft(testCase));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [testCase?.id, testCase?.version]);

  if (isLoading || !testCase || !draft) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader>
          <span className="text-body text-muted-foreground">{t(($) => $.detail.untitled)}</span>
        </PageHeader>
      </div>
    );
  }

  // Local alias: the null guard above does not narrow `draft` inside the
  // hoisted function declarations below.
  const current: DraftState = draft;
  const loaded: TestCase = testCase;
  const warning = crossRepoWarning({
    ...loaded,
    scope: current.scope as TestCaseScope,
    repos: current.repos,
  });
  const busy = updateCase.isPending || approveCase.isPending || deleteCase.isPending;

  function patch(next: Partial<DraftState>) {
    setDraft((previous) => (previous ? { ...previous, ...next } : previous));
  }

  function save() {
    updateCase.mutate({
      ref: refId,
      title: current.title,
      module: current.module,
      preconditions: current.preconditions,
      expected_result: current.expectedResult,
      steps: current.steps,
      repos: current.repos.map((repo) => ({
        project_resource_id: repo.project_resource_id,
        alias: repo.alias,
        role: repo.role,
        path_globs: repo.path_globs,
      })),
      priority: current.priority as TestCasePriority,
      case_type: current.caseType as TestCaseType,
      scope: current.scope as TestCaseScope,
      execution_mode: current.executionMode as TestCaseExecutionMode,
    });
  }

  // Delete navigates, so it has to await the server: a failed request must
  // leave the user on a page whose case still exists.
  function remove() {
    deleteCase.mutate(refId, {
      onSuccess: () => navigation.push(paths.tests()),
    });
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <span className="shrink-0 text-body text-muted-foreground tabular-nums">
            {testCase.key}
          </span>
          <span className="truncate text-body font-medium">{testCase.title}</span>
        </div>
        {testCase.status === "draft" ? (
          <Button size="sm" disabled={busy} onClick={() => approveCase.mutate(refId)}>
            {t(($) => $.actions.approve)}
          </Button>
        ) : null}
        <Button size="sm" variant="ghost" disabled={busy} onClick={remove}>
          <Trash2 className="size-4" />
          {t(($) => $.actions.delete)}
        </Button>
      </PageHeader>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-6 overflow-auto p-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
        <div className="flex min-w-0 flex-col gap-4">
          <Field label={t(($) => $.columns.title)}>
            <Input value={current.title} disabled={busy} onChange={(e) => patch({ title: e.target.value })} />
          </Field>

          <Field label={t(($) => $.detail.preconditions)}>
            <Textarea
              value={current.preconditions}
              disabled={busy}
              rows={3}
              onChange={(e) => patch({ preconditions: e.target.value })}
            />
          </Field>

          <Field label={t(($) => $.detail.steps)}>
            <TestCaseStepsEditor
              value={current.steps}
              disabled={busy}
              repoAliases={repoAliases({ repos: current.repos })}
              onChange={(steps) => patch({ steps })}
            />
          </Field>

          <Field label={t(($) => $.detail.expected)}>
            <Textarea
              value={current.expectedResult}
              disabled={busy}
              rows={3}
              onChange={(e) => patch({ expectedResult: e.target.value })}
            />
          </Field>

          <div className="flex gap-2">
            <Button size="sm" disabled={busy} onClick={save}>
              {t(($) => $.actions.save)}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={() => setDraft(toDraft(loaded))}
            >
              {t(($) => $.actions.cancel)}
            </Button>
          </div>
        </div>

        <aside className="flex min-w-0 flex-col gap-4">
          <Field label={t(($) => $.detail.module)}>
            <Input value={current.module} disabled={busy} onChange={(e) => patch({ module: e.target.value })} />
          </Field>

          <EnumField
            label={t(($) => $.detail.priority)}
            value={current.priority}
            disabled={busy}
            options={TEST_CASE_PRIORITIES.map((p) => ({ value: p, label: t(($) => $.priority[p]) }))}
            onChange={(priority) => patch({ priority })}
          />
          <EnumField
            label={t(($) => $.detail.type)}
            value={current.caseType}
            disabled={busy}
            options={TEST_CASE_TYPES.map((c) => ({ value: c, label: t(($) => $.caseType[c]) }))}
            onChange={(caseType) => patch({ caseType })}
          />
          <EnumField
            label={t(($) => $.detail.scope)}
            value={current.scope}
            disabled={busy}
            options={TEST_CASE_SCOPES.map((s) => ({ value: s, label: t(($) => $.scope[s]) }))}
            onChange={(scope) => patch({ scope })}
          />
          <EnumField
            label={t(($) => $.detail.executionMode)}
            value={current.executionMode}
            disabled={busy}
            options={TEST_CASE_EXECUTION_MODES.map((m) => ({
              value: m,
              label: t(($) => $.executionMode[m]),
            }))}
            onChange={(executionMode) => patch({ executionMode })}
          />

          <Field label={t(($) => $.detail.repos)}>
            {warning === "missing_repos" ? (
              <p className="mb-2 text-caption text-warning">
                {t(($) => $.repos.crossRepoNeedsRepos)}
              </p>
            ) : null}
            {warning === "single_role" ? (
              <p className="mb-2 text-caption text-warning">
                {t(($) => $.repos.crossRepoNeedsRoles)}
              </p>
            ) : null}
            <TestCaseReposField
              wsId={wsId}
              projectId={testCase.project_id}
              value={current.repos}
              disabled={busy}
              onChange={(repos) => patch({ repos })}
            />
          </Field>

          <Field label={t(($) => $.detail.revisions)}>
            {revisions.length === 0 ? (
              <p className="text-caption text-muted-foreground">{t(($) => $.revisions.empty)}</p>
            ) : (
              <ul className="flex flex-col gap-1">
                {revisions.map((revision) => (
                  <li key={revision.id} className="text-caption text-muted-foreground">
                    <span className="tabular-nums">
                      {t(($) => $.revisions.version, { version: revision.version })}
                    </span>
                    {t(($) => $.revisions.separator)}
                    {t(($) => $.revisions[revision.change_kind as TestCaseChangeKind])}
                  </li>
                ))}
              </ul>
            )}
          </Field>
        </aside>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-caption text-muted-foreground">{label}</span>
      {children}
    </div>
  );
}

function EnumField({
  label,
  value,
  options,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <Field label={label}>
      <NativeSelect
        value={value}
        disabled={disabled}
        aria-label={label}
        onChange={(event) => onChange(event.target.value)}
      >
        {/* A value the frontend does not know still renders, so a newer backend
            enum never blanks the field. */}
        {options.some((option) => option.value === value) ? null : (
          <option value={value}>{value}</option>
        )}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </NativeSelect>
    </Field>
  );
}
