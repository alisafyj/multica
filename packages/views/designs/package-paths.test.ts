// @vitest-environment node

// Canonical matrix for package reference resolution. The inliner's own suite
// covers the DOM walk; this covers what a reference resolves to.
import { describe, expect, it } from "vitest";
import {
  cssUrlReferences,
  isExternalReference,
  mediaTypeForPath,
  resolvePackagePath,
} from "./package-paths";

describe("isExternalReference", () => {
  it("claims only references the package can own", () => {
    expect(isExternalReference("styles.css")).toBe(false);
    expect(isExternalReference("../assets/mark.svg")).toBe(false);
    expect(isExternalReference("./a/b.png")).toBe(false);

    expect(isExternalReference("")).toBe(true);
    expect(isExternalReference("   ")).toBe(true);
    expect(isExternalReference("#main")).toBe(true);
    expect(isExternalReference("/absolute.css")).toBe(true);
    expect(isExternalReference("https://cdn.example.com/a.css")).toBe(true);
    expect(isExternalReference("data:image/png;base64,AAAA")).toBe(true);
    expect(isExternalReference("blob:https://x/y")).toBe(true);
    expect(isExternalReference("mailto:a@b.c")).toBe(true);
  });
});

describe("resolvePackagePath", () => {
  it("resolves siblings, parents and dot segments against the referring file", () => {
    expect(resolvePackagePath("prototype/index.html", "styles.css")).toBe("prototype/styles.css");
    expect(resolvePackagePath("prototype/index.html", "./styles.css")).toBe("prototype/styles.css");
    expect(resolvePackagePath("prototype/index.html", "../assets/mark.svg")).toBe("assets/mark.svg");
    expect(resolvePackagePath("prototype/pages/detail.html", "../styles.css")).toBe("prototype/styles.css");
    expect(resolvePackagePath("index.html", "assets/a.png")).toBe("assets/a.png");
  });

  it("drops the query and fragment a static page may still carry", () => {
    expect(resolvePackagePath("prototype/index.html", "styles.css?v=2")).toBe("prototype/styles.css");
    expect(resolvePackagePath("prototype/index.html", "app.js#main")).toBe("prototype/app.js");
  });

  it("refuses references that leave the package or name nothing", () => {
    // One ".." too many: there is no parent of the package root.
    expect(resolvePackagePath("index.html", "../secret.css")).toBeNull();
    expect(resolvePackagePath("prototype/index.html", "../../../etc/passwd")).toBeNull();
    expect(resolvePackagePath("prototype/index.html", "https://cdn/x.css")).toBeNull();
    expect(resolvePackagePath("prototype/index.html", "#top")).toBeNull();
    expect(resolvePackagePath("prototype/index.html", "?only=query")).toBeNull();
  });
});

describe("cssUrlReferences", () => {
  it("finds quoted and bare url() references with their ranges", () => {
    const css = `a{background:url("../assets/a.png")}b{background:url('b.png')}c{background:url(c.svg)}`;
    const found = cssUrlReferences(css);
    expect(found.map((entry) => entry.reference)).toEqual(["../assets/a.png", "b.png", "c.svg"]);
    // The range covers the whole url(...) call, so a replacement supplies its
    // own quoting rather than splicing inside someone else's.
    expect(css.slice(found[0]!.start, found[0]!.end)).toBe(`url("../assets/a.png")`);
    expect(css.slice(found[2]!.start, found[2]!.end)).toBe("url(c.svg)");
  });

  it("tolerates whitespace and skips empty references", () => {
    expect(cssUrlReferences("a{background:url(  'x.png'  )}")[0]?.reference).toBe("x.png");
    expect(cssUrlReferences("a{background:url()}")).toEqual([]);
  });
});

describe("mediaTypeForPath", () => {
  it("types the kinds a prototype package actually carries", () => {
    expect(mediaTypeForPath("prototype/index.html")).toBe("text/html");
    expect(mediaTypeForPath("prototype/styles.CSS")).toBe("text/css");
    expect(mediaTypeForPath("assets/mark.svg")).toBe("image/svg+xml");
    expect(mediaTypeForPath("assets/photo.jpeg")).toBe("image/jpeg");
    expect(mediaTypeForPath("assets/font.woff2")).toBe("font/woff2");
    expect(mediaTypeForPath("weird.bin")).toBe("application/octet-stream");
    expect(mediaTypeForPath("noextension")).toBe("application/octet-stream");
  });
});
