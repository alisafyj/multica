// @vitest-environment node

// PowerPoint validates an OOXML package strictly and says nothing useful when
// it fails, so the parts that actually break decks — content types, the
// relationship graph, and media paths matching what the slides reference — are
// asserted here rather than discovered by a user with an unopenable file.
import { describe, expect, it } from "vitest";
import { createPptx, type PptxSlideImage } from "./export-pptx";

const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3]);

function image(overrides: Partial<PptxSlideImage> = {}): PptxSlideImage {
  return { png: PNG, width: 1280, height: 720, title: "首页", ...overrides };
}

/** Reads the archive back by walking its local file headers. */
function unzip(archive: Uint8Array): Map<string, Uint8Array> {
  const files = new Map<string, Uint8Array>();
  const readU16 = (offset: number) => archive[offset]! | (archive[offset + 1]! << 8);
  const readU32 = (offset: number) =>
    (archive[offset]! | (archive[offset + 1]! << 8) | (archive[offset + 2]! << 16) | (archive[offset + 3]! << 24)) >>> 0;

  let offset = 0;
  while (offset + 4 <= archive.length && readU32(offset) === 0x04034b50) {
    const size = readU32(offset + 18);
    const nameLength = readU16(offset + 26);
    const extraLength = readU16(offset + 28);
    const name = new TextDecoder().decode(archive.subarray(offset + 30, offset + 30 + nameLength));
    const start = offset + 30 + nameLength + extraLength;
    files.set(name, archive.subarray(start, start + size));
    offset = start + size;
  }
  return files;
}

function text(files: Map<string, Uint8Array>, path: string): string {
  const data = files.get(path);
  if (!data) throw new Error(`missing part ${path}`);
  return new TextDecoder().decode(data);
}

describe("createPptx", () => {
  it("declares a content type for every part it ships", () => {
    const files = unzip(createPptx([image(), image({ title: "订单" })]));
    const types = text(files, "[Content_Types].xml");

    for (const part of ["/ppt/presentation.xml", "/ppt/slideMasters/slideMaster1.xml", "/ppt/slideLayouts/slideLayout1.xml", "/ppt/theme/theme1.xml", "/ppt/slides/slide1.xml", "/ppt/slides/slide2.xml"]) {
      expect(types).toContain(`PartName="${part}"`);
    }
    // Media is covered by a Default rather than an Override.
    expect(types).toContain(`<Default Extension="png"`);
    // An undeclared part is what makes PowerPoint refuse the whole file.
    expect(types).not.toContain("/ppt/slides/slide3.xml");
  });

  it("puts [Content_Types].xml first, which some readers require", () => {
    const files = unzip(createPptx([image()]));
    expect([...files.keys()][0]).toBe("[Content_Types].xml");
  });

  it("wires every relationship to a part that exists", () => {
    const archive = createPptx([image(), image(), image()]);
    const files = unzip(archive);

    // Every Target in every .rels resolves against that .rels' own directory.
    for (const [path, data] of files) {
      if (!path.endsWith(".rels")) continue;
      const directory = path.replace(/_rels\/[^/]+$/, "");
      for (const match of new TextDecoder().decode(data).matchAll(/Target="([^"]+)"/g)) {
        const target = match[1]!;
        const resolved = new URL(target, `file:///${directory}`).pathname.replace(/^\//, "");
        expect(files.has(resolved)).toBe(true);
      }
    }
  });

  it("gives each slide its own image and points the slide at it", () => {
    const first = new Uint8Array([0x89, 0x50, 1]);
    const second = new Uint8Array([0x89, 0x50, 2]);
    const files = unzip(createPptx([image({ png: first }), image({ png: second })]));

    expect(Array.from(files.get("ppt/media/image1.png")!)).toEqual(Array.from(first));
    expect(Array.from(files.get("ppt/media/image2.png")!)).toEqual(Array.from(second));
    expect(text(files, "ppt/slides/_rels/slide2.xml.rels")).toContain("../media/image2.png");
    // The slide references its image by the relationship id, not by path.
    expect(text(files, "ppt/slides/slide2.xml")).toContain(`r:embed="rId1"`);
  });

  it("lists the slides in the presentation with distinct ids", () => {
    const files = unzip(createPptx([image(), image(), image()]));
    const presentation = text(files, "ppt/presentation.xml");
    expect(presentation).toContain(`<p:sldId id="256" r:id="rId2"/>`);
    expect(presentation).toContain(`<p:sldId id="258" r:id="rId4"/>`);
    // The master keeps rId1, so slides start at rId2.
    expect(text(files, "ppt/_rels/presentation.xml.rels")).toContain(`Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster"`);
  });

  it("sizes the deck to the page rather than letterboxing into widescreen", () => {
    // 1280x720 CSS px at 914400/96 EMU per px.
    const wide = text(unzip(createPptx([image({ width: 1280, height: 720 })])), "ppt/presentation.xml");
    expect(wide).toContain(`<p:sldSz cx="12192000" cy="6858000"/>`);

    // A tall mobile page keeps its shape instead of being cropped to 16:9.
    const tall = text(unzip(createPptx([image({ width: 390, height: 1400 })])), "ppt/presentation.xml");
    expect(tall).toContain(`cx="3714750"`);
    expect(tall).toContain(`cy="13335000"`);
  });

  it("fills the slide with the picture", () => {
    const files = unzip(createPptx([image({ width: 1280, height: 720 })]));
    expect(text(files, "ppt/slides/slide1.xml")).toContain(`<a:ext cx="12192000" cy="6858000"/>`);
  });

  it("escapes a title that would otherwise break the slide XML", () => {
    const files = unzip(createPptx([image({ title: `订单 & "筛选" <张>` })]));
    const slide = text(files, "ppt/slides/slide1.xml");
    expect(slide).toContain("&amp;");
    expect(slide).toContain("&quot;");
    expect(slide).toContain("&lt;张&gt;");
    expect(slide).not.toContain(`name="订单 & "`);
  });

  it("refuses to write a deck with no slides", () => {
    expect(() => createPptx([])).toThrow(/至少一页/);
  });
});
