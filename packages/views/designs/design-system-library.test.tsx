import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { getBuiltinDesignSystem, getProjectDesignSystem, listBuiltinDesignSystems, listProjectDesignSystemCatalogue } = vi.hoisted(() => ({
  getBuiltinDesignSystem: vi.fn(),
  getProjectDesignSystem: vi.fn(),
  listBuiltinDesignSystems: vi.fn(),
  listProjectDesignSystemCatalogue: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getBuiltinDesignSystem,
    getProjectDesignSystem,
    listBuiltinDesignSystems,
    listProjectDesignSystemCatalogue,
    getBaseUrl: () => "https://api.test",
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

// The library only borrows PLATFORM_OPTIONS from the composer; the composer's
// avatars resolve names through providers this test does not mount.
vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

import { DesignSystemLibrary, designMarkdownModules } from "./design-system-library";

const BUILTIN_APPLE = {
  slug: "apple",
  name: "Apple",
  category: "媒体与消费",
  description: "克制的编辑式版式，单一强调色。",
  showcase_url: "/api/design-systems/builtin/apple/showcase/abc123def456/light",
  swatches: ["#ffffff", "#0071e3", "#1d1d1f"],
};

const BUILTIN_STRIPE = {
  slug: "stripe",
  name: "Stripe",
  category: "金融科技",
  description: "",
  showcase_url: "",
  swatches: [],
};

const APPLE_DESIGN_MD = `---
title: Apple
---
# Apple

Apple 的视觉语言以留白和克制为核心。

## Color Palette

主色 #0071e3 只用于行动。

## Typography

SF Pro Display 用于标题。
`;

const PROJECT_SYSTEM = {
  id: "system-1",
  project_id: "project-1",
  project_title: "看板体验",
  project_resource_id: "",
  name: "Multica Web",
  platform: "web",
  saved_at: "2026-08-16T00:00:00Z",
};

const REPOSITORY_SYSTEM = {
  id: "system-2",
  project_id: "project-1",
  project_title: "看板体验",
  project_resource_id: "resource-h5",
  name: "看板 H5",
  platform: "mobile",
  saved_at: "2026-08-16T02:00:00Z",
};

function systemDetail(overrides: Record<string, unknown> = {}) {
  return {
    id: "system-1",
    workspace_id: "ws-1",
    project_id: "project-1",
    project_resource_id: "",
    name: "Multica Web",
    platform: "web",
    current_agent_id: null,
    status: "saved",
    active_task: null,
    input_snapshot: {},
    content: {
      sections: [],
      token_groups: [
        { id: "color", label: "色彩", tokens: [{ name: "--brand", value: "#2f6feb" }] },
        { id: "typography", label: "字体", tokens: [{ name: "--text-body", value: "14px/20px" }] },
        { id: "radius", label: "圆角与阴影", tokens: [{ name: "--radius", value: "10px" }] },
      ],
      locators: [{ id: "button", kind: "component", label: "主按钮" }],
      preview_html: "",
      integrity_sha256: "digest-1",
    },
    preview_validation: { status: "passed", integrity_sha256: "digest-1", report: {}, verified_at: null },
    has_unsaved_changes: false,
    last_error: null,
    activity: [],
    created_at: "2026-08-16T00:00:00Z",
    updated_at: "2026-08-16T00:00:00Z",
    saved_at: "2026-08-16T00:00:00Z",
    ...overrides,
  };
}

function renderLibrary(onOpenProject = vi.fn()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const ui: ReactNode = (
    <QueryClientProvider client={queryClient}>
      <DesignSystemLibrary onOpenProject={onOpenProject} />
    </QueryClientProvider>
  );
  render(ui);
  return onOpenProject;
}

// The markdown splitter has its matrix here; the DOM test below only checks
// that the modules reach the page.
describe("designMarkdownModules", () => {
  it("drops front matter and the title, keeps the preamble and one module per ## heading", () => {
    const modules = designMarkdownModules(APPLE_DESIGN_MD);
    expect(modules.preamble).toBe("Apple 的视觉语言以留白和克制为核心。");
    expect(modules.sections.map((section) => section.title)).toEqual(["Color Palette", "Typography"]);
    expect(modules.sections[0]?.body).toBe("主色 #0071e3 只用于行动。");
  });

  it("strips numbering from headings and skips empty sections", () => {
    const modules = designMarkdownModules("## 1. 语调\n\n直接。\n\n## 2) 空章节\n\n## 3. 布局\n网格。");
    expect(modules.sections.map((section) => section.title)).toEqual(["语调", "布局"]);
    expect(designMarkdownModules("")).toEqual({ preamble: "", sections: [] });
  });
});

describe("DesignSystemLibrary", () => {
  beforeEach(() => {
    getProjectDesignSystem.mockReset();
    listProjectDesignSystemCatalogue.mockReset();
    listBuiltinDesignSystems.mockReset();
    getBuiltinDesignSystem.mockReset();
    listProjectDesignSystemCatalogue.mockResolvedValue({ design_systems: [PROJECT_SYSTEM] });
    getProjectDesignSystem.mockResolvedValue(systemDetail());
    listBuiltinDesignSystems.mockResolvedValue({ design_systems: [BUILTIN_APPLE, BUILTIN_STRIPE] });
    getBuiltinDesignSystem.mockImplementation(async (slug: string) => slug === "stripe"
      ? { ...BUILTIN_STRIPE, tokens: [], tokens_css: "", design_markdown: "# Stripe" }
      : {
        ...BUILTIN_APPLE,
        tokens: [
          { name: "--bg", value: "#ffffff", type: "color" },
          { name: "--accent", value: "#0071e3", type: "color" },
          { name: "--font-display", value: "\"SF Pro Display\", sans-serif", type: "fontFamily" },
        ],
        tokens_css: ":root{--bg:#ffffff}",
        design_markdown: APPLE_DESIGN_MD,
      });
  });

  it("frames the official showcase and renders the design language as modules", async () => {
    renderLibrary();
    await userEvent.click(await screen.findByRole("button", { name: /官方/ }));

    // The list row shows the palette at a glance; the first row opens.
    expect(await screen.findByRole("heading", { name: "Apple" })).toBeInTheDocument();
    const frame = screen.getByTitle("Apple 展示");
    expect(frame).toHaveAttribute("src", "https://api.test/api/design-systems/builtin/apple/showcase/abc123def456/light");
    expect(frame).toHaveAttribute("sandbox", "");
    await userEvent.click(screen.getByRole("button", { name: "深色" }));
    expect(screen.getByTitle("Apple 展示")).toHaveAttribute("src", "https://api.test/api/design-systems/builtin/apple/showcase/abc123def456/dark");

    // Identity preamble, then one module per heading, plus typed tokens.
    expect(screen.getByText("Apple 的视觉语言以留白和克制为核心。")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Color Palette" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Typography" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "字体排版" })).toBeInTheDocument();
    expect(screen.getByText("--font-display")).toBeInTheDocument();
    expect(screen.getByText("--accent")).toBeInTheDocument();

    // A package without a showcase simply has no cover.
    await userEvent.click(screen.getByRole("button", { name: /Stripe/ }));
    expect(await screen.findByRole("heading", { name: "Stripe" })).toBeInTheDocument();
    expect(screen.queryByTitle("Stripe 展示")).not.toBeInTheDocument();
  });

  it("filters the official catalogue by category from a dropdown", async () => {
    renderLibrary();
    await userEvent.click(await screen.findByRole("button", { name: /官方/ }));
    expect(await screen.findByRole("heading", { name: "Apple" })).toBeInTheDocument();

    // Twenty-odd categories as pills buried the list; a select keeps the
    // list in view. The trigger carries the active label and its count.
    const select = screen.getByRole("combobox", { name: "官方设计体系分类" });
    expect(select.textContent).toContain("全部分类");
    expect(select.textContent).toContain("2");
    await userEvent.click(select);
    await userEvent.click(await screen.findByRole("option", { name: /金融科技/ }));

    expect(screen.getByRole("combobox", { name: "官方设计体系分类" }).textContent).toContain("金融科技");
    expect(screen.queryByRole("button", { name: /Apple/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Stripe/ })).toBeInTheDocument();
  });

  it("opens the first saved system and shows its own token sections", async () => {
    renderLibrary();

    expect(await screen.findByRole("heading", { name: "Multica Web" })).toBeInTheDocument();
    expect(screen.getByText("项目绑定 · 看板体验 · Web")).toBeInTheDocument();
    expect(await screen.findByText("--brand")).toBeInTheDocument();
    expect(screen.getByText("--text-body")).toBeInTheDocument();
    expect(screen.getByText("--radius")).toBeInTheDocument();
    expect(screen.getByText("主按钮")).toBeInTheDocument();
  });

  it("never offers a workspace default, only the scope a system belongs to", async () => {
    listProjectDesignSystemCatalogue.mockResolvedValue({
      design_systems: [PROJECT_SYSTEM, REPOSITORY_SYSTEM],
    });
    const user = userEvent.setup();
    renderLibrary();

    // Repository scope replaces inheritance: each system is its own, and no
    // system is ever the workspace default (DC-052).
    expect(await screen.findByRole("heading", { name: "Multica Web" })).toBeInTheDocument();
    expect(screen.getAllByText("项目通用").length).toBeGreaterThan(0);
    expect(screen.queryByText("默认")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "设为默认" })).not.toBeInTheDocument();
    expect(screen.queryByText(/继承自/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "新增覆盖" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /看板 H5/ }));
    expect(await screen.findByRole("heading", { name: "看板 H5" })).toBeInTheDocument();
    expect(screen.getAllByText("仓库专属").length).toBeGreaterThan(0);
  });

  it("marks a system under adjustment as a draft from the detail payload", async () => {
    getProjectDesignSystem.mockResolvedValue(systemDetail({ status: "draft" }));
    renderLibrary();

    expect(await screen.findByText("草稿")).toBeInTheDocument();
  });

  it("says why an ownership scope is empty instead of borrowing another one's systems", async () => {
    const user = userEvent.setup();
    renderLibrary();

    const mine = await screen.findByRole("button", { name: /我的/ });
    await user.click(mine);

    expect(await screen.findByText("这里还没有设计体系")).toBeInTheDocument();
    expect(screen.getByText(/设计体系归属项目/)).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Multica Web" })).not.toBeInTheDocument();
  });

  it("hands editing back to the project that owns the system", async () => {
    const user = userEvent.setup();
    const onOpenProject = renderLibrary();

    await user.click(await screen.findByRole("button", { name: "打开项目" }));
    expect(onOpenProject).toHaveBeenCalledWith("project-1");
  });

  it("keeps the list usable when the detail payload cannot be loaded", async () => {
    getProjectDesignSystem.mockRejectedValue(new Error("nope"));
    renderLibrary();

    expect(await screen.findByText("无法加载这套设计体系的内容。")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Multica Web" })).toBeInTheDocument();
  });
});
