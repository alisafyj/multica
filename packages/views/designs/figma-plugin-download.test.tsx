import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FigmaPluginDownload } from "./figma-plugin-download";

const IMAGE_URL = "https://static.soyoung.com/sy-pre/figma-1779257400638.png";
const DOWNLOAD_URL = "https://static.soyoung.com/sy-design/releases/multica-figma-plugin.zip";
const DEFAULT_DOWNLOAD_URL = "https://static.soyoung.com/sy-pre/multica-figma-plugin-1784509800688.zip";

describe("FigmaPluginDownload", () => {
  it("renders the Figma image and a downloadable CDN link", () => {
    render(<FigmaPluginDownload downloadUrl={DOWNLOAD_URL} />);

    expect(screen.getByRole("img", { name: "Figma" })).toHaveAttribute("src", IMAGE_URL);
    expect(screen.getByRole("link", { name: "下载 Figma 插件" })).toHaveAttribute("href", DOWNLOAD_URL);
    expect(screen.getByRole("link", { name: "下载 Figma 插件" })).toHaveAttribute("download", "multica-figma-plugin.zip");
  });

  it("uses the published CDN package when no override is configured", () => {
    render(<FigmaPluginDownload />);

    expect(screen.getByRole("link", { name: "下载 Figma 插件" })).toHaveAttribute("href", DEFAULT_DOWNLOAD_URL);
  });
});
