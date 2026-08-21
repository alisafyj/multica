// Needs a DOM: the inliner walks the parsed document. Reference resolution
// itself is covered in package-paths.test.ts, not re-run through here.
import { describe, expect, it } from "vitest";
import { inlinePrototypePage, PAGE_LINK_ATTRIBUTE, toDataUri, type PackageFileSource } from "./inline-prototype";

function source(files: Record<string, string | Uint8Array>): PackageFileSource {
  return {
    read: async (path: string) => {
      const value = files[path];
      if (value === undefined) throw new Error(`missing ${path}`);
      return {
        bytes: typeof value === "string" ? new TextEncoder().encode(value) : value,
        mediaType: "",
      };
    },
  };
}

const PAGE = `<!doctype html><html><head>
  <link rel="stylesheet" href="styles.css">
  <link rel="icon" href="../assets/mark.svg">
</head><body>
  <img src="../assets/logo.png" alt="logo">
  <a href="orders.html">订单</a>
  <a href="https://example.com">外链</a>
  <script src="app.js"></script>
</body></html>`;

const FILES = {
  "prototype/index.html": PAGE,
  "prototype/styles.css": `@import "tokens.css";\nbody{background:url('../assets/bg.png')}`,
  "prototype/tokens.css": ":root{--accent:#ff5701}",
  "prototype/app.js": "window.__ready = true;",
  "assets/mark.svg": "<svg/>",
  "assets/logo.png": new Uint8Array([1, 2, 3]),
  "assets/bg.png": new Uint8Array([4, 5, 6]),
};

async function inlineFixture(options?: { stripScripts?: boolean }) {
  return inlinePrototypePage("prototype/index.html", source(FILES), options);
}

describe("inlinePrototypePage", () => {
  it("produces a document that needs no network", async () => {
    const result = await inlineFixture();

    // Every package reference became a data URI or an inlined block.
    expect(result.html).not.toMatch(/href="styles\.css"/);
    expect(result.html).not.toMatch(/src="\.\.\/assets\/logo\.png"/);
    expect(result.html).toContain("data:image/png;base64,AQID");
    expect(result.html).toContain(':root{--accent:#ff5701}');
    // The stylesheet's own url() and its @import both resolved.
    expect(result.html).toContain("data:image/png;base64,BAUG");
    expect(result.html).toContain('data-multica-inlined-from="prototype/styles.css"');
    expect(result.missing).toEqual([]);
  });

  it("keeps external references from re-opening the network", async () => {
    const result = await inlinePrototypePage(
      "prototype/index.html",
      source({ ...FILES, "prototype/index.html": `<html><head><link rel="stylesheet" href="https://cdn.example.com/a.css"></head><body></body></html>` }),
    );
    expect(result.html).not.toContain("cdn.example.com");
  });

  it("records the pages it links to and neutralises their hrefs", async () => {
    const result = await inlineFixture();
    expect(result.linkedPages).toEqual(["prototype/orders.html"]);
    expect(result.html).toContain(`${PAGE_LINK_ATTRIBUTE}="prototype/orders.html"`);
    // The off-package link keeps its href: it is not this canvas's to rewrite.
    expect(result.html).toContain('href="https://example.com"');
  });

  it("inlines scripts by default and strips them for the static canvas", async () => {
    expect((await inlineFixture()).html).toContain("window.__ready = true;");

    const stripped = await inlineFixture({ stripScripts: true });
    expect(stripped.html).not.toContain("window.__ready");
    expect(stripped.html).not.toContain("<script");
  });

  it("renders without an asset it cannot read rather than refusing to open", async () => {
    const partial = { ...FILES };
    delete (partial as Record<string, unknown>)["assets/logo.png"];
    const result = await inlinePrototypePage("prototype/index.html", source(partial));

    expect(result.missing).toContain("assets/logo.png");
    // The page still opened, and the unreadable image kept its original src.
    expect(result.html).toContain("<img");
    expect(result.html).toContain("../assets/logo.png");
  });

  it("refuses only when the page itself cannot be read", async () => {
    await expect(inlinePrototypePage("prototype/gone.html", source(FILES))).rejects.toThrow(/无法读取页面/);
  });
});

describe("toDataUri", () => {
  it("encodes bytes with their media type, chunking past the argument limit", () => {
    expect(toDataUri(new Uint8Array([1, 2, 3]), "image/png")).toBe("data:image/png;base64,AQID");
    expect(toDataUri(new Uint8Array([1]), "")).toBe("data:application/octet-stream;base64,AQ==");
    // 0x8000 is the chunk boundary; a larger buffer must not blow the stack.
    expect(() => toDataUri(new Uint8Array(0x8000 * 2 + 7), "application/octet-stream")).not.toThrow();
  });
});
