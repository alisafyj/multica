"use client";

/**
 * Naming a node the user pointed at, so the workbench and the agent can talk
 * about the same element.
 *
 * A click gives us a DOM node; an instruction to an agent, or a manual edit
 * that has to survive a round trip, needs something written down. Two things
 * come out of a pick: a selector that finds the node again in the same
 * document, and a label a person recognises. Prototype markup often carries a
 * semantic handle (`data-block`, `data-page`, `data-state`, an `id`) — those
 * are preferred over a structural path, because they are what the agent that
 * wrote the page also calls the element.
 */

export interface ElementDescriptor {
  /** A CSS selector that resolves back to this element in its document. */
  selector: string;
  /** What the picker shows the user, e.g. `按钮 · 保存筛选`. */
  label: string;
  /** The package's own name for this element, when the markup carries one. */
  handle: string;
  /** A short excerpt of the element's own text. */
  text: string;
  /** Tag name, lower case. */
  tag: string;
}

/** Attributes the prototype markup uses to name a region, best first. */
const HANDLE_ATTRIBUTES = ["data-block", "data-page", "data-state", "data-flow", "data-testid"];

const MAX_TEXT = 60;

function escapeIdentifier(value: string): string {
  const escape = (globalThis as { CSS?: { escape?: (value: string) => string } }).CSS?.escape;
  if (typeof escape === "function") return escape(value);
  // Conservative fallback: escape anything outside the safe identifier set.
  return value.replace(/[^a-zA-Z0-9_-]/g, (character) => `\\${character}`);
}

/** True when `id` is usable as a selector on its own in this document. */
function idIsUnique(element: Element, id: string): boolean {
  if (id === "" || /\s/.test(id)) return false;
  try {
    return element.ownerDocument.querySelectorAll(`#${escapeIdentifier(id)}`).length === 1;
  } catch {
    return false;
  }
}

function attributeSelector(element: Element): string {
  for (const attribute of HANDLE_ATTRIBUTES) {
    const value = element.getAttribute(attribute);
    if (!value || /["\\]/.test(value)) continue;
    const selector = `[${attribute}="${value}"]`;
    try {
      if (element.ownerDocument.querySelectorAll(selector).length === 1) return selector;
    } catch {
      continue;
    }
  }
  return "";
}

/** Position among siblings of the same tag, 1-based, as :nth-of-type wants. */
function nthOfType(element: Element): number {
  let index = 1;
  let sibling = element.previousElementSibling;
  while (sibling) {
    if (sibling.tagName === element.tagName) index += 1;
    sibling = sibling.previousElementSibling;
  }
  return index;
}

/**
 * Builds a selector for `element`, stopping as soon as an ancestor anchors it
 * uniquely. The walk is bounded by the document body, so the result is always
 * resolvable from the document root.
 */
export function elementSelector(element: Element): string {
  const id = element.getAttribute("id") ?? "";
  if (idIsUnique(element, id)) return `#${escapeIdentifier(id)}`;
  const handle = attributeSelector(element);
  if (handle !== "") return handle;

  const parts: string[] = [];
  let current: Element | null = element;
  while (current && current.tagName.toLowerCase() !== "html") {
    const currentId = current.getAttribute("id") ?? "";
    if (idIsUnique(current, currentId)) {
      parts.unshift(`#${escapeIdentifier(currentId)}`);
      break;
    }
    const currentHandle = attributeSelector(current);
    if (currentHandle !== "") {
      parts.unshift(currentHandle);
      break;
    }
    const tag = current.tagName.toLowerCase();
    parts.unshift(current.parentElement ? `${tag}:nth-of-type(${nthOfType(current)})` : tag);
    current = current.parentElement;
  }
  return parts.join(" > ");
}

/** Trimmed, single-line, capped text of the element itself. */
export function elementText(element: Element): string {
  const text = (element.textContent ?? "").replace(/\s+/g, " ").trim();
  return text.length > MAX_TEXT ? `${text.slice(0, MAX_TEXT)}…` : text;
}

const TAG_LABELS: Record<string, string> = {
  a: "链接",
  button: "按钮",
  img: "图片",
  input: "输入框",
  textarea: "文本域",
  select: "下拉框",
  table: "表格",
  ul: "列表",
  ol: "列表",
  li: "列表项",
  form: "表单",
  header: "页头",
  footer: "页脚",
  nav: "导航",
  main: "主区域",
  aside: "侧栏",
  section: "区块",
  h1: "标题",
  h2: "标题",
  h3: "标题",
  h4: "标题",
  p: "段落",
  svg: "图形",
};

export function describeElement(element: Element): ElementDescriptor {
  const tag = element.tagName.toLowerCase();
  const handle = HANDLE_ATTRIBUTES.map((attribute) => element.getAttribute(attribute) ?? "").find((value) => value !== "") ?? "";
  const text = elementText(element);
  const kind = TAG_LABELS[tag] ?? tag;
  // The handle names the element better than its text when the markup has
  // one; otherwise the text is what the user just clicked on and recognises.
  const detail = handle || text;
  return {
    selector: elementSelector(element),
    label: detail ? `${kind} · ${detail}` : kind,
    handle,
    text,
    tag,
  };
}
