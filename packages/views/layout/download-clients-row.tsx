"use client";

import { Download } from "lucide-react";
import { useDownloadPageUrl } from "./use-download-page-url";
import { useT } from "../i18n";

/**
 * "Download apps" entry sharing the sidebar footer strip with the help
 * launcher. Replaced the dismissible Discord promo row: this one is a
 * permanent utility — the only in-app path to the /download page — so it
 * has no dismiss affordance.
 *
 * Deliberately shaped as a sidebar row, not a bordered card: it matches
 * SidebarMenuButton metrics (h-8 / rounded-md / gap-2 / text-body) and
 * carries no resting border or fill, so it is not drawn heavier than the
 * navigation above it. No external-link arrow either — the strip leaves
 * ~128px for the label at the default sidebar width and shrinks 1px per
 * 1px of sidebar drag (MUL-5704), so the label's truncation budget wins
 * and the Download mark alone signals the destination.
 */
export function DownloadClientsRow() {
  const { t } = useT("layout");
  const downloadUrl = useDownloadPageUrl();

  return (
    <div className="min-w-0 flex-1">
      <a
        href={downloadUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="flex h-8 items-center gap-2 rounded-md px-2 text-body text-muted-foreground ring-sidebar-ring outline-hidden transition-colors hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground focus-visible:ring-2"
      >
        <Download className="size-4 shrink-0" />
        <span className="truncate">{t(($) => $.help.download_clients)}</span>
      </a>
    </div>
  );
}
