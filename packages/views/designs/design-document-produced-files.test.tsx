import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@multica/core/api", () => ({
  api: {
    getDesignDocumentPreviewFileURL: (base: string, path: string) => `${base}/${path}`,
  },
}));

import type { DesignDocumentRevision } from "@multica/core/types";
import { ProducedFiles } from "./design-document-conversation";

function revisionWith(files: Array<{ path: string; size_bytes: number }>): DesignDocumentRevision {
  return {
    revision_number: 2,
    resource_base_path: "/api/design-document-previews/base",
    files: files.map((file) => ({ ...file, role: "prototype", media_type: "text/html" })),
  } as unknown as DesignDocumentRevision;
}

describe("ProducedFiles", () => {
  it("offers open and download for a file the loaded revision still serves", () => {
    render(
      <ProducedFiles
        paths={["prototype/index.html"]}
        revision={revisionWith([{ path: "prototype/index.html", size_bytes: 61440 }])}
      />,
    );

    expect(screen.getByText("prototype/index.html")).toBeInTheDocument();
    expect(screen.getByText("60.0 KB")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "打开" })).toHaveAttribute(
      "href",
      "/api/design-document-previews/base/prototype/index.html",
    );
    expect(screen.getByRole("link", { name: "下载" })).toHaveAttribute("download", "index.html");
  });

  // A turn whose revision is not the one on screen has no capability URL, so a
  // link would 404. The path is still the honest record of what was written.
  it("lists the path without links when the revision is not loaded", () => {
    render(<ProducedFiles paths={["DESIGN.md"]} revision={undefined} />);

    expect(screen.getByText("DESIGN.md")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  // Written, but absent from the package index — the run wrote a scratch file
  // the package does not carry. Naming it is right; linking it is not.
  it("lists a written path the package index does not carry, without links", () => {
    render(
      <ProducedFiles
        paths={["work/notes.txt"]}
        revision={revisionWith([{ path: "prototype/index.html", size_bytes: 100 }])}
      />,
    );

    expect(screen.getByText("work/notes.txt")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
