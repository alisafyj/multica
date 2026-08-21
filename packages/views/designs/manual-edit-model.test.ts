// @vitest-environment node

// Canonical matrix for the pending edit set. The panel's own suite only checks
// that editing a control reaches these helpers.
import { describe, expect, it } from "vitest";
import {
  computedColorToHex,
  countDeclarations,
  declarationOf,
  EDITABLE_PROPERTIES,
  submittableEdits,
  withDeclaration,
  withoutSelector,
  type ManualEdit,
} from "./manual-edit-model";

const PAGE = "prototype/index.html";

describe("withDeclaration", () => {
  it("starts a set, then merges into the same element", () => {
    let edits: ManualEdit[] = [];
    edits = withDeclaration(edits, PAGE, "#go", "color", "#fff");
    edits = withDeclaration(edits, PAGE, "#go", "font-size", "14px");
    expect(edits).toHaveLength(1);
    expect(edits[0]!.declarations).toEqual({ color: "#fff", "font-size": "14px" });
  });

  it("keeps elements and pages apart", () => {
    let edits = withDeclaration([], PAGE, "#go", "color", "#fff");
    edits = withDeclaration(edits, PAGE, "#other", "color", "#000");
    edits = withDeclaration(edits, "prototype/orders.html", "#go", "color", "#111");
    expect(edits).toHaveLength(3);
    expect(declarationOf(edits, PAGE, "#go", "color")).toBe("#fff");
    expect(declarationOf(edits, "prototype/orders.html", "#go", "color")).toBe("#111");
  });

  it("does not mutate the set it was given", () => {
    const original = withDeclaration([], PAGE, "#go", "color", "#fff");
    const snapshot = JSON.stringify(original);
    withDeclaration(original, PAGE, "#go", "color", "#000");
    expect(JSON.stringify(original)).toBe(snapshot);
  });

  it("records clearing an override, because the base may have set it", () => {
    const edits = withDeclaration(withDeclaration([], PAGE, "#go", "color", "#fff"), PAGE, "#go", "color", "");
    expect(declarationOf(edits, PAGE, "#go", "color")).toBe("");
    // Still submitted: the run has to know the property was cleared.
    expect(submittableEdits(edits)[0]!.declarations).toEqual({ color: "" });
  });
});

describe("withoutSelector", () => {
  it("drops one element's overrides and leaves the rest", () => {
    let edits = withDeclaration([], PAGE, "#go", "color", "#fff");
    edits = withDeclaration(edits, PAGE, "#other", "color", "#000");
    const remaining = withoutSelector(edits, PAGE, "#go");
    expect(remaining).toHaveLength(1);
    expect(remaining[0]!.selector).toBe("#other");
  });
});

describe("submittableEdits", () => {
  it("drops properties the panel cannot produce", () => {
    // A property outside the panel would be refused by the server anyway;
    // filtering here means a stale client cannot fail a whole edit set.
    const edits: ManualEdit[] = [{ page: PAGE, selector: "#go", declarations: { color: "#fff", behavior: "url(x)" } }];
    expect(submittableEdits(edits)[0]!.declarations).toEqual({ color: "#fff" });
  });

  it("drops an element left with nothing to change", () => {
    const edits: ManualEdit[] = [{ page: PAGE, selector: "#go", declarations: { behavior: "url(x)" } }];
    expect(submittableEdits(edits)).toEqual([]);
  });

  it("counts what would actually be written", () => {
    let edits = withDeclaration([], PAGE, "#go", "color", "#fff");
    edits = withDeclaration(edits, PAGE, "#go", "gap", "8px");
    edits = withDeclaration(edits, PAGE, "#other", "width", "50%");
    expect(countDeclarations(edits)).toBe(3);
    expect(countDeclarations([])).toBe(0);
  });
});

describe("EDITABLE_PROPERTIES", () => {
  it("has no duplicates, so a property cannot appear twice in the panel", () => {
    expect(new Set(EDITABLE_PROPERTIES).size).toBe(EDITABLE_PROPERTIES.length);
  });
});

describe("computedColorToHex", () => {
  it("reads the formats a browser reports", () => {
    expect(computedColorToHex("rgb(255, 87, 1)")).toBe("#ff5701");
    expect(computedColorToHex("rgba(255, 87, 1, 1)")).toBe("#ff5701");
    expect(computedColorToHex("rgb(255 87 1)")).toBe("#ff5701");
    expect(computedColorToHex("#FF5701")).toBe("#ff5701");
    expect(computedColorToHex("#f50")).toBe("#ff5500");
  });

  it("gives up rather than showing a confidently wrong swatch", () => {
    // Transparency cannot be shown in a hex swatch, and a colour this cannot
    // read must not be rendered as black.
    expect(computedColorToHex("rgba(255, 87, 1, 0.5)")).toBe("");
    expect(computedColorToHex("transparent")).toBe("");
    expect(computedColorToHex("color-mix(in srgb, red, blue)")).toBe("");
    expect(computedColorToHex("")).toBe("");
  });

  it("clamps and rounds channels rather than emitting an invalid hex", () => {
    expect(computedColorToHex("rgb(300, 20, 1)")).toBe("#ff1401");
    expect(computedColorToHex("rgb(255.6, 86.5, 0.4)")).toBe("#ff5700");
    // A negative channel is not something a computed style produces; it does
    // not parse, and an unparseable colour shows no swatch.
    expect(computedColorToHex("rgb(300, -20, 1)")).toBe("");
  });
});
