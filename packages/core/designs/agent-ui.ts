/**
 * Parser for the UI blocks an agent emits inside its own assistant text —
 * ported from Open Design's `artifacts/question-form.ts` and
 * `contracts/artifacts/od-card.ts`.
 *
 * The contract is deliberately not a tool call. The agent writes a tagged
 * block into the message it is already writing, and the host renders it in
 * place of that slice of prose:
 *
 *   <question-form id="direction" title="先定个方向">
 *   { "questions": [
 *       { "id": "tone", "label": "气质", "type": "radio",
 *         "options": ["克制", "热烈"], "required": true }
 *   ] }
 *   </question-form>
 *
 * Keeping it in the text stream is what makes it work at all here: our agents
 * deliver text through the task-message timeline, and nothing about that
 * pipeline has to change for a form to arrive.
 *
 * `<ask-question>` is accepted as an alias, as upstream does, because models
 * drift to the colloquial tag name and the alternative is raw markup leaking
 * into the user's prose.
 */

/** Controls an agent may ask for. Mirrors Open Design's set exactly. */
export type AgentFormQuestionType =
  | "radio"
  | "checkbox"
  | "select"
  | "text"
  | "textarea"
  | "number"
  | "range"
  | "date"
  | "time"
  | "datetime-local"
  | "color"
  | "url"
  | "email"
  | "tel"
  | "switch"
  | "direction-cards";

const QUESTION_TYPES = new Set<string>([
  "radio", "checkbox", "select", "text", "textarea", "number", "range",
  "date", "time", "datetime-local", "color", "url", "email", "tel",
  "switch", "direction-cards",
]);

/**
 * One option of a finite-choice question. The agent may write a bare string,
 * which becomes both the value sent back and the label shown.
 */
export interface AgentFormOption {
  value: string;
  label: string;
}

/**
 * Rich metadata for a `direction-cards` option: the picker renders a swatch
 * row and a live type sample so a visual direction can be judged by looking
 * at it rather than by reading a radio label. Emitted inline by the agent so
 * the card needs no second fetch.
 */
export interface AgentFormDirectionCard {
  /** Matches an option value; what comes back in the answer. */
  id: string;
  label: string;
  /** One or two sentences of mood. */
  mood: string;
  /** Real-world exemplars. */
  references: string[];
  /** Swatches for the palette row. */
  palette: string[];
  /** Font stacks for the live samples. */
  displayFont: string;
  bodyFont: string;
}

export interface AgentFormQuestion {
  id: string;
  label: string;
  type: AgentFormQuestionType;
  options: AgentFormOption[];
  required: boolean;
  placeholder: string;
  help: string;
  /** Finite-choice questions keep an escape hatch unless explicitly closed. */
  allowCustom: boolean;
  customLabel: string;
  min: number | null;
  max: number | null;
  step: number | null;
  cards: AgentFormDirectionCard[];
}

export interface AgentQuestionForm {
  id: string;
  title: string;
  description: string;
  questions: AgentFormQuestion[];
}

/** Display-only cards. Upstream's `<od-card>` kinds, minus those with no counterpart here. */
export type AgentCardKind = "task-brief" | "memory-applied" | "verify-scorecard" | "rule-proposal";

export interface AgentCard {
  kind: AgentCardKind;
  title: string;
  body: string;
  /** `memory-applied` / `rule-proposal`: the entries or the proposed rule. */
  items: string[];
  /** `verify-scorecard`: one row per rubric line. */
  rows: Array<{ label: string; verdict: string; note: string }>;
  /** `verify-scorecard`: the headline status. */
  status: string;
}

export type AgentUiSegment =
  | { kind: "text"; text: string }
  | { kind: "form"; form: AgentQuestionForm }
  | { kind: "card"; card: AgentCard };

const FORM_TAGS = ["question-form", "ask-question"] as const;
const CARD_TAG = "od-card";

function str(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : typeof value === "number" ? String(value) : fallback;
}

function strList(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => str(item)).filter((item) => item.length > 0) : [];
}

