/**
 * A minimal .pptx: one slide per rasterised prototype page, each slide a
 * single full-bleed picture.
 *
 * A pptx is an OOXML package — a ZIP of XML parts plus the media they
 * reference. PowerPoint validates that package strictly and reports nothing
 * useful when it fails, so everything here is deliberately the smallest
 * structure that is still complete: one master, one blank layout, one theme,
 * and N slides. The relationship ids and content types are the parts that
 * actually break decks, so export-pptx.test.ts asserts them.
 *
 * Slides are sized to the first page's aspect ratio rather than 16:9. A design
 * review is looking at a screen, and letterboxing a tall mobile page into a
 * widescreen slide throws away most of the pixels.
 */

import { createZip, utf8, type ZipEntry } from "./zip-writer";

export interface PptxSlideImage {
  /** PNG bytes for this slide. */
  png: Uint8Array;
  width: number;
  height: number;
  /** Shown in the slide's shape name; not rendered on the slide. */
  title: string;
}

/** OOXML measures in EMU: 914400 per inch, and 96 CSS pixels per inch. */
const EMU_PER_PIXEL = 914400 / 96;
/** Bounded so one very tall page cannot produce a slide PowerPoint rejects. */
const MAX_SLIDE_EMU = 51206400; // 56 inches, the format's practical ceiling

function emu(pixels: number): number {
  return Math.max(1, Math.min(Math.round(pixels * EMU_PER_PIXEL), MAX_SLIDE_EMU));
}

function xmlEscape(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

const XML_HEADER = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n`;

function contentTypes(slideCount: number): string {
  const slides = Array.from({ length: slideCount }, (_, index) =>
    `<Override PartName="/ppt/slides/slide${index + 1}.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`,
  ).join("");
  return XML_HEADER
    + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`
    + `<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`
    + `<Default Extension="xml" ContentType="application/xml"/>`
    + `<Default Extension="png" ContentType="image/png"/>`
    + `<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`
    + `<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`
    + `<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`
    + `<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`
    + slides
    + `</Types>`;
}

const ROOT_RELS = XML_HEADER
  + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
  + `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>`
  + `</Relationships>`;

function presentation(slideCount: number, widthEmu: number, heightEmu: number): string {
  // Slide ids must be >= 256; the master takes rId1 and slide N takes rId(N+1).
  const slideIds = Array.from({ length: slideCount }, (_, index) =>
    `<p:sldId id="${256 + index}" r:id="rId${index + 2}"/>`,
  ).join("");
  return XML_HEADER
    + `<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
    + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
    + ` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`
    + `<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>`
    + `<p:sldIdLst>${slideIds}</p:sldIdLst>`
    + `<p:sldSz cx="${widthEmu}" cy="${heightEmu}"/>`
    + `<p:notesSz cx="${heightEmu}" cy="${widthEmu}"/>`
    + `</p:presentation>`;
}

function presentationRels(slideCount: number): string {
  const slides = Array.from({ length: slideCount }, (_, index) =>
    `<Relationship Id="rId${index + 2}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide${index + 1}.xml"/>`,
  ).join("");
  return XML_HEADER
    + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
    + `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`
    + slides
    + `<Relationship Id="rId${slideCount + 2}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`
    + `</Relationships>`;
}

/** An empty placeholder tree: the slides carry pictures, not text frames. */
const EMPTY_SHAPE_TREE = `<p:cSld><p:spTree>`
  + `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`
  + `<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/>`
  + `<a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`
  + `</p:spTree></p:cSld>`;

const SLIDE_MASTER = XML_HEADER
  + `<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
  + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
  + ` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`
  + EMPTY_SHAPE_TREE
  + `<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2"`
  + ` accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6"`
  + ` hlink="hlink" folHlink="folHlink"/>`
  + `<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>`
  + `</p:sldMaster>`;

const SLIDE_MASTER_RELS = XML_HEADER
  + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
  + `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`
  + `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>`
  + `</Relationships>`;

const SLIDE_LAYOUT = XML_HEADER
  + `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
  + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
  + ` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">`
  + EMPTY_SHAPE_TREE
  + `</p:sldLayout>`;

const SLIDE_LAYOUT_RELS = XML_HEADER
  + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
  + `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>`
  + `</Relationships>`;

