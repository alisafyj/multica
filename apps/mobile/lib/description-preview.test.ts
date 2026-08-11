/**
 * Mirrors the cases web's board card relies on
 * (`packages/views/issues/components/board-card.tsx` `descriptionPreview`).
 * The card renders this as one clamped line, so anything that survives the
 * strip — a raw image URL, a stray `#`, an unwrapped newline — shows up as
 * visible junk in the card.
 */
import { describe, expect, it } from "vitest";
import { descriptionPreview } from "./description-preview";

describe("descriptionPreview", () => {
  it("drops file embeds entirely", () => {
    expect(descriptionPreview("before !file[report.pdf](mc://a/b) after")).toBe(
      "before after",
    );
  });

  it("drops image embeds entirely", () => {
    expect(descriptionPreview("look ![alt text](https://x/y.png) here")).toBe(
      "look here",
    );
  });

  it("keeps link text and drops the URL", () => {
    expect(descriptionPreview("see [the RFC](https://example.test/rfc)")).toBe(
      "see the RFC",
    );
  });

  it("strips emphasis, code and strikethrough markers", () => {
    expect(descriptionPreview("**bold** _em_ `code` ~~gone~~")).toBe(
      "bold em code gone",
    );
  });

  it("strips heading and blockquote markers on every line", () => {
    expect(descriptionPreview("# Title\n> quoted\nbody")).toBe(
      "Title quoted body",
    );
  });

  it("collapses newlines and runs of whitespace into single spaces", () => {
    expect(descriptionPreview("a\n\n\nb   c\td")).toBe("a b c d");
  });

  it("trims and survives an empty or whitespace-only description", () => {
    expect(descriptionPreview("   \n  ")).toBe("");
    expect(descriptionPreview("")).toBe("");
  });
});
