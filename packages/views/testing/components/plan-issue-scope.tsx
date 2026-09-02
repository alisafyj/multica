"use client";

import { useState } from "react";
import { useQueries } from "@tanstack/react-query";
import { Plus, X } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueDetailOptions } from "@multica/core/issues/queries";
import type { Issue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { IssuePickerModal } from "../../modals/issue-picker-modal";
import { useT } from "../../i18n";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * The issue scope of a generation plan, edited by picking rather than typing.
 *
 * The plan's `issues` array is what the agent is handed and what every case it
 * produces gets linked to, so an entry that resolves to nothing costs the run
 * its provenance. Picking guarantees a real id; the field still renders
 * whatever an agent-authored plan put there, including hand-typed "MUL-123"
 * identifiers, which the server resolves on its own.
 */
export function PlanIssueScope({
  refs,
  onChange,
}: {
  refs: string[];
  onChange: (next: string[]) => void;
}) {
  const { t } = useT("testing");
  const wsId = useWorkspaceId();
  const [pickerOpen, setPickerOpen] = useState(false);

  // A plan saved earlier holds bare UUIDs, so the ids are resolved to their
  // identifiers for display. Only UUID-shaped entries are looked up: an
  // agent-authored plan may carry "MUL-123" already, which needs no request and
  // reads fine as-is.
  const uuidRefs = refs.filter((ref) => UUID_RE.test(ref));
  const resolved = useQueries({
    queries: uuidRefs.map((id) => issueDetailOptions(wsId, id)),
  });
  const labels: Record<string, string> = {};
  uuidRefs.forEach((id, index) => {
    const identifier = resolved[index]?.data?.identifier;
    if (identifier) labels[id] = identifier;
  });

  function add(issue: Issue) {
    if (refs.includes(issue.id)) return;
    onChange([...refs, issue.id]);
  }

  return (
    <div className="flex flex-col gap-1.5">
      {refs.length === 0 ? (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.job.scopeEdit.issuesEmpty)}
        </p>
      ) : (
        <div className="flex flex-wrap gap-1">
          {refs.map((ref) => (
            <span
              key={ref}
              className="inline-flex items-center gap-1 rounded-md border border-border px-1.5 py-0.5 text-caption"
            >
              <span className="max-w-40 truncate tabular-nums">{labels[ref] ?? ref}</span>
              <button
                type="button"
                aria-label={t(($) => $.job.scopeEdit.issuesRemove)}
                onClick={() => onChange(refs.filter((value) => value !== ref))}
                className="shrink-0 text-muted-foreground hover:text-destructive"
              >
                <X className="size-3" />
              </button>
            </span>
          ))}
        </div>
      )}

      <Button
        size="sm"
        variant="outline"
        className="w-fit"
        onClick={() => setPickerOpen(true)}
      >
        <Plus className="size-3.5" />
        {t(($) => $.job.scopeEdit.issuesAdd)}
      </Button>

      <IssuePickerModal
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        title={t(($) => $.job.scopeEdit.issuesPickerTitle)}
        description={t(($) => $.job.scopeEdit.issuesPickerDescription)}
        excludeIds={refs}
        onSelect={add}
      />
    </div>
  );
}
