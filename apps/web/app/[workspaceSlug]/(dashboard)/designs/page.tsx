"use client";

import { DesignsPage } from "@multica/views/designs";

export default function Page() {
  return <DesignsPage figmaPluginDownloadUrl={process.env.NEXT_PUBLIC_FIGMA_PLUGIN_DOWNLOAD_URL} />;
}
