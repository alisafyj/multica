import { Download } from "lucide-react";
import { buttonVariants } from "@multica/ui/components/ui/button";

const FIGMA_PLUGIN_IMAGE_URL = "https://static.soyoung.com/sy-pre/figma-1779257400638.png";
const DEFAULT_FIGMA_PLUGIN_DOWNLOAD_URL = "https://static.soyoung.com/sy-pre/multica-figma-plugin-1784509800688.zip";

export function FigmaPluginDownload({ downloadUrl }: { downloadUrl?: string }) {
  const url = downloadUrl?.trim() || DEFAULT_FIGMA_PLUGIN_DOWNLOAD_URL;
  const label = (
    <>
      <Download className="h-3.5 w-3.5" />
      <span className="hidden sm:inline">下载 Figma 插件</span>
      <span className="sm:hidden">下载</span>
    </>
  );

  return (
    <div className="flex shrink-0 items-center gap-1.5">
      <img src={FIGMA_PLUGIN_IMAGE_URL} alt="Figma" width={28} height={28} className="size-7 shrink-0" />
      <a
        href={url}
        download="multica-figma-plugin.zip"
        aria-label="下载 Figma 插件"
        className={buttonVariants({ variant: "outline", size: "sm" })}
      >
        {label}
      </a>
    </div>
  );
}
