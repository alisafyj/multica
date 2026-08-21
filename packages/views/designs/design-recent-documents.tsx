"use client";

import { useMemo } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { designDocumentListOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectListOptions } from "@multica/core/projects/queries";
import type { DesignDocument } from "@multica/core/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useDesignDocumentActions } from "./design-document-actions";
import { DesignDocumentCard } from "./design-document-card";

/**
 * The document listing is per-project, so "recent across the workspace" costs
 * one request per project. Scanning the most recently touched projects keeps
 * that bounded; a workspace-wide listing endpoint would replace the fan-out.
 */
const SCANNED_PROJECT_LIMIT = 12;
/**
 * Two full rows of the 5-across grid below. Slicing already-fetched results
 * costs nothing extra — the fan-out is bounded by SCANNED_PROJECT_LIMIT — so
 * this is purely about the wall ending on a clean row rather than an orphan.
 */
const RECENT_LIMIT = 10;

function updatedAtOf(document: DesignDocument): number {
  const value = Date.parse(document.updated_at || document.created_at || "");
  return Number.isNaN(value) ? 0 : value;
}

/**
 * "最近生成" on the create panel. Every field comes from the document itself —
 * status, recipe origin and timestamps — so the section never invents progress
 * or usage numbers for a run the server has not reported.
 */
export function DesignRecentDocuments({
  onOpenDocument,
}: {
  /** Opens a document's own workspace. */
  onOpenDocument?: (document: DesignDocument) => void;
}) {
  const wsId = useWorkspaceId();
  const documentActions = useDesignDocumentActions();
  const { data: projects = [] } = useQuery(projectListOptions(wsId));

  const scanned = useMemo(
    () =>
      [...projects]
        .sort((left, right) => Date.parse(right.updated_at) - Date.parse(left.updated_at))
        .slice(0, SCANNED_PROJECT_LIMIT),
    [projects],
  );
  const documentQueries = useQueries({
    queries: scanned.map((project) => designDocumentListOptions(wsId, project.id)),
  });

  const loading = documentQueries.some((query) => query.isLoading);
  const recent: Array<{ document: DesignDocument; projectTitle: string }> = [];
  documentQueries.forEach((query, index) => {
    const project = scanned[index];
    if (!project) return;
    for (const document of query.data ?? []) {
      recent.push({ document, projectTitle: project.title });
    }
  });
  recent.sort((left, right) => updatedAtOf(right.document) - updatedAtOf(left.document));
  const visible = recent.slice(0, RECENT_LIMIT);

  if (!loading && visible.length === 0) {
    return (
      <section className="mt-8">
        <header className="flex items-center gap-2 border-b pb-2 text-caption text-muted-foreground">
          <span className="font-medium text-foreground">最近生成</span>
        </header>
        <p className="mt-3 text-caption text-muted-foreground">
          还没有生成过页面设计。在上面描述你想要的页面，生成的设计稿会出现在这里，也会留在对应项目的「设计稿」里。
        </p>
      </section>
    );
  }

  return (
    <section className="mt-8">
      <header className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-b pb-2 text-caption text-muted-foreground">
        <div className="flex items-center gap-2">
          <span className="font-medium text-foreground">最近生成</span>
          {loading ? null : <span className="font-mono tabular-nums">{visible.length}</span>}
        </div>
        <span>打开设计稿所在项目的「设计稿」查看全部</span>
      </header>
      {loading ? (
        <div className="mt-3 grid gap-4 grid-cols-2 min-[564px]:grid-cols-3 min-[756px]:grid-cols-4 min-[948px]:grid-cols-5">
          {Array.from({ length: 5 }).map((_, index) => (
            <Skeleton key={index} className="aspect-[16/9] w-full rounded-lg" />
          ))}
        </div>
      ) : (
        <div className="mt-3 grid gap-4 grid-cols-2 min-[564px]:grid-cols-3 min-[756px]:grid-cols-4 min-[948px]:grid-cols-5">
          {visible.map((item) => (
            <DesignDocumentCard
              key={item.document.id}
              document={item.document}
              projectTitle={item.projectTitle}
              onOpen={
                onOpenDocument ? () => onOpenDocument(item.document) : undefined
              }
              {...documentActions.cardProps(item.document)}
            />
          ))}
        </div>
      )}
      {documentActions.dialog}
    </section>
  );
}
