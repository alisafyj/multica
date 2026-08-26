// @vitest-environment node

// The ZIP byte layout is fixed and unforgiving: a wrong offset or a wrong CRC
// produces a file the reader refuses to open, with no useful error. This is
// the canonical matrix; the pptx suite only checks which entries go in.
import { describe, expect, it } from "vitest";
import { createZip, crc32, utf8, type ZipEntry } from "./zip-writer";

function readU16(bytes: Uint8Array, offset: number): number {
  return bytes[offset]! | (bytes[offset + 1]! << 8);
}

function readU32(bytes: Uint8Array, offset: number): number {
  return (bytes[offset]! | (bytes[offset + 1]! << 8) | (bytes[offset + 2]! << 16) | (bytes[offset + 3]! << 24)) >>> 0;
}

/** Locates the end-of-central-directory record, which sits last. */
function endOfCentralDirectory(archive: Uint8Array): number {
  for (let offset = archive.length - 22; offset >= 0; offset -= 1) {
    if (readU32(archive, offset) === 0x06054b50) return offset;
  }
  throw new Error("no end-of-central-directory record");
}

const ENTRIES: ZipEntry[] = [
  { path: "[Content_Types].xml", data: utf8("<Types/>") },
  { path: "ppt/media/image1.png", data: new Uint8Array([0x89, 0x50, 0x4e, 0x47, 1, 2, 3]) },
];

describe("crc32", () => {
  it("matches the known IEEE values readers check against", () => {
    // "123456789" is the standard CRC-32 check vector.
    expect(crc32(utf8("123456789"))).toBe(0xcbf43926);
    expect(crc32(new Uint8Array())).toBe(0);
    expect(crc32(utf8("a"))).toBe(0xe8b7be43);
  });
});

describe("createZip", () => {
  it("writes a local header, the data, and a central directory that points back at it", () => {
    const archive = createZip(ENTRIES);

    // First entry starts at 0 with the local file header signature.
    expect(readU32(archive, 0)).toBe(0x04034b50);
    expect(readU16(archive, 8)).toBe(0); // stored, never deflated
    expect(readU32(archive, 14)).toBe(crc32(ENTRIES[0]!.data));
    expect(readU32(archive, 18)).toBe(ENTRIES[0]!.data.length); // compressed
    expect(readU32(archive, 22)).toBe(ENTRIES[0]!.data.length); // uncompressed

    const end = endOfCentralDirectory(archive);
    expect(readU16(archive, end + 8)).toBe(ENTRIES.length); // entries on disk
    expect(readU16(archive, end + 10)).toBe(ENTRIES.length); // entries total

    const directoryOffset = readU32(archive, end + 16);
    const directorySize = readU32(archive, end + 12);
    expect(readU32(archive, directoryOffset)).toBe(0x02014b50);
    expect(directoryOffset + directorySize).toBe(end);

    // The first directory record points at offset 0, where the first local
    // header actually is. An off-by-one here is what makes an archive open
    // as "corrupt" with no further explanation.
    expect(readU32(archive, directoryOffset + 42)).toBe(0);
  });

  it("keeps entry order, because OOXML wants [Content_Types].xml first", () => {
    const archive = createZip(ENTRIES);
    const firstName = new TextDecoder().decode(archive.subarray(30, 30 + readU16(archive, 26)));
    expect(firstName).toBe("[Content_Types].xml");
  });

  it("stores the payload verbatim so a reader gets the bytes back", () => {
    const archive = createZip(ENTRIES);
    const nameLength = readU16(archive, 26);
    const stored = archive.subarray(30 + nameLength, 30 + nameLength + ENTRIES[0]!.data.length);
    expect(new TextDecoder().decode(stored)).toBe("<Types/>");
  });

  it("marks names as UTF-8, so a non-ASCII path is not mojibake", () => {
    const archive = createZip([{ path: "ppt/media/图片.png", data: new Uint8Array([1]) }]);
    expect(readU16(archive, 6) & 0x0800).toBe(0x0800);
    const name = new TextDecoder().decode(archive.subarray(30, 30 + readU16(archive, 26)));
    expect(name).toBe("ppt/media/图片.png");
  });

  it("is byte-identical for identical input", () => {
    // No timestamps, no ordering from a map: the same deck exported twice must
    // not produce two different files.
    expect(Array.from(createZip(ENTRIES))).toEqual(Array.from(createZip(ENTRIES)));
  });

  it("writes a valid empty archive", () => {
    const archive = createZip([]);
    const end = endOfCentralDirectory(archive);
    expect(end).toBe(0);
    expect(readU16(archive, end + 10)).toBe(0);
  });
});
