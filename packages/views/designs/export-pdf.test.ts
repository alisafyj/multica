// @vitest-environment node

// The cross-reference table is what makes a PDF readable: every entry is a
// byte offset into this exact file, so an off-by-one produces a document that
// opens as damaged with no useful error. This matrix walks the offsets the way
// a reader does.
import { describe, expect, it } from "vitest";
import { createPdf, type PdfPageImage } from "./export-pdf";

/** Not a real JPEG — the writer embeds bytes verbatim and never decodes them. */
function page(overrides: Partial<PdfPageImage> = {}): PdfPageImage {
  return { jpeg: new Uint8Array([0xff, 0xd8, 0xff, 0xdb, 1, 2, 3, 0xff, 0xd9]), width: 1280, height: 720, ...overrides };
}

function text(pdf: Uint8Array): string {
  return new TextDecoder("latin1").decode(pdf);
}

/** Reads the xref table back as a reader would: offsets in declaration order. */
function xrefOffsets(pdf: Uint8Array): number[] {
  const body = text(pdf);
  const startxref = /startxref\n(\d+)\n%%EOF/.exec(body);
  if (!startxref) throw new Error("no startxref");
  const table = body.slice(Number(startxref[1]));
  const entries = [...table.matchAll(/^(\d{10}) \d{5} [nf] $/gm)].map((match) => Number(match[1]));
  return entries;
}

describe("createPdf", () => {
  it("declares an object at every offset its xref table claims", () => {
    const pdf = createPdf([page(), page({ width: 390, height: 1400 })], "订单总览");
    const body = text(pdf);
    const offsets = xrefOffsets(pdf);

    // Entry 0 is the free head; every other entry must land on "<n> 0 obj".
    expect(offsets[0]).toBe(0);
    offsets.slice(1).forEach((offset, index) => {
      const number = index + 1;
      expect(body.slice(offset, offset + `${number} 0 obj`.length)).toBe(`${number} 0 obj`);
    });
    // startxref must point at the table itself.
    const startxref = Number(/startxref\n(\d+)\n/.exec(body)![1]);
    expect(body.slice(startxref, startxref + 4)).toBe("xref");
  });

  it("sizes each page to its own image rather than a paper aspect ratio", () => {
    const pdf = text(createPdf([page({ width: 1280, height: 720 }), page({ width: 390, height: 1400 })], "t"));
    // 1280 CSS px at 72/96 units per px = 960; 720 -> 540.
    expect(pdf).toContain("/MediaBox [0 0 960 540]");
    // A tall mobile page stays tall.
    expect(pdf).toContain("/MediaBox [0 0 293 1050]");
  });

  it("embeds the JPEG bytes verbatim as DCTDecode", () => {
    const jpeg = new Uint8Array([0xff, 0xd8, 0x41, 0x42, 0xff, 0xd9]);
    const pdf = createPdf([page({ jpeg })], "t");
    const body = text(pdf);
    expect(body).toContain("/Filter /DCTDecode");
    expect(body).toContain(`/Length ${jpeg.length}`);
    // The stream between "stream\n" and "\nendstream" is the payload.
    const start = body.indexOf("stream\n", body.indexOf("/DCTDecode")) + "stream\n".length;
    expect(Array.from(pdf.subarray(start, start + jpeg.length))).toEqual(Array.from(jpeg));
  });

  it("counts the pages it wrote", () => {
    const pdf = text(createPdf([page(), page(), page()], "t"));
    expect(pdf).toContain("/Type /Pages /Count 3");
    // "/Type /Page " with the trailing space, so the page TREE ("/Type
    // /Pages ") is not counted as a page.
    expect((pdf.match(/\/Type \/Page /g) ?? []).length).toBe(3);
  });

  it("draws each image over its whole page", () => {
    const pdf = text(createPdf([page({ width: 1280, height: 720 })], "t"));
    expect(pdf).toContain("960 0 0 540 0 0 cm");
    expect(pdf).toContain("/Im0 Do");
  });

  it("keeps a title a reader cannot render out of the file", () => {
    // Non-Latin-1 would need a UTF-16 string; dropping it beats emitting bytes
    // that render as mojibake in the document properties.
    const pdf = text(createPdf([page()], "订单总览 (v2)"));
    expect(pdf).toContain("/Title");
    expect(pdf).toContain("\\(v2\\)");
    expect(pdf).not.toContain("订单");
  });

  it("opens and closes the file the way a reader looks for", () => {
    const pdf = text(createPdf([page()], "t"));
    expect(pdf.startsWith("%PDF-1.4\n")).toBe(true);
    expect(pdf.endsWith("%%EOF\n")).toBe(true);
    expect(pdf).toContain("/Type /Catalog");
  });

  it("refuses to write a document with no pages", () => {
    expect(() => createPdf([], "t")).toThrow(/at least one page/);
  });
});
