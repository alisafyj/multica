// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CommentDesignRequest, Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { IssueDesignRestoreSection } from "./issue-design-restore-section";

const buildPrompt = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", () => ({ api: { buildDesignImplementationPrompt: buildPrompt } }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/designs/queries", () => ({
  designFileListOptions: () => ({ queryKey: ["files"], queryFn: async () => [{ id: "file-1", title: "Figma checkout", project_id: "project-1", design_ref: "figma:checkout", current_revision_id: "revision-1", updated_at: "2026-09-08" }] }),
  designDocumentListOptions: () => ({ queryKey: ["documents"], queryFn: async () => [] }),
  designAssetFramesOptions: (_wsId: string, ref: string) => ({ queryKey: ["frames", ref], enabled: !!ref, queryFn: async () => ({ frames: [{ frame_ref: "figma:1:2", selection_key: "page/checkout", title: "Checkout" }] }) }),
}));
afterEach(() => { cleanup(); buildPrompt.mockReset(); });

describe("comment-owned UI restore", () => {
  it("prepares exact saved refs without executing or reassigning an issue", async () => {
    buildPrompt.mockResolvedValue({ prompt: "Call multica_design_get_implementation_context with {}", context: { design_ref: "figma:checkout", revision_id: "revision-1", content_digest: "sha256:fixed" } });
    const prepared = vi.fn();
    function Harness() {
      const [request, setRequest] = useState<CommentDesignRequest>({ request_id: "request-1", operation: "implement", agent_id: "chosen-agent", project_resource_id: "repository-1" });
      return <IssueDesignRestoreSection issue={{ id: "issue-1", title: "Ordinary issue", project_id: "project-1", assignee_id: "another-agent" } as Issue} request={request} onChange={setRequest} onPrepared={prepared} />;
    }
    renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><Harness /></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("option", { name: /Figma checkout/ }));
    await screen.findByRole("option", { name: "Checkout" });
    fireEvent.change(screen.getByRole("combobox", { name: "Page / Frame" }), { target: { value: "figma:1:2" } });
    expect(buildPrompt).not.toHaveBeenCalled();
    expect(prepared).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Prepare implementation prompt" }));
    await waitFor(() => expect(prepared).toHaveBeenCalledTimes(1));
    const [prompt, request] = prepared.mock.calls[0]!;
    expect(prompt).toContain("multica_design_get_implementation_context");
    expect(prompt).toContain("<!-- multica-design-implementation:");
    expect(prompt).not.toContain("mention://");
    expect(request).toMatchObject({ operation: "implement", agent_id: "chosen-agent", project_resource_id: "repository-1", design_ref: "figma:checkout", revision_id: "revision-1", frame_refs: ["figma:1:2"] });
    expect(buildPrompt).toHaveBeenCalledWith("figma:checkout", { revision_id: "revision-1", frame_refs: ["figma:1:2"], project_resource_id: "repository-1", issue_id: "issue-1" });
  });
});
