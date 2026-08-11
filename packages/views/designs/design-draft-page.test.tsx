import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  getDesignDraft,
  getDesignFile,
  materializeDesignDraft,
  navigate,
} = vi.hoisted(() => ({
  getDesignDraft: vi.fn(),
  getDesignFile: vi.fn(),
  materializeDesignDraft: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getDesignDraft,
    getDesignFile,
    materializeDesignDraft,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designs: () => "/acme/designs",
    designDetail: (id: string) => `/acme/designs/${id}`,
    designDraftDetail: (id: string) => `/acme/designs/drafts/${id}`,
  }),
}));

vi.mock("../navigation", () => ({
  AppLink: ({ children, href }: { children: ReactNode; href: string }) => <a href={href}>{children}</a>,
  useNavigation: () => ({ push: navigate }),
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("./native-renderer", () => ({
  NativeDesignPreview: ({ nativeJson }: { nativeJson?: { file?: { title?: string } } }) => (
    <div data-testid="native-preview">{nativeJson?.file?.title ?? "empty"}</div>
  ),
}));

import { DesignDraftPage } from "./design-draft-page";

const compiledNativeJson = {
  version: "1.0",
  file: { title: "客户列表语义稿", sourceType: "ai_generated" },
  frames: [{ id: "frame-1", name: "客户列表", rootLayerId: "layer-1", width: 1200, height: 800 }],
  layers: {
    "layer-1": {
      id: "layer-1",
      frameId: "frame-1",
      name: "客户列表",
      type: "frame",
      visible: true,
      x: 0,
      y: 0,
      width: 1200,
      height: 800,
    },
  },
  assets: {},
};

const semanticDraft = {
  id: "draft-1",
  workspace_id: "ws-1",
  template_id: null,
  catalog_template_id: "template-1",
  template_revision_id: "template-rev-1",
  file_id: "template-file-1",
  revision_id: "template-rev-1",
  generated_file_id: null,
  generated_revision_id: null,
  issue_id: "issue-1",
  title: "客户列表草稿",
  requirement_core: { version: "1.0", title: "客户列表" },
  slot_values: {},
  patch: [],
  status: "generated_with_warnings",
  validation_errors: [],
  created_by: "user-1",
  created_at: "2026-07-23T00:00:00Z",
  updated_at: "2026-07-23T00:00:00Z",
  materialized_at: null,
  generation_mode: "semantic_pagespec",
  page_spec: {
    version: "1.0",
    page: { type: "list", title: "客户列表" },
  },
  compiled_native_json: compiledNativeJson,
  quality_report: {
    diagnostics: [{ severity: "warning", code: "minor_spacing" }],
  },
  blueprint_id: "blueprint-1",
  recipe_set_id: "recipe-set-1",
  parent_draft_id: null,
  version: 2,
};

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

describe("DesignDraftPage", () => {
  beforeEach(() => {
    getDesignDraft.mockReset();
    getDesignFile.mockReset();
    materializeDesignDraft.mockReset();
    navigate.mockReset();
    getDesignDraft.mockResolvedValue(semanticDraft);
  });

  it("renders semantic drafts from compiled native JSON without loading template preview", async () => {
    renderWithClient(<DesignDraftPage draftId="draft-1" />);

    expect(await screen.findByTestId("native-preview")).toHaveTextContent("客户列表语义稿");
    await waitFor(() => expect(getDesignFile).not.toHaveBeenCalled());
    expect(screen.getByText("PageSpec")).toBeInTheDocument();
    expect(screen.getByText("编译质量")).toBeInTheDocument();
    expect(screen.getByText("版本：v2")).toBeInTheDocument();
    expect(screen.getByText("blueprint-1")).toBeInTheDocument();
    expect(screen.getByText("当前草稿可预览")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /生成设计稿/ })).not.toBeInTheDocument();
  });
});
