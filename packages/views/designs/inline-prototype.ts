"use client";

import { cssUrlReferences, isExternalReference, mediaTypeForPath, resolvePackagePath } from "./package-paths";

/**
 * Folding one prototype page and everything it references into a single
 * self-contained HTML document.
 *
 * The live preview frames the package over the capability route under
 * `sandbox="allow-scripts"`, which puts the document on an opaque origin — the
 * workbench cannot read a node out of it, so click-to-select, region capture,
 * rasterising and presentation control are all impossible there. A document
 * with every asset already inlined needs no network, so it can be mounted from
 * a `blob:` URL under `sandbox="allow-same-origin"` instead: the parent gets
 * real DOM access and the sandbox still refuses to run the package's scripts.
 * That trade is the whole point — a static, inspectable page for editing,
 * annotating and exporting; the live frame stays live.
 *
 * The same document is what a share or a single-file publish hands out, which
 * is why the result is a plain string and not a React concern.
 */

export interface PackageFileSource {
  /** Reads one package file. Rejects when the package has no such file. */
  read(path: string): Promise<{ bytes: Uint8Array; mediaType: string }>;
}

export interface InlinePrototypePageResult {
  /** The self-contained document. */
  html: string;
  /** Package paths of the pages this one links to, in document order. */
  linkedPages: string[];
  /** References that could not be read; the document renders without them. */
  missing: string[];
}

export interface InlinePrototypePageOptions {
  /**
   * Drop every script. The static canvas mounts under a sandbox that refuses
   * to run them anyway; removing them keeps a shared or exported document from
   * carrying code it will never be allowed to execute.
   */
  stripScripts?: boolean;
  /** Guard against a stylesheet import cycle. */
  maxImportDepth?: number;
}

const DEFAULT_MAX_IMPORT_DEPTH = 4;

/** Marks the link a click inside the static canvas should treat as a page nav. */
export const PAGE_LINK_ATTRIBUTE = "data-multica-page";

export function toDataUri(bytes: Uint8Array, mediaType: string): string {
  let binary = "";
  // Chunked: String.fromCharCode(...bytes) overflows the argument limit on
  // anything larger than a small icon.
  const chunk = 0x8000;
  for (let index = 0; index < bytes.length; index += chunk) {
    binary += String.fromCharCode(...bytes.subarray(index, index + chunk));
  }
  return `data:${mediaType || "application/octet-stream"};base64,${btoa(binary)}`;
}

/** Reads a package file once per document, however many nodes reference it. */
class FileCache {
  private readonly entries = new Map<string, Promise<{ bytes: Uint8Array; mediaType: string }>>();

  constructor(private readonly source: PackageFileSource, private readonly missing: Set<string>) {}

  read(path: string): Promise<{ bytes: Uint8Array; mediaType: string }> {
    const existing = this.entries.get(path);
    if (existing) return existing;
    const pending = this.source.read(path).catch((error: unknown) => {
      this.missing.add(path);
      throw error;
    });
    this.entries.set(path, pending);
    return pending;
  }

  async dataUri(path: string): Promise<string | null> {
    try {
      const file = await this.read(path);
      return toDataUri(file.bytes, file.mediaType || mediaTypeForPath(path));
    } catch {
      return null;
    }
  }

  async text(path: string): Promise<string | null> {
    try {
      const file = await this.read(path);
      return new TextDecoder().decode(file.bytes);
    } catch {
      return null;
    }
  }
}

/** Rewrites every url() in a stylesheet to a data URI, following @import. */
async function inlineStylesheet(
  css: string,
  fromPath: string,
  cache: FileCache,
  depth: number,
  maxDepth: number,
): Promise<string> {
  const withImports = depth >= maxDepth ? css : await inlineCssImports(css, fromPath, cache, depth, maxDepth);
  const references = cssUrlReferences(withImports);
  if (references.length === 0) return withImports;

  const replacements = await Promise.all(references.map(async (entry) => {
    const path = resolvePackagePath(fromPath, entry.reference);
    if (!path) return null;
    return cache.dataUri(path);
  }));

  let result = "";
  let cursor = 0;
  references.forEach((entry, index) => {
    const dataUri = replacements[index];
    result += withImports.slice(cursor, entry.start);
    result += dataUri ? `url("${dataUri}")` : withImports.slice(entry.start, entry.end);
    cursor = entry.end;
  });
  return result + withImports.slice(cursor);
}

/** Splices `@import` targets into the stylesheet that imports them. */
async function inlineCssImports(
  css: string,
  fromPath: string,
  cache: FileCache,
  depth: number,
  maxDepth: number,
): Promise<string> {
  const pattern = /@import\s+(?:url\(\s*(?:"([^"]*)"|'([^']*)'|([^)'"\s]*))\s*\)|"([^"]*)"|'([^']*)')\s*;?/gi;
  const matches: Array<{ reference: string; start: number; end: number }> = [];
  for (let match = pattern.exec(css); match !== null; match = pattern.exec(css)) {
    const reference = match[1] ?? match[2] ?? match[3] ?? match[4] ?? match[5] ?? "";
    if (reference !== "") {
      matches.push({ reference, start: match.index, end: match.index + match[0].length });
    }
  }
  if (matches.length === 0) return css;

  const imported = await Promise.all(matches.map(async (entry) => {
    const path = resolvePackagePath(fromPath, entry.reference);
    if (!path) return null;
    const text = await cache.text(path);
    if (text === null) return null;
    return inlineStylesheet(text, path, cache, depth + 1, maxDepth);
  }));

  let result = "";
  let cursor = 0;
  matches.forEach((entry, index) => {
    result += css.slice(cursor, entry.start);
    result += imported[index] ?? "";
    cursor = entry.end;
  });
  return result + css.slice(cursor);
}

