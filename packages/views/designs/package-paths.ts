/**
 * Resolving the references inside a design document package.
 *
 * A prototype page addresses its own assets the way any static site does —
 * `styles.css`, `../assets/mark.svg` — while the package (and the capability
 * route that serves it) speaks absolute package paths. Turning one into the
 * other is the whole job here, and getting it wrong silently drops an asset
 * from an inlined page, so the matrix lives in package-paths.test.ts.
 */

/** A reference this package cannot own: external, absolute, or in-document. */
export function isExternalReference(reference: string): boolean {
  const value = reference.trim();
  if (value === "") return true;
  if (value.startsWith("#") || value.startsWith("/")) return true;
  // Any scheme (http:, https:, data:, blob:, mailto:, tel:) is off-package.
  return /^[a-z][a-z0-9+.-]*:/i.test(value);
}

/**
 * Resolves `reference` as written inside the package file at `fromPath`, and
 * returns the package path it names — or null when the reference is not this
 * package's to resolve, or escapes it.
 */
export function resolvePackagePath(fromPath: string, reference: string): string | null {
  if (isExternalReference(reference)) return null;
  // Strip the query/fragment a static page may still carry (`a.css?v=2`).
  const cleaned = reference.trim().split(/[?#]/)[0] ?? "";
  if (cleaned === "") return null;

  const segments = fromPath.split("/").slice(0, -1);
  for (const segment of cleaned.split("/")) {
    if (segment === "" || segment === ".") continue;
    if (segment === "..") {
      // Escaping the package root is not a path this package can serve.
      if (segments.length === 0) return null;
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  return segments.length > 0 ? segments.join("/") : null;
}

/**
 * The `url(...)` references a stylesheet makes, with the byte range of each
 * one so a caller can splice replacements in without re-parsing. Quotes are
 * reported as part of the range, so a replacement supplies its own.
 */
export function cssUrlReferences(css: string): Array<{ reference: string; start: number; end: number }> {
  const found: Array<{ reference: string; start: number; end: number }> = [];
  const pattern = /url\(\s*(?:"([^"]*)"|'([^']*)'|([^)'"\s]*))\s*\)/gi;
  for (let match = pattern.exec(css); match !== null; match = pattern.exec(css)) {
    const reference = match[1] ?? match[2] ?? match[3] ?? "";
    if (reference === "") continue;
    found.push({ reference, start: match.index, end: match.index + match[0].length });
  }
  return found;
}

/** The media type to serve a package path as when its index entry is silent. */
export function mediaTypeForPath(path: string): string {
  const extension = path.toLowerCase().split(".").pop() ?? "";
  switch (extension) {
    case "html":
    case "htm":
      return "text/html";
    case "css":
      return "text/css";
    case "js":
    case "mjs":
      return "text/javascript";
    case "json":
      return "application/json";
    case "svg":
      return "image/svg+xml";
    case "png":
      return "image/png";
    case "jpg":
    case "jpeg":
      return "image/jpeg";
    case "gif":
      return "image/gif";
    case "webp":
      return "image/webp";
    case "avif":
      return "image/avif";
    case "woff":
      return "font/woff";
    case "woff2":
      return "font/woff2";
    case "ttf":
      return "font/ttf";
    case "otf":
      return "font/otf";
    default:
      return "application/octet-stream";
  }
}
