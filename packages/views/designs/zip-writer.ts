/**
 * A minimal ZIP writer, because a .pptx is a ZIP and the workspace has no zip
 * dependency.
 *
 * Everything is stored uncompressed (method 0). A slide deck is mostly PNG
 * bytes, which do not compress further, so deflating would cost code and
 * correctness risk to save almost nothing — and an incorrect deflate stream
 * produces a file PowerPoint silently refuses to open, which is the worst
 * possible failure for an export.
 *
 * The format is fixed and unforgiving, so the byte layout is asserted in
 * zip-writer.test.ts rather than trusted.
 */

export interface ZipEntry {
  /** Path inside the archive, forward slashes, no leading slash. */
  path: string;
  data: Uint8Array;
}

/** CRC-32 (IEEE), which the ZIP central directory requires per entry. */
const CRC_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let index = 0; index < 256; index += 1) {
    let value = index;
    for (let bit = 0; bit < 8; bit += 1) {
      value = value & 1 ? 0xedb88320 ^ (value >>> 1) : value >>> 1;
    }
    table[index] = value >>> 0;
  }
  return table;
})();

export function crc32(data: Uint8Array): number {
  let crc = 0xffffffff;
  for (let index = 0; index < data.length; index += 1) {
    crc = CRC_TABLE[(crc ^ data[index]!) & 0xff]! ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

class ByteWriter {
  private chunks: Uint8Array[] = [];
  length = 0;

  bytes(value: Uint8Array): void {
    this.chunks.push(value);
    this.length += value.length;
  }

  u16(value: number): void {
    this.bytes(new Uint8Array([value & 0xff, (value >>> 8) & 0xff]));
  }

  u32(value: number): void {
    this.bytes(new Uint8Array([value & 0xff, (value >>> 8) & 0xff, (value >>> 16) & 0xff, (value >>> 24) & 0xff]));
  }

  concat(): Uint8Array {
    const result = new Uint8Array(this.length);
    let offset = 0;
    for (const chunk of this.chunks) {
      result.set(chunk, offset);
      offset += chunk.length;
    }
    return result;
  }
}

export function utf8(value: string): Uint8Array {
  return new TextEncoder().encode(value);
}

/**
 * Builds the archive. Entry order is preserved, which matters for OOXML:
 * `[Content_Types].xml` has to come first or some readers reject the package.
 */
export function createZip(entries: ReadonlyArray<ZipEntry>): Uint8Array {
  const output = new ByteWriter();
  const directory: Array<{ name: Uint8Array; crc: number; size: number; offset: number }> = [];

  for (const entry of entries) {
    const name = utf8(entry.path);
    const crc = crc32(entry.data);
    const offset = output.length;

    output.u32(0x04034b50); // local file header
    output.u16(20); // version needed: 2.0
    output.u16(0x0800); // flags: names are UTF-8
    output.u16(0); // method: stored
    output.u16(0); // modification time
    output.u16(0x0021); // modification date: a fixed 1980-01-01, so the
    // archive is byte-identical for identical input.
    output.u32(crc);
    output.u32(entry.data.length);
    output.u32(entry.data.length);
    output.u16(name.length);
    output.u16(0); // extra field length
    output.bytes(name);
    output.bytes(entry.data);

    directory.push({ name, crc, size: entry.data.length, offset });
  }

  const directoryOffset = output.length;
  for (const entry of directory) {
    output.u32(0x02014b50); // central directory header
    output.u16(20); // version made by
    output.u16(20); // version needed
    output.u16(0x0800);
    output.u16(0);
    output.u16(0);
    output.u16(0x0021);
    output.u32(entry.crc);
    output.u32(entry.size);
    output.u32(entry.size);
    output.u16(entry.name.length);
    output.u16(0); // extra
    output.u16(0); // comment
    output.u16(0); // disk number
    output.u16(0); // internal attributes
    output.u32(0); // external attributes
    output.u32(entry.offset);
    output.bytes(entry.name);
  }
  const directorySize = output.length - directoryOffset;

  output.u32(0x06054b50); // end of central directory
  output.u16(0); // this disk
  output.u16(0); // disk with the directory
  output.u16(directory.length);
  output.u16(directory.length);
  output.u32(directorySize);
  output.u32(directoryOffset);
  output.u16(0); // comment length

  return output.concat();
}
