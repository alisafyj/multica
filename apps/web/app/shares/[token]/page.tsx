"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { useParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchDesignDocumentShareExchange } from "@multica/core/api";
import type { DesignDocumentShareExchange } from "@multica/core/types/design";
import { Button } from "@multica/ui/components/ui/button";
import { inlinePrototypePage, PAGE_LINK_ATTRIBUTE } from "@multica/views/designs/inline-prototype";

/**
 * The public face of a design document share link. An anonymous visitor trades
 * the token for a per-visit preview capability, every referenced file is folded
 * into one self-contained document, and that document runs inside a sandboxed
 * frame — the token alone decides access, so this page never touches the API
 * client or its session credentials.
 */

const SHARE_PAGE_MESSAGE_TYPE = "multica-share-page";

/**
 * The live frame keeps scripts but sits on an opaque origin (`allow-scripts`
 * without `allow-same-origin`), so the parent cannot reach in to intercept page
 * links the way the static canvas does. Instead this capture-phase listener
 * runs inside the frame and forwards each page-link click to the parent via
 * postMessage — the one door an opaque origin leaves open.
 */
const SHARE_BRIDGE_SCRIPT = `<script>(function(){document.addEventListener("click",function(event){var target=event.target;var node=target&&target.closest?target.closest("[${PAGE_LINK_ATTRIBUTE}]"):null;if(!node)return;event.preventDefault();event.stopPropagation();var path=node.getAttribute("${PAGE_LINK_ATTRIBUTE}");if(path)parent.postMessage({type:"${SHARE_PAGE_MESSAGE_TYPE}",path:path},"*");},true);})();</script>`;

/** Puts the bridge as early in the document as the serializer allows. */
function injectShareBridge(html: string): string {
  if (/<head[^>]*>/i.test(html)) {
    return html.replace(/<head[^>]*>/i, (head) => head + SHARE_BRIDGE_SCRIPT);
  }
  return html.replace(/<html[^>]*>/i, (htmlTag) => htmlTag + SHARE_BRIDGE_SCRIPT);
}

/**
 * Reads package files over the per-visit capability route. Plain fetch, no
 * credentials: the capability in the base path is the only authorization an
 * anonymous visitor has or needs.
 */
function shareFileSource(exchange: DesignDocumentShareExchange) {
  return {
    read: async (path: string) => {
      const res = await fetch(`${exchange.resource_base_path}/${path}`);
      if (!res.ok) throw new Error(path);
      return {
        bytes: new Uint8Array(await res.arrayBuffer()),
        mediaType: (res.headers.get("content-type") ?? "").split(";")[0]?.trim() ?? "",
      };
    },
  };
}

function CenteredCard({ children }: { children: ReactNode }) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-muted/30 px-4 py-10">
      <div className="w-full max-w-md rounded-xl border bg-background p-6 shadow-sm">{children}</div>
    </main>
  );
}

export default function DesignSharePage() {
  const params = useParams<{ token: string }>();
  const token = params.token ?? "";
  const queryClient = useQueryClient();

  // One exchange per visit; the server re-issues a fresh capability each time
  // (Cache-Control: no-store), so this query is only refetched on purpose.
  const exchangeQuery = useQuery({
    queryKey: ["design-share", token],
    queryFn: () => fetchDesignDocumentShareExchange(token),
    enabled: token !== "",
    retry: false,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });

  const exchange = exchangeQuery.data?.status === "live" ? exchangeQuery.data.exchange : null;

  if (exchangeQuery.data?.status === "dead") {
    return (
      <CenteredCard>
        <div className="text-body font-semibold">链接已失效</div>
        <p className="mt-2 text-body text-muted-foreground">该分享链接不存在或已被撤销，请联系分享者重新获取。</p>
      </CenteredCard>
    );
  }

  if (exchangeQuery.data?.status === "error") {
    return (
      <CenteredCard>
        <div className="text-body font-semibold">加载失败</div>
        <p className="mt-2 text-body text-muted-foreground">加载分享内容时出现问题，请稍后重试。</p>
        <Button className="mt-5 w-full" disabled={exchangeQuery.isFetching} onClick={() => void exchangeQuery.refetch()}>
          {exchangeQuery.isFetching ? "加载中…" : "重试"}
        </Button>
      </CenteredCard>
    );
  }

  if (!exchange) {
    return (
      <CenteredCard>
        <div className="text-body font-semibold">加载中…</div>
        <p className="mt-2 text-body text-muted-foreground">正在获取分享的设计稿。</p>
      </CenteredCard>
    );
  }

  return <ShareViewer token={token} exchange={exchange} refreshShare={() => void queryClient.invalidateQueries({ queryKey: ["design-share"] })} />;
}

