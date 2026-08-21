/**
 * A minimal PDF writer: one page per rasterised prototype page.
 *
 * The pages arrive as JPEG because JPEG is the one image encoding a PDF takes
 * verbatim — `/DCTDecode` embeds the compressed bytes as they are. A PNG would
 * have to be decoded and re-deflated into a raw colour stream, which is more
 * code, a much larger file, and a new way to produce a document no reader will
 * open. Nothing else in the file needs compressing.
 *
 * The byte offsets in the cross-reference table are what make a PDF readable
 * at all, so they are asserted in export-pdf.test.ts rather than trusted.
 */

export interface PdfPageImage {
  /** JPEG bytes, as produced by canvas.toBlob("image/jpeg"). */
  jpeg: Uint8Array;
  /** Pixel dimensions of that JPEG. */
  width: number;
  height: number;
}

/** PDF user-space units are 1/72 inch; CSS pixels are conventionally 1/96. */
const PDF_UNITS_PER_CSS_PIXEL = 72 / 96;

function latin1(value: string): Uint8Array {
  const bytes = new Uint8Array(value.length);
  for (let index = 0; index < value.length; index += 1) {
    bytes[index] = value.charCodeAt(index) & 0xff;
  }
  return bytes;
}

/**
 * Escapes a string for a PDF literal. Metadata only — page content is images,
 * so this never has to carry anything a reader would render.
 */
function pdfString(value: string): string {
  // Non-Latin-1 characters cannot survive a plain literal, and a title is not
  // worth a UTF-16 encoding path; they are dropped rather than mangled.
  const ascii = value.replace(/[^\x20-\x7e]/g, "");
  return `(${ascii.replace(/[\\()]/g, (character) => `\\${character}`)})`;
}

/**
 * Builds the document. Every page is sized to its own image, so a tall page
 * stays tall instead of being letterboxed into A4 — a design review wants the
 * whole screen, not a paper aspect ratio.
 */
export function createPdf(pages: ReadonlyArray<PdfPageImage>, title: string): Uint8Array {
  if (pages.length === 0) throw new Error("PDF needs at least one page");

  // Object numbering: 1 catalog, 2 page tree, then per page a page object and
  // an image XObject, then the content streams.
  const pageObjectNumber = (index: number) => 3 + index * 3;
  const imageObjectNumber = (index: number) => 4 + index * 3;
  const contentObjectNumber = (index: number) => 5 + index * 3;
  const totalObjects = 2 + pages.length * 3;

  const chunks: Uint8Array[] = [];
  const offsets = new Array<number>(totalObjects + 1).fill(0);
  let length = 0;

  const push = (bytes: Uint8Array) => {
    chunks.push(bytes);
    length += bytes.length;
  };
  const pushText = (value: string) => push(latin1(value));
  const beginObject = (number: number) => {
    offsets[number] = length;
    pushText(`${number} 0 obj\n`);
  };

  pushText("%PDF-1.4\n");
  // A binary comment marks the file as binary for tools that sniff it.
  push(new Uint8Array([0x25, 0xe2, 0xe3, 0xcf, 0xd3, 0x0a]));

  beginObject(1);
  pushText(`<< /Type /Catalog /Pages 2 0 R >>\nendobj\n`);

  beginObject(2);
  const kids = pages.map((_, index) => `${pageObjectNumber(index)} 0 R`).join(" ");
  pushText(`<< /Type /Pages /Count ${pages.length} /Kids [${kids}] >>\nendobj\n`);

  pages.forEach((page, index) => {
    const pageWidth = Math.max(1, Math.round(page.width * PDF_UNITS_PER_CSS_PIXEL));
    const pageHeight = Math.max(1, Math.round(page.height * PDF_UNITS_PER_CSS_PIXEL));

    beginObject(pageObjectNumber(index));
    pushText(
      `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${pageWidth} ${pageHeight}]`
      + ` /Resources << /XObject << /Im0 ${imageObjectNumber(index)} 0 R >> >>`
      + ` /Contents ${contentObjectNumber(index)} 0 R >>\nendobj\n`,
    );

    beginObject(imageObjectNumber(index));
    pushText(
      `<< /Type /XObject /Subtype /Image /Width ${page.width} /Height ${page.height}`
      + ` /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode`
      + ` /Length ${page.jpeg.length} >>\nstream\n`,
    );
    push(page.jpeg);
    pushText("\nendstream\nendobj\n");

    // The image fills the page: scale to the MediaBox, origin bottom-left.
    const content = `q\n${pageWidth} 0 0 ${pageHeight} 0 0 cm\n/Im0 Do\nQ\n`;
    beginObject(contentObjectNumber(index));
    pushText(`<< /Length ${content.length} >>\nstream\n${content}endstream\nendobj\n`);
  });

  const infoNumber = totalObjects + 1;
  offsets.push(0);
  beginObject(infoNumber);
  pushText(`<< /Title ${pdfString(title)} /Producer ${pdfString("Multica Design")} >>\nendobj\n`);

  const xrefOffset = length;
  const objectCount = infoNumber + 1;
  pushText(`xref\n0 ${objectCount}\n`);
  pushText("0000000000 65535 f \n");
  for (let number = 1; number < objectCount; number += 1) {
    pushText(`${String(offsets[number] ?? 0).padStart(10, "0")} 00000 n \n`);
  }
  pushText(`trailer\n<< /Size ${objectCount} /Root 1 0 R /Info ${infoNumber} 0 R >>\nstartxref\n${xrefOffset}\n%%EOF\n`);

  const result = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    result.set(chunk, offset);
    offset += chunk.length;
  }
  return result;
}
