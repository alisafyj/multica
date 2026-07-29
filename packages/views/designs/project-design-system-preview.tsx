"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { CircleAlert } from "lucide-react";
import type {
  ProjectDesignSystemLocator,
  ProjectDesignSystemScope,
} from "@multica/core/types";

const SELECTION_MESSAGE = "multica:project-design-system-select";

export function ProjectDesignSystemPreview({
  previewHtml,
  locators,
  onSelect,
}: {
  previewHtml: string;
  locators: ProjectDesignSystemLocator[];
  onSelect: (scope: ProjectDesignSystemScope) => void;
}) {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const locatorById = useMemo(
    () => new Map(locators.map((locator) => [locator.id, locator])),
    [locators],
  );

  useEffect(() => {
    setLoadFailed(false);
  }, [previewHtml]);

  useEffect(() => {
    const handleMessage = (event: MessageEvent<unknown>) => {
      if (event.source !== frameRef.current?.contentWindow) return;
      if (!event.data || typeof event.data !== "object" || Array.isArray(event.data)) return;
      const message = event.data as { type?: unknown; id?: unknown };
      if (message.type !== SELECTION_MESSAGE || typeof message.id !== "string") return;
      const locator = locatorById.get(message.id);
      if (!locator) return;
      onSelect({ kind: locator.kind, id: locator.id });
    };

    window.addEventListener("message", handleMessage);
    return () => window.removeEventListener("message", handleMessage);
  }, [locatorById, onSelect]);

  if (!previewHtml.trim() || loadFailed) {
    return (
      <div
        role="alert"
        className="flex min-h-64 items-center justify-center gap-2 border-y px-6 text-sm text-muted-foreground"
      >
        <CircleAlert className="h-4 w-4 shrink-0" />
        <span>UI Kit 暂时不可用，请重新生成或稍后重试。</span>
      </div>
    );
  }

  return (
    <iframe
      ref={frameRef}
      srcDoc={previewHtml}
      sandbox="allow-scripts"
      title="项目设计体系 UI Kit"
      className="h-[680px] w-full border-0 bg-white"
      onError={() => setLoadFailed(true)}
    />
  );
}
