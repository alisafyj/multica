// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchLatestRelease } from "./github-release";

/** The twelve desktop artifacts a finished release carries. */
function completeAssets(version: string) {
  return [
    `multica-desktop-${version}-mac-arm64.dmg`,
    `multica-desktop-${version}-mac-arm64.zip`,
    `multica-desktop-${version}-mac-x64.dmg`,
    `multica-desktop-${version}-mac-x64.zip`,
    `multica-desktop-${version}-windows-x64.exe`,
    `multica-desktop-${version}-windows-arm64.exe`,
    `multica-desktop-${version}-linux-x86_64.AppImage`,
    `multica-desktop-${version}-linux-amd64.deb`,
    `multica-desktop-${version}-linux-x86_64.rpm`,
    `multica-desktop-${version}-linux-arm64.AppImage`,
    `multica-desktop-${version}-linux-arm64.deb`,
    `multica-desktop-${version}-linux-aarch64.rpm`,
  ].map((name) => ({
    name,
    browser_download_url: `https://github.test/download/v${version}/${name}`,
  }));
}

/** What a release looks like before the Windows/Linux jobs upload. */
function macOnlyAssets(version: string) {
  return completeAssets(version).filter((a) => a.name.includes("-mac-"));
}

function releasePayload(overrides: {
  tag: string;
  assets?: { name: string; browser_download_url: string }[];
  prerelease?: boolean;
  draft?: boolean;
}) {
  return {
    tag_name: overrides.tag,
    published_at: "2026-08-17T10:00:00Z",
    html_url: `https://github.com/coder-zkl1988/multica/releases/tag/${overrides.tag}`,
    prerelease: overrides.prerelease ?? false,
    draft: overrides.draft ?? false,
    assets: overrides.assets ?? [],
  };
}

function mockFetchWithReleases(releases: unknown[]) {
  const fetchMock = vi.fn().mockResolvedValue(
    new Response(JSON.stringify(releases), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("fetchLatestRelease", () => {
  it("uses the newest complete desktop-v release from the fork, ignoring other tags and drafts", async () => {
    const fetchMock = mockFetchWithReleases([
      // Not a desktop-v tag — excluded regardless of assets or prerelease status.
      releasePayload({ tag: "v0.4.16-sso.1", prerelease: true }),
      // A newer desktop-v tag, but a draft — excluded.
      releasePayload({
        tag: "desktop-v0.4.17-sso.3",
        draft: true,
        assets: completeAssets("0.4.17-sso.3"),
      }),
      // Eligible: desktop-v tag, not a draft. Prereleases are allowed for
      // this fork's SSO rollout tags.
      releasePayload({
        tag: "desktop-v0.4.17-sso.2",
        assets: completeAssets("0.4.17-sso.2"),
        prerelease: true,
      }),
    ]);

    const result = await fetchLatestRelease();
    expect(result.version).toBe("desktop-v0.4.17-sso.2");
    expect(result.assets.macArm64Dmg).toContain("0.4.17-sso.2");
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("coder-zkl1988/multica/releases?per_page=20"),
      expect.any(Object),
    );
  });

  // MUL-6313: a release's Windows packaging job failed and its Linux job
  // never finished, so the newest release carried Mac builds only and
  // /download rendered every Windows and Linux button as disabled.
  it("steps back to the newest complete desktop-v release when the latest is missing platforms", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    mockFetchWithReleases([
      releasePayload({ tag: "desktop-v0.4.28", assets: macOnlyAssets("0.4.28") }),
      releasePayload({ tag: "desktop-v0.4.27", assets: completeAssets("0.4.27") }),
    ]);

    const result = await fetchLatestRelease();
    expect(result.version).toBe("desktop-v0.4.27");
    expect(result.htmlUrl).toContain("desktop-v0.4.27");
    expect(result.assets.winX64Exe).toContain("0.4.27");
    expect(result.assets.linuxArm64Rpm).toContain("0.4.27");
  });

  it("keeps stepping back regardless of how old the incomplete release is", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    mockFetchWithReleases([
      {
        ...releasePayload({
          tag: "desktop-v0.4.28",
          assets: macOnlyAssets("0.4.28"),
        }),
        published_at: "2020-01-01T00:00:00Z",
      },
      releasePayload({ tag: "desktop-v0.4.27", assets: completeAssets("0.4.27") }),
    ]);

    const result = await fetchLatestRelease();
    expect(result.version).toBe("desktop-v0.4.27");
  });

  it("searches past several incomplete desktop-v releases", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    mockFetchWithReleases([
      releasePayload({ tag: "desktop-v0.5.2", assets: macOnlyAssets("0.5.2") }),
      releasePayload({ tag: "desktop-v0.5.1", assets: [] }),
      releasePayload({ tag: "desktop-v0.5.0", assets: macOnlyAssets("0.5.0") }),
      releasePayload({ tag: "desktop-v0.4.9", assets: completeAssets("0.4.9") }),
    ]);

    const result = await fetchLatestRelease();
    expect(result.version).toBe("desktop-v0.4.9");
  });

  it("falls back to the latest desktop-v release when no candidate is complete", async () => {
    mockFetchWithReleases([
      releasePayload({ tag: "desktop-v0.4.28", assets: macOnlyAssets("0.4.28") }),
      releasePayload({ tag: "desktop-v0.4.27", assets: macOnlyAssets("0.4.27") }),
    ]);

    const result = await fetchLatestRelease();
    expect(result.version).toBe("desktop-v0.4.28");
    expect(result.assets.macArm64Dmg).toContain("0.4.28");
    expect(result.assets.winX64Exe).toBeUndefined();
  });

  it("returns an empty release shape when the API errors", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response("rate limited", { status: 403 }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    const result = await fetchLatestRelease();
    expect(result).toEqual({
      version: null,
      publishedAt: null,
      htmlUrl: null,
      assets: {},
    });
    expect(warnSpy).toHaveBeenCalled();
  });

  it("returns an empty release shape for a draft", async () => {
    mockFetchWithReleases([
      releasePayload({ tag: "desktop-v0.4.16-sso.1", draft: true }),
    ]);

    const result = await fetchLatestRelease();
    expect(result.version).toBeNull();
    expect(result.assets).toEqual({});
  });

  it("returns an empty release shape when no desktop-v tag is present", async () => {
    mockFetchWithReleases([
      releasePayload({ tag: "v0.4.16-sso.1" }),
      releasePayload({ tag: "cli-v0.4.16" }),
    ]);

    const result = await fetchLatestRelease();
    expect(result.version).toBeNull();
    expect(result.assets).toEqual({});
  });
});
