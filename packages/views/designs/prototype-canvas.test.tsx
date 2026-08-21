import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@multica/core/api", () => ({
  api: { getDesignDocumentPreviewFileURL: (base: string, path: string) => `https://api.test${base}/${path}` },
}));

import { elementsInRegion, PrototypeCanvas } from "./prototype-canvas";

let objectUrlSeq = 0;
// Saved and put back by hand: these are direct property assignments, and
// vi.restoreAllMocks() only restores spies. Leaving a stubbed
// URL.createObjectURL behind breaks every later suite that makes a blob URL.
const realCreateObjectURL = URL.createObjectURL;
const realRevokeObjectURL = URL.revokeObjectURL;

beforeEach(() => {
  // jsdom has no object-URL support. Each call returns a distinct URL, as the
  // real one does — a stub returning a constant would hide the revoke.
  objectUrlSeq = 0;
  URL.createObjectURL = vi.fn(() => `blob:test-document-${(objectUrlSeq += 1)}`);
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  URL.createObjectURL = realCreateObjectURL;
  URL.revokeObjectURL = realRevokeObjectURL;
  vi.restoreAllMocks();
});

describe("PrototypeCanvas", () => {
  // The canvas mounts an agent-written page from a blob: URL, which inherits
  // THIS app's origin. That is what gives the workbench DOM access — and it is
  // exactly why the frame must never be granted allow-scripts: the package's
  // own code would then execute on our origin, against our storage. The live
  // preview frame is the other half of that trade and runs scripts on an
  // opaque origin instead.
  it("never lets the inlined package run scripts on our origin", () => {
    render(
      <PrototypeCanvas
        html="<!doctype html><html><body>hi</body></html>"
        frameWidth={1280}
        zoom={1}
        mode="select"
        title="订单总览 · 首页"
      />,
    );

    const frame = screen.getByTitle("订单总览 · 首页");
    expect(frame).toHaveAttribute("sandbox", "allow-same-origin");
    expect(frame.getAttribute("sandbox")).not.toContain("allow-scripts");
    expect(frame.getAttribute("src")).toMatch(/^blob:test-document-/);
  });

  it("releases the object URL when the document changes", () => {
    const { rerender, unmount } = render(
      <PrototypeCanvas html="<html><body>a</body></html>" frameWidth={null} zoom={1} mode={null} title="画布" />,
    );
    rerender(<PrototypeCanvas html="<html><body>b</body></html>" frameWidth={null} zoom={1} mode={null} title="画布" />);
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(1);
    unmount();
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(2);
  });
});

describe("elementsInRegion", () => {
  function documentWith(boxes: Record<string, { x: number; y: number; width: number; height: number }>): Document {
    const parsed = new DOMParser().parseFromString(
      `<!doctype html><html><body>
        <section id="outer"><p id="inner">文字</p></section>
        <aside id="aside"></aside>
      </body></html>`,
      "text/html",
    );
    for (const [id, box] of Object.entries(boxes)) {
      const element = parsed.getElementById(id)!;
      element.getBoundingClientRect = () => ({
        x: box.x, y: box.y, width: box.width, height: box.height,
        left: box.x, top: box.y, right: box.x + box.width, bottom: box.y + box.height,
        toJSON: () => ({}),
      }) as DOMRect;
    }
    return parsed;
  }

  it("reports the outermost fully covered element, not its children", () => {
    const parsed = documentWith({
      outer: { x: 10, y: 10, width: 100, height: 50 },
      inner: { x: 20, y: 20, width: 40, height: 20 },
      aside: { x: 400, y: 400, width: 50, height: 50 },
    });

    const found = elementsInRegion(parsed, { x: 0, y: 0, width: 200, height: 200 });
    // #inner is inside #outer, so naming #outer already describes it; #aside
    // sits outside the marquee entirely.
    expect(found.map((entry) => entry.selector)).toEqual(["#outer"]);
  });

  it("ignores an element the marquee only clips", () => {
    const parsed = documentWith({
      outer: { x: 10, y: 10, width: 100, height: 50 },
      inner: { x: 20, y: 20, width: 40, height: 20 },
      aside: { x: 0, y: 0, width: 0, height: 0 },
    });

    // The rectangle cuts #outer in half: partially covered is context, not a
    // pick, so only the child it fully contains comes back.
    const found = elementsInRegion(parsed, { x: 15, y: 15, width: 60, height: 30 });
    expect(found.map((entry) => entry.selector)).toEqual(["#inner"]);
  });

  it("caps how many elements one mark can name", () => {
    const parsed = documentWith({
      outer: { x: 10, y: 10, width: 100, height: 50 },
      inner: { x: 20, y: 20, width: 40, height: 20 },
      aside: { x: 10, y: 100, width: 20, height: 20 },
    });
    expect(elementsInRegion(parsed, { x: 0, y: 0, width: 500, height: 500 }, 1)).toHaveLength(1);
  });
});
