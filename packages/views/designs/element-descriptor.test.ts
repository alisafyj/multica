// Needs a DOM: every helper here reads a live element. This is the canonical
// matrix; the canvas suite only checks that a click reaches describeElement.
import { beforeEach, describe, expect, it } from "vitest";
import { describeElement, elementSelector, elementText } from "./element-descriptor";

function parse(html: string): Document {
  return new DOMParser().parseFromString(`<!doctype html><html><body>${html}</body></html>`, "text/html");
}

let doc: Document;

beforeEach(() => {
  doc = parse(`
    <main class="workspace" data-page="page.orders">
      <header data-block="block.orders.toolbar">
        <button id="open-filters" type="button">保存筛选</button>
        <button type="button">导出</button>
      </header>
      <section>
        <p>第一段</p>
        <p>第二段</p>
      </section>
      <span id="dup"></span><span id="dup"></span>
    </main>
  `);
});

function find(selector: string): Element {
  const element = doc.querySelector(selector);
  if (!element) throw new Error(`fixture has no ${selector}`);
  return element;
}

describe("elementSelector", () => {
  it("prefers a unique id", () => {
    expect(elementSelector(find("#open-filters"))).toBe("#open-filters");
  });

  it("falls back to the package's own semantic handle", () => {
    expect(elementSelector(find("header"))).toBe('[data-block="block.orders.toolbar"]');
    expect(elementSelector(find("main"))).toBe('[data-page="page.orders"]');
  });

  it("builds a structural path anchored on the nearest named ancestor", () => {
    // The second <p> has no name of its own; the path stops at the page handle.
    const selector = elementSelector(find("section p:nth-of-type(2)"));
    expect(selector).toBe('[data-page="page.orders"] > section:nth-of-type(1) > p:nth-of-type(2)');
    expect(doc.querySelectorAll(selector)).toHaveLength(1);
    expect(doc.querySelector(selector)?.textContent).toBe("第二段");
  });

  it("resolves back to the element it described", () => {
    for (const element of [find("#open-filters"), find("header"), find("section p:nth-of-type(1)")]) {
      expect(doc.querySelector(elementSelector(element))).toBe(element);
    }
  });

  it("refuses a duplicated id, which would name two elements", () => {
    const selector = elementSelector(find("#dup"));
    expect(selector).not.toBe("#dup");
    expect(doc.querySelectorAll(selector)).toHaveLength(1);
  });
});

describe("elementText", () => {
  it("collapses whitespace and caps the excerpt", () => {
    const element = parse("<p>  多行\n  文本  </p>").querySelector("p")!;
    expect(elementText(element)).toBe("多行 文本");

    const long = parse(`<p>${"字".repeat(120)}</p>`).querySelector("p")!;
    const text = elementText(long);
    expect(text.endsWith("…")).toBe(true);
    expect(text.length).toBe(61);
  });
});

describe("describeElement", () => {
  it("labels by kind plus the handle the markup carried", () => {
    expect(describeElement(find("header"))).toMatchObject({
      handle: "block.orders.toolbar",
      label: "页头 · block.orders.toolbar",
      tag: "header",
    });
  });

  it("labels by the text the user clicked when there is no handle", () => {
    expect(describeElement(find("#open-filters"))).toMatchObject({
      handle: "",
      label: "按钮 · 保存筛选",
      selector: "#open-filters",
      text: "保存筛选",
    });
  });

  it("still names an element with neither handle nor text", () => {
    const empty = parse("<div><span></span></div>").querySelector("span")!;
    expect(describeElement(empty).label).toBe("span");
  });
});