function ShareViewer({
  token,
  exchange,
  refreshShare,
}: {
  token: string;
  exchange: DesignDocumentShareExchange;
  refreshShare: () => void;
}) {
  const frameRef = useRef<HTMLIFrameElement | null>(null);
  const pages = useMemo(() => exchange.pages.filter((page) => page.entry !== ""), [exchange.pages]);
  const initialEntry = exchange.prototype_entry || pages[0]?.entry || "";
  const [entryPath, setEntryPath] = useState(initialEntry);

  // The capability is part of the query key: after a refresh hands back a new
  // one, the inlining re-runs even for the same page.
  const inlineQuery = useQuery({
    queryKey: ["design-share", "inline", token, exchange.resource_base_path, entryPath],
    queryFn: async () => {
      const result = await inlinePrototypePage(entryPath, shareFileSource(exchange), { stripScripts: false });
      return { html: injectShareBridge(result.html) };
    },
    enabled: entryPath !== "",
    retry: false,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });

  // Page navigation from inside the frame arrives as postMessage. Only the
  // share frame's own window is trusted; the payload must name a page path.
  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      if (event.source !== frameRef.current?.contentWindow) return;
      const data = event.data as { type?: unknown; path?: unknown } | null;
      if (data === null || typeof data !== "object") return;
      if (data.type !== SHARE_PAGE_MESSAGE_TYPE || typeof data.path !== "string" || data.path === "") return;
      setEntryPath(data.path);
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  const blobUrl = useMemo(
    () => (inlineQuery.data ? URL.createObjectURL(new Blob([inlineQuery.data.html], { type: "text/html" })) : null),
    [inlineQuery.data],
  );
  useEffect(() => {
    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl);
    };
  }, [blobUrl]);

  if (initialEntry === "") {
    return (
      <CenteredCard>
        <div className="text-body font-semibold">{exchange.document_title}</div>
        <p className="mt-2 text-body text-muted-foreground">该设计稿没有可预览的页面。</p>
      </CenteredCard>
    );
  }

  return (
    <main className="flex h-full flex-col bg-muted/30">
      <header className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b bg-background px-4 py-2.5">
        <div className="min-w-0">
          <div className="truncate text-body font-semibold">{exchange.document_title}</div>
          <div className="text-caption text-muted-foreground">通过 Multica 分享的设计稿原型</div>
        </div>
        {pages.length > 1 ? (
          <nav className="ml-auto flex flex-wrap items-center gap-1.5" aria-label="页面">
            {pages.map((page) =>
              page.entry === "" ? null : (
                <button
                  key={page.id}
                  type="button"
                  onClick={() => setEntryPath(page.entry)}
                  className={
                    "h-7 rounded-full border px-3 text-caption transition-colors " +
                    (page.entry === entryPath
                      ? "border-foreground/30 bg-background font-medium text-foreground"
                      : "border-border bg-background/60 text-muted-foreground hover:bg-background hover:text-foreground")
                  }
                >
                  {page.title}
                </button>
              ),
            )}
          </nav>
        ) : null}
      </header>
      <div className="relative min-h-0 flex-1">
        {inlineQuery.isPending ? (
          <div className="absolute inset-0 flex items-center justify-center text-body text-muted-foreground">加载原型中…</div>
        ) : null}
        {inlineQuery.isError ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 px-4">
            <div className="text-body text-muted-foreground">原型加载失败，分享凭证可能已过期。</div>
            <Button variant="outline" onClick={refreshShare}>
              重新加载
            </Button>
          </div>
        ) : null}
        {blobUrl ? (
          <iframe
            ref={frameRef}
            title={exchange.document_title}
            src={blobUrl}
            // Scripts stay live in a shared prototype, but the frame keeps an
            // opaque origin: without allow-same-origin it cannot touch anything
            // of ours, and the parent cannot reach in — page navigation travels
            // through the injected postMessage bridge instead.
            sandbox="allow-scripts"
            className="h-full w-full border-0 bg-background"
          />
        ) : null}
      </div>
    </main>
  );
}