function num(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function normalizeOptions(value: unknown): AgentFormOption[] {
  if (!Array.isArray(value)) return [];
  const options: AgentFormOption[] = [];
  for (const raw of value) {
    if (typeof raw === "string" || typeof raw === "number") {
      const text = String(raw);
      if (text) options.push({ value: text, label: text });
      continue;
    }
    if (raw && typeof raw === "object") {
      const record = raw as Record<string, unknown>;
      // A label-only option answers with its label: the agent gets back the
      // words it wrote, which is what an unlabelled value would have been.
      const label = str(record.label) || str(record.value);
      const value_ = str(record.value) || label;
      if (label) options.push({ value: value_, label });
    }
  }
  return options;
}

function normalizeCards(value: unknown): AgentFormDirectionCard[] {
  if (!Array.isArray(value)) return [];
  const cards: AgentFormDirectionCard[] = [];
  for (const raw of value) {
    if (!raw || typeof raw !== "object") continue;
    const record = raw as Record<string, unknown>;
    const id = str(record.id) || str(record.label);
    if (!id) continue;
    cards.push({
      id,
      label: str(record.label) || id,
      mood: str(record.mood),
      references: strList(record.references),
      palette: strList(record.palette),
      displayFont: str(record.displayFont),
      bodyFont: str(record.bodyFont),
    });
  }
  return cards;
}

function normalizeQuestion(raw: unknown, index: number): AgentFormQuestion | null {
  if (!raw || typeof raw !== "object") return null;
  const record = raw as Record<string, unknown>;
  const label = str(record.label);
  if (!label) return null;
  const declared = str(record.type);
  // An unknown control degrades to a text box rather than dropping the
  // question: the agent still gets an answer, and the user still sees what
  // was asked.
  const type = (QUESTION_TYPES.has(declared) ? declared : "text") as AgentFormQuestionType;
  const options = normalizeOptions(record.options);
  const cards = normalizeCards(record.cards);
  return {
    id: str(record.id) || `q${index + 1}`,
    label,
    type,
    options: options.length > 0 ? options : cards.map((card) => ({ value: card.id, label: card.label })),
    required: record.required === true,
    placeholder: str(record.placeholder),
    help: str(record.help) || str(record.description),
    // Upstream's rule: a finite-choice question keeps its escape hatch unless
    // the agent explicitly closes it.
    allowCustom: record.allowCustom !== false,
    customLabel: str(record.customLabel),
    min: num(record.min),
    max: num(record.max),
    step: num(record.step),
    cards,
  };
}

/** Reads `id="…"` / `title="…"` off an opening tag. */
function attribute(openTag: string, name: string): string {
  const match = openTag.match(new RegExp(`${name}\\s*=\\s*"([^"]*)"`, "i"));
  return match?.[1] ?? "";
}

export function parseQuestionForm(openTag: string, body: string): AgentQuestionForm | null {
  let payload: unknown;
  try {
    payload = JSON.parse(body.trim());
  } catch {
    return null;
  }
  if (!payload || typeof payload !== "object") return null;
  const record = payload as Record<string, unknown>;
  const rawQuestions = Array.isArray(record.questions) ? record.questions : [];
  const questions = rawQuestions
    .map((item, index) => normalizeQuestion(item, index))
    .filter((item): item is AgentFormQuestion => item !== null);
  // A form with nothing to answer is not a form. Returning null leaves the
  // block as prose, which at least shows the user what the agent tried to ask.
  if (questions.length === 0) return null;
  return {
    id: attribute(openTag, "id") || str(record.id) || "form",
    title: attribute(openTag, "title") || str(record.title),
    description: str(record.description),
    questions,
  };
}

export function parseAgentCard(openTag: string, body: string): AgentCard | null {
  let payload: unknown;
  try {
    payload = JSON.parse(body.trim());
  } catch {
    return null;
  }
  if (!payload || typeof payload !== "object") return null;
  const record = payload as Record<string, unknown>;
  const kind = attribute(openTag, "kind") || str(record.kind);
  if (kind !== "task-brief" && kind !== "memory-applied" && kind !== "verify-scorecard" && kind !== "rule-proposal") {
    return null;
  }
  const rows = Array.isArray(record.rows)
    ? record.rows.flatMap((raw) => {
        if (!raw || typeof raw !== "object") return [];
        const row = raw as Record<string, unknown>;
        const label = str(row.label);
        return label ? [{ label, verdict: str(row.verdict), note: str(row.note) }] : [];
      })
    : [];
  return {
    kind,
    title: attribute(openTag, "title") || str(record.title),
    body: str(record.body) || str(record.text),
    items: strList(record.items) || [],
    rows,
    status: str(record.status),
  };
}

/**
 * Splits assistant text into ordered prose / form / card segments so a
 * message renders as what the agent wrote, with the blocks in place.
 *
 * A block whose JSON does not parse stays prose. That matters while a message
 * is still streaming: the closing tag arrives last, so a half-written form is
 * simply text until it is complete, and never renders as a broken control.
 */
export function splitAgentUi(input: string): AgentUiSegment[] {
  const segments: AgentUiSegment[] = [];
  let cursor = 0;

  const pushText = (text: string) => {
    if (text.length > 0) segments.push({ kind: "text", text });
  };

  while (cursor < input.length) {
    const next = nextBlock(input, cursor);
    if (!next) break;
    pushText(input.slice(cursor, next.openStart));
    const parsed = next.tag === CARD_TAG
      ? parseAgentCard(next.openTag, next.body)
      : parseQuestionForm(next.openTag, next.body);
    if (parsed === null) {
      // Unparseable: keep the raw block as prose rather than swallowing it.
      pushText(input.slice(next.openStart, next.blockEnd));
    } else if (next.tag === CARD_TAG) {
      segments.push({ kind: "card", card: parsed as AgentCard });
    } else {
      segments.push({ kind: "form", form: parsed as AgentQuestionForm });
    }
    cursor = next.blockEnd;
  }
  pushText(input.slice(cursor));
  return segments;
}

function nextBlock(input: string, from: number): {
  tag: string;
  openTag: string;
  body: string;
  openStart: number;
  blockEnd: number;
} | null {
  let best: { tag: string; openStart: number } | null = null;
  for (const tag of [...FORM_TAGS, CARD_TAG]) {
    const at = input.indexOf(`<${tag}`, from);
    if (at === -1) continue;
    if (!best || at < best.openStart) best = { tag, openStart: at };
  }
  if (!best) return null;
  const openEnd = input.indexOf(">", best.openStart);
  if (openEnd === -1) return null;
  const close = `</${best.tag}>`;
  const closeStart = input.indexOf(close, openEnd);
  if (closeStart === -1) return null;
  return {
    tag: best.tag,
    openTag: input.slice(best.openStart, openEnd + 1),
    body: input.slice(openEnd + 1, closeStart),
    openStart: best.openStart,
    blockEnd: closeStart + close.length,
  };
}

/** True when the text carries a complete, parseable block worth rendering. */
export function hasAgentUi(input: string): boolean {
  return splitAgentUi(input).some((segment) => segment.kind !== "text");
}

/**
 * Renders answers as the text sent back to the agent. Upstream's shape, kept
 * verbatim so the same header is recognisable on both ends:
 *
 *   [form answers — direction]
 *   - 气质: 克制
 *
 * A question left blank is reported as skipped rather than omitted: the agent
 * asked, and "no answer" is itself an answer it needs in order to proceed.
 */
export function formatAgentFormAnswers(
  form: AgentQuestionForm,
  answers: Record<string, string | string[]>,
): string {
  const lines = [`[form answers — ${form.id}]`];
  for (const question of form.questions) {
    const value = answers[question.id];
    const display = Array.isArray(value)
      ? value.map((item) => optionLabel(question, item)).filter(Boolean).join("、")
      : optionLabel(question, (value ?? "").trim());
    lines.push(`- ${question.label}: ${display || "(未选择)"}`);
  }
  return lines.join("\n");
}

function optionLabel(question: AgentFormQuestion, value: string): string {
  if (!value) return "";
  const match = question.options.find((option) => option.value === value || option.label === value);
  return match ? match.label : value;
}
