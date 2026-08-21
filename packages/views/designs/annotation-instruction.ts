import type { ElementDescriptor } from "./element-descriptor";

/**
 * Turning what the user marked on the canvas into an instruction the agent can
 * act on.
 *
 * A marked element or region is worth more than a screenshot only if the
 * agent can find the same thing in the source. So every annotation carries the
 * page it was made on and the selector the pick resolved to, and the note the
 * user typed is attached to that anchor rather than left to describe itself.
 * The wording is assembled here, deterministically, so the instruction the
 * agent receives is reviewable in a test instead of improvised in a component.
 */

export interface AnnotationRegion {
  x: number;
  y: number;
  width: number;
  height: number;
  elements: ElementDescriptor[];
}

export interface Annotation {
  id: string;
  /** Package path of the page the mark was made on. */
  pagePath: string;
  /** The page's title in the workbench, for a readable instruction. */
  pageTitle: string;
  /** Set for a picked element. */
  element?: ElementDescriptor;
  /** Set for a marked rectangle. */
  region?: AnnotationRegion;
  /** What the user wants changed there. */
  note: string;
}

function round(value: number): number {
  return Math.round(value);
}

function anchorOf(annotation: Annotation): string {
  if (annotation.element) {
    return `${annotation.element.label}（选择器 \`${annotation.element.selector}\`）`;
  }
  const region = annotation.region;
  if (!region) return "整页";
  const box = `矩形区域 x=${round(region.x)} y=${round(region.y)} 宽=${round(region.width)} 高=${round(region.height)}`;
  if (region.elements.length === 0) return box;
  const covered = region.elements.map((element) => `\`${element.selector}\``).join("、");
  return `${box}，覆盖 ${covered}`;
}

/**
 * Groups the annotations by page and writes them out as a numbered brief.
 * `summary` is the user's overall ask; annotations without a note still carry
 * their anchor, because "this element" is itself the message.
 */
export function annotationInstruction(annotations: Annotation[], summary: string): string {
  const trimmedSummary = summary.trim();
  if (annotations.length === 0) return trimmedSummary;

  const pages: Array<{ path: string; title: string; items: Annotation[] }> = [];
  for (const annotation of annotations) {
    const existing = pages.find((page) => page.path === annotation.pagePath);
    if (existing) existing.items.push(annotation);
    else pages.push({ path: annotation.pagePath, title: annotation.pageTitle, items: [annotation] });
  }

  const lines: string[] = [];
  lines.push(trimmedSummary || "按下面标注的位置调整这份设计稿。");
  for (const page of pages) {
    lines.push("");
    lines.push(`页面 ${page.title}（${page.path}）：`);
    page.items.forEach((annotation, index) => {
      const note = annotation.note.trim();
      lines.push(`${index + 1}. ${anchorOf(annotation)}${note ? `：${note}` : "：这里需要调整"}`);
    });
  }
  lines.push("");
  lines.push("选择器与坐标来自当前版本的静态渲染，用来定位；请在源文件中找到对应元素后修改，不要只覆盖行内样式。");
  return lines.join("\n");
}

/** The one-line summary the annotation list shows for an anchor. */
export function annotationLabel(annotation: Annotation): string {
  if (annotation.element) return annotation.element.label;
  const region = annotation.region;
  if (!region) return "整页";
  if (region.elements.length > 0) return `区域 · ${region.elements[0]!.label}`;
  return `区域 · ${round(region.width)}×${round(region.height)}`;
}