/** The smallest theme PowerPoint accepts: one font scheme, one colour scheme. */
const THEME = XML_HEADER
  + `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Multica">`
  + `<a:themeElements>`
  + `<a:clrScheme name="Multica">`
  + `<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>`
  + `<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>`
  + `<a:dk2><a:srgbClr val="111827"/></a:dk2><a:lt2><a:srgbClr val="F6F6F1"/></a:lt2>`
  + `<a:accent1><a:srgbClr val="FF5701"/></a:accent1><a:accent2><a:srgbClr val="111827"/></a:accent2>`
  + `<a:accent3><a:srgbClr val="6B7280"/></a:accent3><a:accent4><a:srgbClr val="9CA3AF"/></a:accent4>`
  + `<a:accent5><a:srgbClr val="D1D5DB"/></a:accent5><a:accent6><a:srgbClr val="E5E7EB"/></a:accent6>`
  + `<a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink>`
  + `</a:clrScheme>`
  + `<a:fontScheme name="Multica">`
  + `<a:majorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>`
  + `<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>`
  + `</a:fontScheme>`
  + `<a:fmtScheme name="Multica">`
  + `<a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`
  + `<a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>`
  + `<a:lnStyleLst>`
  + `<a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>`
  + `<a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>`
  + `<a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>`
  + `</a:lnStyleLst>`
  + `<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle>`
  + `<a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>`
  + `<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`
  + `<a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>`
  + `</a:fmtScheme>`
  + `</a:themeElements>`
  + `</a:theme>`;

/** One slide: a single picture filling the whole slide. */
function slide(image: PptxSlideImage, widthEmu: number, heightEmu: number): string {
  return XML_HEADER
    + `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
    + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
    + ` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`
    + `<p:cSld><p:spTree>`
    + `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`
    + `<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/>`
    + `<a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`
    + `<p:pic>`
    + `<p:nvPicPr><p:cNvPr id="2" name="${xmlEscape(image.title) || "Page"}"/>`
    + `<p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>`
    + `<p:blipFill><a:blip r:embed="rId1"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>`
    + `<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="${widthEmu}" cy="${heightEmu}"/></a:xfrm>`
    + `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>`
    + `</p:pic>`
    + `</p:spTree></p:cSld><p:clrMapOvr><a:overrideClrMapping bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2"`
    + ` accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5"`
    + ` accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:clrMapOvr></p:sld>`;
}

function slideRels(index: number): string {
  return XML_HEADER
    + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
    + `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image${index + 1}.png"/>`
    + `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`
    + `</Relationships>`;
}

/**
 * Builds the deck. Every slide shares one size — the format has one slide size
 * per presentation — taken from the first page, so a set of same-viewport
 * pages exports without distortion.
 */
export function createPptx(images: ReadonlyArray<PptxSlideImage>): Uint8Array {
  if (images.length === 0) throw new Error("演示文稿需要至少一页");
  const first = images[0]!;
  const widthEmu = emu(first.width);
  const heightEmu = emu(first.height);

  const entries: ZipEntry[] = [
    // [Content_Types].xml must be the first entry in the archive.
    { path: "[Content_Types].xml", data: utf8(contentTypes(images.length)) },
    { path: "_rels/.rels", data: utf8(ROOT_RELS) },
    { path: "ppt/presentation.xml", data: utf8(presentation(images.length, widthEmu, heightEmu)) },
    { path: "ppt/_rels/presentation.xml.rels", data: utf8(presentationRels(images.length)) },
    { path: "ppt/slideMasters/slideMaster1.xml", data: utf8(SLIDE_MASTER) },
    { path: "ppt/slideMasters/_rels/slideMaster1.xml.rels", data: utf8(SLIDE_MASTER_RELS) },
    { path: "ppt/slideLayouts/slideLayout1.xml", data: utf8(SLIDE_LAYOUT) },
    { path: "ppt/slideLayouts/_rels/slideLayout1.xml.rels", data: utf8(SLIDE_LAYOUT_RELS) },
    { path: "ppt/theme/theme1.xml", data: utf8(THEME) },
  ];
  images.forEach((image, index) => {
    entries.push({ path: `ppt/slides/slide${index + 1}.xml`, data: utf8(slide(image, widthEmu, heightEmu)) });
    entries.push({ path: `ppt/slides/_rels/slide${index + 1}.xml.rels`, data: utf8(slideRels(index)) });
    entries.push({ path: `ppt/media/image${index + 1}.png`, data: image.png });
  });
  return createZip(entries);
}
