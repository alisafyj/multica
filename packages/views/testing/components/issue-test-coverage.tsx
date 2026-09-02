"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  TEST_RUN_RESULTS,
  TEST_RUN_RESULT_TONE,
  issueTestCasesOptions,
} from "@multica/core/testing";
import type { IssueTestCaseLink } from "@multica/core/types";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { knownEnumKey } from "../case-summary";

/**
 * The test cases covering one issue, each with its latest recorded outcome.
 *
 * This is the direction that makes a task card answer "is this tested, and does
 * it pass". It renders nothing when the issue has no coverage: an empty block on
 * every issue in a workspace that does not use the testing surface would be
 * noise, and the case side is where a link gets created.
 */
export function IssueTestCoverage({ issueId }: { issueId: string }) {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();

  const { data: cases = [] } = useQuery(issueTestCasesOptions(wsId, issueId));

  if (cases.length === 0) return null;

  const failing = cases.filter((c) => c.latest_result === "failed").length;
  const untested = cases.filter((c) => c.latest_result === null).length;

  return (
    <div>
      <div className="mb-2 flex items-center gap-2 px-2 py-1 text-caption font-medium">
        <span>{t(($) => $.coverage.issueSection)}</span>
        <span className="text-muted-foreground tabular-nums">{cases.length}</span>
        {/* Two numbers earn their place next to the count: a failing case is
            the reason to look, and an unexecuted one means the coverage is a
            claim rather than evidence. */}
        {failing > 0 ? (
          <span className="text-destructive tabular-nums">
            {t(($) => $.coverage.failingCount, { count: failing })}
          </span>
        ) : null}
        {untested > 0 ? (
          <span className="text-muted-foreground tabular-nums">
            {t(($) => $.coverage.untestedCount, { count: untested })}
          </span>
        ) : null}
      </div>
      <ul className="flex flex-col gap-1 pl-2">
        {cases.map((link) => (
          <CoverageRow
            key={link.test_case_id}
            link={link}
            href={paths.testCaseDetail(link.case_key)}
          />
        ))}
      </ul>
    </div>
  );
}

function CoverageRow({ link, href }: { link: IssueTestCaseLink; href: string }) {
  const { t } = useT("testing");
  const result = link.latest_result
    ? knownEnumKey(link.latest_result, TEST_RUN_RESULTS)
    : null;

  return (
    <li className="flex items-center gap-1.5 text-caption">
      <AppLink
        href={href}
        className="shrink-0 text-muted-foreground tabular-nums hover:text-foreground hover:underline"
      >
        {link.case_key}
      </AppLink>
      <AppLink href={href} className="min-w-0 flex-1 truncate hover:underline" title={link.case_title}>
        {link.case_title}
      </AppLink>
      {link.origin === "ai" ? (
        <span className="shrink-0 rounded bg-muted px-1 text-micro text-muted-foreground">
          {t(($) => $.origin.ai)}
        </span>
      ) : null}
      <span
        className={`shrink-0 font-medium ${
          result ? TEST_RUN_RESULT_TONE[result] : "text-muted-foreground"
        }`}
      >
        {/* Never executed reads as "not run", not as a result the case does not
            have. A backend result this build does not know still renders. */}
        {link.latest_result === null
          ? t(($) => $.coverage.neverRun)
          : result
            ? t(($) => $.run.result[result])
            : link.latest_result}
      </span>
    </li>
  );
}