/** Rewrites a `srcset` candidate list, keeping each candidate's descriptor. */
async function inlineSrcset(value: string, fromPath: string, cache: FileCache): Promise<string> {
  const candidates = value.split(",").map((candidate) => candidate.trim()).filter(Boolean);
  const rewritten = await Promise.all(candidates.map(async (candidate) => {
    const [reference = "", ...descriptor] = candidate.split(/\s+/);
    const path = resolvePackagePath(fromPath, reference);
    if (!path) return candidate;
    const dataUri = await cache.dataUri(path);
    if (!dataUri) return candidate;
    return [dataUri, ...descriptor].join(" ");
  }));
  return rewritten.join(", ");
}

/**
 * Produces the self-contained document for one prototype page.
 *
 * Anything that cannot be read is left as written and reported in `missing`:
 * a page that renders without one background is far better than a page that
 * refuses to open because an asset went astray.
 */
export async function inlinePrototypePage(
  entryPath: string,
  source: PackageFileSource,
  options: InlinePrototypePageOptions = {},
): Promise<InlinePrototypePageResult> {
  const missing = new Set<string>();
  const cache = new FileCache(source, missing);
  const maxImportDepth = options.maxImportDepth ?? DEFAULT_MAX_IMPORT_DEPTH;

  const entryHtml = await cache.text(entryPath);
  if (entryHtml === null) throw new Error(`无法读取页面 ${entryPath}`);
  const document = new DOMParser().parseFromString(entryHtml, "text/html");

  // <base> would re-anchor every relative reference the walk below resolves
  // itself, and points at an origin the inlined document no longer talks to.
  document.querySelectorAll("base").forEach((node) => node.remove());

  const work: Array<Promise<void>> = [];

  document.querySelectorAll("link[href]").forEach((node) => {
    const rel = (node.getAttribute("rel") ?? "").toLowerCase();
    const href = node.getAttribute("href") ?? "";
    const path = resolvePackagePath(entryPath, href);
    if (!path) {
      // A stylesheet this package cannot serve would be fetched over the
      // network by a document that is supposed to be self-contained.
      if (rel.includes("stylesheet") && isExternalReference(href)) node.remove();
      return;
    }
    if (rel.includes("stylesheet")) {
      work.push((async () => {
        const css = await cache.text(path);
        if (css === null) {
          node.remove();
          return;
        }
        const style = document.createElement("style");
        style.setAttribute("data-multica-inlined-from", path);
        style.textContent = await inlineStylesheet(css, path, cache, 0, maxImportDepth);
        node.replaceWith(style);
      })());
      return;
    }
    // Icons, preloads and anything else that names package bytes.
    work.push((async () => {
      const dataUri = await cache.dataUri(path);
      if (dataUri) node.setAttribute("href", dataUri);
      else node.remove();
    })());
  });

  document.querySelectorAll("style").forEach((node) => {
    const css = node.textContent ?? "";
    if (css === "") return;
    work.push((async () => {
      node.textContent = await inlineStylesheet(css, entryPath, cache, 0, maxImportDepth);
    })());
  });

  document.querySelectorAll("[style]").forEach((node) => {
    const declaration = node.getAttribute("style") ?? "";
    if (!declaration.includes("url(")) return;
    work.push((async () => {
      node.setAttribute("style", await inlineStylesheet(declaration, entryPath, cache, maxImportDepth, maxImportDepth));
    })());
  });

  document.querySelectorAll("img[src], source[src], video[src], audio[src], video[poster], input[type=image][src]").forEach((node) => {
    for (const attribute of ["src", "poster"]) {
      const reference = node.getAttribute(attribute);
      if (!reference) continue;
      const path = resolvePackagePath(entryPath, reference);
      if (!path) continue;
      work.push((async () => {
        const dataUri = await cache.dataUri(path);
        if (dataUri) node.setAttribute(attribute, dataUri);
      })());
    }
  });

  document.querySelectorAll("img[srcset], source[srcset]").forEach((node) => {
    const value = node.getAttribute("srcset") ?? "";
    if (value === "") return;
    work.push((async () => {
      node.setAttribute("srcset", await inlineSrcset(value, entryPath, cache));
    })());
  });

  if (options.stripScripts) {
    document.querySelectorAll("script").forEach((node) => node.remove());
  } else {
    document.querySelectorAll("script[src]").forEach((node) => {
      const path = resolvePackagePath(entryPath, node.getAttribute("src") ?? "");
      if (!path) return;
      work.push((async () => {
        const code = await cache.text(path);
        if (code === null) {
          node.remove();
          return;
        }
        node.removeAttribute("src");
        node.setAttribute("data-multica-inlined-from", path);
        node.textContent = code;
      })());
    });
  }

  // Cross-page links: a self-contained document has nowhere to navigate, so
  // the target is recorded for the canvas to act on and the href neutralised.
  const linkedPages: string[] = [];
  document.querySelectorAll("a[href]").forEach((node) => {
    const href = node.getAttribute("href") ?? "";
    const path = resolvePackagePath(entryPath, href);
    if (!path || !/\.html?$/i.test(path)) return;
    if (!linkedPages.includes(path)) linkedPages.push(path);
    node.setAttribute(PAGE_LINK_ATTRIBUTE, path);
    node.setAttribute("href", "#");
  });

  await Promise.all(work);

  const serialized = new XMLSerializer().serializeToString(document.documentElement);
  return {
    html: `<!doctype html>\n${serialized}`,
    linkedPages,
    missing: [...missing],
  };
}
