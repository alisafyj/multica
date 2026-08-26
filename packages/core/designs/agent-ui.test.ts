// @vitest-environment node
// @canonical agent-emitted UI block parsing. The renderer suite mounts the
// happy path and the wiring only; every shape lives here.
import { describe, expect, it } from "vitest";

import {
  formatAgentFormAnswers,
  hasAgentUi,
  parseQuestionForm,
  splitAgentUi,
  type AgentQuestionForm,
} from "./agent-ui";

const FORM_JSON = JSON.stringify({
  questions: [
    { id: "tone", label: "气质", type: "radio", options: ["克制", "热烈"], required: true },
    { id: "audience", label: "受众", type: "text", placeholder: "例如 SaaS 采购方" },
  ],
});

function block(inner: string, tag = "question-form", attrs = ' id="direction" title="先定个方向"') {
  return `<${tag}${attrs}>\n${inner}\n</${tag}>`;
}

describe("splitAgentUi", () => {
  it("keeps the agent's prose around the block, in order", () => {
    const segments = splitAgentUi(`开始之前先确认。\n${block(FORM_JSON)}\n确认后我就开工。`);
    expect(segments.map((segment) => segment.kind)).toEqual(["text", "form", "text"]);
    expect(segments[0]).toMatchObject({ text: expect.stringContaining("开始之前") });
    expect(segments[2]).toMatchObject({ text: expect.stringContaining("我就开工") });
  });

  it("accepts ask-question as an alias, as upstream does", () => {
    const segments = splitAgentUi(block(FORM_JSON, "ask-question", ' id="d"'));
    expect(segments).toHaveLength(1);
    expect(segments[0]?.kind).toBe("form");
  });

  // The closing tag arrives last while a message streams, so a half-written
  // form has to read as text — never as a broken control.
  it("leaves an unterminated block as prose", () => {
    const partial = `先确认方向：\n<question-form id="direction">\n{ "questions": [`;
    const segments = splitAgentUi(partial);
    expect(segments).toEqual([{ kind: "text", text: partial }]);
    expect(hasAgentUi(partial)).toBe(false);
  });

  it("keeps a complete block whose JSON is broken as prose rather than swallowing it", () => {
    const broken = block("{ not json");
    const segments = splitAgentUi(broken);
    expect(segments).toEqual([{ kind: "text", text: broken }]);
  });

  it("renders several blocks in one message", () => {
    const text = `${block(FORM_JSON)}\n中间说明\n${block(FORM_JSON, "question-form", ' id="second"')}`;
    expect(splitAgentUi(text).map((segment) => segment.kind)).toEqual(["form", "text", "form"]);
  });

  it("parses a display-only card", () => {
    const card = `<od-card kind="verify-scorecard" title="自检">${JSON.stringify({
      status: "pass",
      rows: [{ label: "对比度", verdict: "pass", note: "AA" }, { label: "无标签", verdict: "fail" }],
    })}</od-card>`;
    const segments = splitAgentUi(card);
    expect(segments[0]).toMatchObject({
      kind: "card",
      card: { kind: "verify-scorecard", title: "自检", status: "pass" },
    });
    expect(segments[0]).toMatchObject({ card: { rows: [{ label: "对比度", verdict: "pass", note: "AA" }, { label: "无标签", verdict: "fail", note: "" }] } });
  });

  it("leaves a card of an unknown kind as prose", () => {
    const unknown = `<od-card kind="not-a-kind">{}</od-card>`;
    expect(splitAgentUi(unknown)).toEqual([{ kind: "text", text: unknown }]);
  });
});

describe("parseQuestionForm", () => {
  it("takes id and title from the tag, and normalises bare string options", () => {
    const form = parseQuestionForm('<question-form id="direction" title="先定个方向">', FORM_JSON);
    expect(form).toMatchObject({ id: "direction", title: "先定个方向" });
    expect(form?.questions[0]?.options).toEqual([
      { value: "克制", label: "克制" },
      { value: "热烈", label: "热烈" },
    ]);
  });

  it("keeps an option's value and label apart when the agent gives both", () => {
    const form = parseQuestionForm("<question-form>", JSON.stringify({
      questions: [{ label: "平台", type: "select", options: [{ value: "web", label: "桌面网页" }] }],
    }));
    expect(form?.questions[0]?.options).toEqual([{ value: "web", label: "桌面网页" }]);
  });

  // A control we do not know still has to reach the user: dropping the
  // question would leave the agent waiting on an answer nobody was asked for.
  it("degrades an unknown control to a text box instead of dropping the question", () => {
    const form = parseQuestionForm("<question-form>", JSON.stringify({
      questions: [{ id: "x", label: "某项", type: "hologram" }],
    }));
    expect(form?.questions[0]).toMatchObject({ id: "x", label: "某项", type: "text" });
  });

  it("leaves the escape hatch open unless the agent explicitly closes it", () => {
    const open = parseQuestionForm("<question-form>", JSON.stringify({
      questions: [{ label: "气质", type: "radio", options: ["A"] }],
    }));
    const closed = parseQuestionForm("<question-form>", JSON.stringify({
      questions: [{ label: "气质", type: "radio", options: ["A"], allowCustom: false }],
    }));
    expect(open?.questions[0]?.allowCustom).toBe(true);
    expect(closed?.questions[0]?.allowCustom).toBe(false);
  });

  it("carries direction-card metadata and derives its options", () => {
    const form = parseQuestionForm("<question-form>", JSON.stringify({
      questions: [{
        id: "direction", label: "视觉方向", type: "direction-cards",
        cards: [{
          id: "editorial", label: "编辑风", mood: "克制的杂志感",
          references: ["Monocle"], palette: ["#111", "#eee"],
          displayFont: "serif", bodyFont: "sans-serif",
        }],
      }],
    }));
    expect(form?.questions[0]?.cards[0]).toMatchObject({
      id: "editorial", palette: ["#111", "#eee"], displayFont: "serif",
    });
    // Options come from the cards so the answer path is identical to a radio.
    expect(form?.questions[0]?.options).toEqual([{ value: "editorial", label: "编辑风" }]);
  });

  it("rejects a form with nothing answerable", () => {
    expect(parseQuestionForm("<question-form>", JSON.stringify({ questions: [] }))).toBeNull();
    expect(parseQuestionForm("<question-form>", JSON.stringify({ questions: [{ type: "text" }] }))).toBeNull();
  });
});

describe("formatAgentFormAnswers", () => {
  const form = parseQuestionForm('<question-form id="direction">', FORM_JSON) as AgentQuestionForm;

  it("echoes the labels the agent wrote, under a header it can recognise", () => {
    expect(formatAgentFormAnswers(form, { tone: "克制", audience: "SaaS 采购方" })).toBe(
      ["[form answers — direction]", "- 气质: 克制", "- 受众: SaaS 采购方"].join("\n"),
    );
  });

  it("reports a blank answer as skipped rather than dropping the line", () => {
    expect(formatAgentFormAnswers(form, { tone: "热烈" })).toContain("- 受众: (未选择)");
  });

  it("joins a multi-select and resolves values back to their labels", () => {
    const multi = parseQuestionForm("<question-form>", JSON.stringify({
      questions: [{ id: "surface", label: "平台", type: "checkbox", options: [
        { value: "web", label: "桌面网页" }, { value: "app", label: "移动应用" },
      ] }],
    })) as AgentQuestionForm;
    expect(formatAgentFormAnswers(multi, { surface: ["web", "app"] })).toContain("- 平台: 桌面网页、移动应用");
  });
});
