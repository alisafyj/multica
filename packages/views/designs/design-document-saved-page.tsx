"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { designDocumentDetailOptions, designDocumentRevisionOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { NativeSelect, NativeSelectOption } from "@multica/ui/components/ui/native-select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { BreadcrumbHeader } from "../layout/breadcrumb-header";
import { useNavigation } from "../navigation";
import { previewEntries } from "./design-document-preview";

export function DesignDocumentSavedPage({ documentId }: { documentId: string }) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const documentQuery = useQuery(designDocumentDetailOptions(wsId, documentId));
  const document = documentQuery.data;
  // The saved pointer is the only authority here; draft and history are workbench concerns.
  const savedRevisionId = document?.saved_revision_id ?? "";
  const revisionQuery = useQuery(designDocumentRevisionOptions(wsId, documentId, savedRevisionId));
  const revision = savedRevisionId && revisionQuery.data?.id === savedRevisionId ? revisionQuery.data : undefined;
  const entries = useMemo(() => previewEntries(revision), [revision]);
  const [activeEntry, setActiveEntry] = useState("");
  const page = entries.find((entry) => entry.entry === activeEntry) ?? entries[0];
  const previewUrl = revision?.resource_base_path && page
    ? api.getDesignDocumentPreviewFileURL(revision.resource_base_path, page.entry)
    : "";
  const title = document?.title || "设计稿";

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <BreadcrumbHeader
        segments={[{ href: paths.designs(), label: "设计库" }]}
        leaf={<span className="truncate font-medium">{title}</span>}
        actions={<Button size="sm" variant="outline" onClick={() => navigation.push(paths.designDocumentDetail(documentId))}>继续调整</Button>}
      />
      {documentQuery.isLoading ? (
        <div role="status" aria-label="加载设计稿" className="flex min-h-0 flex-1 p-4"><Skeleton className="min-h-64 w-full" /></div>
      ) : documentQuery.isError || !document ? (
        <div role="alert" className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center">
          <p>无法加载这份设计稿</p>
          <Button size="sm" variant="outline" onClick={() => void documentQuery.refetch()}>重试</Button>
        </div>
      ) : !savedRevisionId ? (
        <div role="status" className="flex flex-1 items-center justify-center p-6 text-center text-muted-foreground">
          这份设计稿还没有已保存版本。请前往工作台继续调整并保存。
        </div>
      ) : revisionQuery.isError ? (
        <div role="alert" className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center">
          <p>无法加载已保存版本</p>
          <Button size="sm" variant="outline" onClick={() => void revisionQuery.refetch()}>重试</Button>
        </div>
      ) : revisionQuery.isLoading ? (
        <div role="status" aria-label="加载已保存版本" className="flex min-h-0 flex-1 p-4"><Skeleton className="min-h-64 w-full" /></div>
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-3 border-b bg-background px-4 py-3">
            <Badge variant="secondary">已保存{revision ? ` · v${revision.revision_number}` : ""}</Badge>
            <span className="text-caption text-muted-foreground">只读查看</span>
            {entries.length > 0 ? (
              <NativeSelect aria-label="页面" value={page?.entry ?? ""} onChange={(event) => setActiveEntry(event.target.value)}>
                {entries.map((entry) => <NativeSelectOption key={entry.entry} value={entry.entry}>{entry.title}</NativeSelectOption>)}
              </NativeSelect>
            ) : null}
          </div>
          <div className="flex min-h-0 flex-1 justify-center overflow-auto bg-muted/30 p-4">
            {previewUrl ? (
              <iframe
                key={savedRevisionId + ":" + page?.entry}
                title={title + " · " + page?.title}
                src={previewUrl}
                sandbox="allow-scripts"
                referrerPolicy="no-referrer"
                className="min-h-[480px] w-full rounded-md border bg-background shadow-sm"
                style={{ maxWidth: document.platform === "mobile" ? 390 : undefined }}
              />
            ) : (
              <p role="status" className="self-center text-muted-foreground">已保存版本没有可预览的页面。</p>
            )}
          </div>
        </>
      )}
    </div>
  );
}
