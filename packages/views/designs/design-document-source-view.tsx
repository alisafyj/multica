"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Download, FileCode, FileImage, FileText, LoaderCircle } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import type { DesignDocumentFileEntry, DesignDocumentRevision } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";

/** Files above this stay download-only: the source pane is for reading, not
 *  for streaming a bundle into the DOM. */
const MAX_INLINE_TEXT_BYTES = 1_500_000;

export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

type FileKind = "text" | "image" | "binary";

/**
 * How the pane renders a file, decided by the artifact index's media type
 * (the same value the capability route serves the file with) with an
 * extension fallback for entries an older package left untyped.
 */
export function fileKind(entry: DesignDocumentFileEntry): FileKind {
  const mediaType = entry.media_type.toLowerCase();
  if (mediaType.startsWith("image/") && mediaType !== "image/svg+xml") return "image";
  if (
    mediaType.startsWith("text/")
    || mediaType === "image/svg+xml"
    || mediaType === "application/json"
    || mediaType === "application/javascript"
    || mediaType === "application/xml"
    || mediaType.endsWith("+json")
    || mediaType.endsWith("+xml")
  ) {
    return "text";
  }
  if (mediaType === "") {
    const path = entry.path.toLowerCase();
    if (/[.](html?|css|js|mjs|json|md|txt|svg|xml)$/.test(path)) return "text";
    if (/[.](png|jpe?g|gif|webp|avif)$/.test(path)) return "image";
  }
  return "binary";
}

async function downloadRevisionFile(url: string, path: string) {
  try {
    const response = await fetch(url);
    if (!response.ok) throw new Error(`download failed (${response.status})`);
    const blob = await response.blob();
    const objectUrl = URL.createObjectURL(blob);
    const anchor = window.document.createElement("a");
    anchor.href = objectUrl;
    anchor.download = path.split("/").pop() || path;
    anchor.click();
    URL.revokeObjectURL(objectUrl);
  } catch {
    toast.error("下载失败，请稍后重试");
  }
}

/**
 * The 代码 side of the workspace's 预览/代码 toggle, and the honest version of
 * Open Design's "本轮产出的文件" list: every file of the shown revision's
 * package, readable in place and downloadable one by one, served over the
 * same capability route the preview frame already uses.
 */
export function DesignDocumentSourceView({ revision }: { revision: DesignDocumentRevision }) {
  const files = revision.files;
  const [selectedPath, setSelectedPath] = useState("");
  const activePath = files.some((file) => file.path === selectedPath)
    ? selectedPath
    : (files.some((file) => file.path === revision.prototype_entry) ? revision.prototype_entry : files[0]?.path ?? "");
  const activeFile = files.find((file) => file.path === activePath);
  const activeKind = activeFile ? fileKind(activeFile) : "binary";
  const activeUrl = useMemo(() => {
    if (!activeFile || !revision.resource_base_path) return "";
    try {
      return api.getDesignDocumentPreviewFileURL(revision.resource_base_path, activeFile.path);
    } catch {
      return "";
    }
  }, [activeFile, revision.resource_base_path]);

  const tooLarge = !!activeFile && activeFile.size_bytes > MAX_INLINE_TEXT_BYTES;
  const contentQuery = useQuery({
    // Digest-keyed: the bytes behind a digest never change, so a revision
    // switch refetches and anything else stays cached.
    queryKey: ["design-document-file", revision.content_digest, activePath],
    queryFn: async () => {
      const response = await fetch(activeUrl);
      if (!response.ok) throw new Error(`load failed (${response.status})`);
      return response.text();
    },
    enabled: !!activeUrl && activeKind === "text" && !tooLarge,
    staleTime: Infinity,
    retry: false,
  });

  if (files.length === 0) {
    return (
      <div className="flex h-full min-h-64 w-full items-center justify-center text-caption text-muted-foreground">
        这个版本没有记录文件清单。
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 w-full">
      <nav aria-label="包内文件" className="flex w-64 shrink-0 flex-col overflow-y-auto border-r bg-background">
        {files.map((file) => {
          const kind = fileKind(file);
          const Icon = kind === "image" ? FileImage : kind === "text" ? FileCode : FileText;
          const active = file.path === activePath;
          return (
            <div
              key={file.path}
              className={cn(
                "group/source-file flex min-w-0 items-center gap-1 border-b px-2 py-1.5",
                active ? "bg-accent" : "hover:bg-muted/50",
              )}
            >
              <button
                type="button"
                aria-pressed={active}
                title={file.path}
                onClick={() => setSelectedPath(file.path)}
                className="flex min-w-0 flex-1 items-center gap-2 text-left"
              >
                <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className={cn("min-w-0 flex-1 truncate font-mono text-caption", active && "font-medium")}>{file.path}</span>
                <span className="shrink-0 text-micro tabular-nums text-muted-foreground">{formatFileSize(file.size_bytes)}</span>
              </button>
              <Button
                type="button"
                size="icon-sm"
                variant="ghost"
                aria-label={`下载 ${file.path}`}
                title={`下载 ${file.path}`}
                className="shrink-0 opacity-0 transition-opacity focus-visible:opacity-100 group-hover/source-file:opacity-100"
                onClick={() => {
                  try {
                    const url = api.getDesignDocumentPreviewFileURL(revision.resource_base_path, file.path);
                    void downloadRevisionFile(url, file.path);
                  } catch {
                    toast.error("下载失败，请稍后重试");
                  }
                }}
              >
                <Download className="h-3.5 w-3.5" />
              </Button>
            </div>
          );
        })}
      </nav>

      <div className="min-h-0 min-w-0 flex-1 overflow-auto bg-background">
        {!activeFile || !activeUrl ? (
          <div className="flex h-full items-center justify-center text-caption text-muted-foreground">选择左侧的文件查看内容。</div>
        ) : activeKind === "image" ? (
          <div className="flex min-h-full items-center justify-center p-6">
            <img src={activeUrl} alt={activeFile.path} className="max-h-full max-w-full rounded-md border bg-muted/20" />
          </div>
        ) : activeKind === "binary" || tooLarge ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 text-center text-caption text-muted-foreground">
            <p>{tooLarge ? "文件过大，不在此处展示。" : "二进制文件，不在此处展示。"}</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void downloadRevisionFile(activeUrl, activeFile.path)}>
              <Download className="h-3.5 w-3.5" />
              下载 {activeFile.path.split("/").pop()}
            </Button>
          </div>
        ) : contentQuery.isLoading ? (
          <div className="flex h-full items-center justify-center text-muted-foreground">
            <LoaderCircle className="h-4 w-4 animate-spin" />
          </div>
        ) : contentQuery.isError ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 text-caption text-muted-foreground">
            <p>读取文件失败。</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void contentQuery.refetch()}>重试</Button>
          </div>
        ) : (
          <pre aria-label={`${activeFile.path} 的源码`} className="min-h-full whitespace-pre p-4 font-mono text-caption leading-5">
            {contentQuery.data}
          </pre>
        )}
      </div>
    </div>
  );
}
