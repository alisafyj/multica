"use client";

import { useConfigStore } from "@multica/core/config";

/**
 * URL of the marketing /download page, shared by every dashboard entry
 * point that links there (sidebar row, help menu).
 *
 * The page lives on the web origin. On web an empty daemonAppUrl degrades
 * to a same-origin relative link; on desktop the renderer is not the web
 * origin, so the absolute URL from /api/config is what makes the link land
 * in the system browser correctly.
 */
export function useDownloadPageUrl(): string {
  const daemonAppUrl = useConfigStore((state) => state.daemonAppUrl);
  return `${daemonAppUrl.replace(/\/+$/, "")}/download`;
}
