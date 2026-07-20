import { describe, expect, it } from "vitest";
import {
  designKeys,
  validateJsonPatchOperations,
  validateRequirementCore,
  validateSlotValues,
} from "./index";

describe("Gallery Native schema helpers", () => {
  it("accepts a valid RequirementCore", () => {
    expect(validateRequirementCore({
      version: "1.0",
      title: "User management",
      pageType: "saas.filter-table-pagination",
      entity: { key: "user", label: "User" },
      fields: [{ key: "name", label: "Name" }],
    }).valid).toBe(true);
  });

  it("rejects unsupported RequirementCore pageType", () => {
    expect(validateRequirementCore({
      version: "1.0",
      title: "Dashboard",
      pageType: "dashboard",
      entity: { key: "dashboard", label: "Dashboard" },
      fields: [],
    }).valid).toBe(false);
  });

  it("accepts object slot values", () => {
    expect(validateSlotValues({ title: "Users", columns: ["name", "status"] }).valid).toBe(true);
  });

  it("rejects layout json patches", () => {
    const result = validateJsonPatchOperations([{ op: "replace", path: "/layers/layer-1/x", value: 10 }]);
    expect(result.valid).toBe(false);
    expect(result.errors[0]).toContain("not allowed");
  });

  it("accepts semantic json patches", () => {
    expect(validateJsonPatchOperations([
      { op: "replace", path: "/layers/layer-1/semantic/role", value: "table" },
      { op: "replace", path: "/componentBindings/layer-1/componentKey", value: "DataTable" },
    ]).valid).toBe(true);
  });

  it("builds workspace-scoped design query keys", () => {
    expect(designKeys.file("ws-1", "file-1")).toEqual(["designs", "ws-1", "files", "file-1"]);
  });
});
