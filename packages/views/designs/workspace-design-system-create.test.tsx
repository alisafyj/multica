import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const navigate = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());
const { createProjectDesignSystem, listAgents, uploadFile } = vi.hoisted(() => ({
  createProjectDesignSystem: vi.fn(),
  listAgents: vi.fn(),
  uploadFile: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: { createProjectDesignSystem, uploadFile },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({
    queryKey: ["agents"],
    queryFn: listAgents,
  }),
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designs: () => "/acme/designs",
    projectDesignSystemDetail: (id: string) => `/acme/designs/systems/${id}`,
    projectDesignSystemNew: () => "/acme/designs/systems/new",
  }),
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: navigate }),
}));

// ReadonlyContent drags in lowlight + KaTeX + Mermaid; the preview pane's
// rendering is the editor suite's concern, not this page's.
vi.mock("../editor", () => ({
  ReadonlyContent: ({ content }: { content: string }) => <div data-testid="design-md-preview">{content}</div>,
}));

vi.mock("sonner", () => ({
  toast: { error: toastError },
}));

import { WorkspaceDesignSystemCreate, sourceLinkLabel } from "./workspace-design-system-create";
import { BRAND_CATEGORIES, BRAND_CATEGORY_LABELS, BRAND_REFERENCES, QUICK_PICK_BRANDS } from "./brand-references";

const AGENT = { id: "agent-1", workspace_id: "ws-1", name: "小设计", runtime_id: "runtime-1", runtime_bound: true, archived_at: null, status: "online" };

function renderCreate() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <WorkspaceDesignSystemCreate />
    </QueryClientProvider>,
  );
}

/** The topbar's requirement hint (a second copy renders for small screens). */
function requirementHint(): string {
  return screen.getAllByRole("status")[0]?.textContent ?? "";
}

describe("WorkspaceDesignSystemCreate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listAgents.mockResolvedValue([AGENT]);
  });

  it("replicates Open Design's creation page: sticky-topbar generate, hero, one extraction card", () => {
    renderCreate();

    // The generate action lives in the top bar, next to 返回 — not a footer.
    expect(screen.getByRole("button", { name: "返回" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /继续生成/ })).toBeDisabled();
    expect(requirementHint()).toContain("名字");

    // Hero and the single extraction section.
    expect(screen.getByRole("heading", { level: 1, name: "几分钟，生成一套设计体系" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "从 GitHub、网站或源素材提取" })).toBeInTheDocument();

    // Open Design's row set, plus this product's own rows.
    for (const label of ["名称", "GitHub 或网站", "添加文件", "描述品牌", "粘贴 DESIGN.md", "备注", "智能体", "平台"]) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0);
    }
  });

  it("sends a standalone create with the collected sources, then opens the system", async () => {
    const user = userEvent.setup();
    createProjectDesignSystem.mockResolvedValue({ id: "system-9", project_id: "", name: "品牌视觉基线" });
    uploadFile.mockResolvedValue({ id: "att-md", filename: "DESIGN.md", url: "/files/att-md", content_type: "text/markdown" });
    renderCreate();

    await user.type(screen.getByLabelText("设计体系名称"), "品牌视觉基线");
    await user.type(screen.getByLabelText("GitHub 或网站"), "https://acme.example");
    await user.click(screen.getByRole("button", { name: "添加" }));
    await user.type(screen.getByLabelText("品牌描述"), "克制的工具品牌，蓝灰基调。");
    await user.type(screen.getByLabelText("备注"), "圆角，语气专业。");
    await user.type(screen.getByLabelText("粘贴 DESIGN.md"), "# Tokens");
    await user.click(screen.getByRole("radio", { name: "移动端" }));

    // 从品牌开始: the quick-pick chip adds the brand's website to the source
    // links (Open Design's pick semantics) and shows up as a labelled chip.
    await user.click(screen.getByRole("button", { name: /从品牌开始/ }));
    const dialog = await screen.findByRole("dialog");
    const quickPicks = within(dialog).getByRole("group", { name: "热门品牌 · 点击添加" });
    await user.click(within(quickPicks).getByRole("button", { name: "Shopify" }));
    expect(within(screen.getByLabelText("已添加的来源链接")).getByText("shopify.com")).toBeInTheDocument();

    // agents query resolves on the next tick; pick once options appear.
    await screen.findByRole("option", { name: /小设计/ });
    await user.selectOptions(screen.getByLabelText("智能体"), "agent-1");

    await user.click(screen.getByRole("button", { name: /继续生成/ }));

    await vi.waitFor(() =>
      expect(createProjectDesignSystem).toHaveBeenCalledWith(
        expect.objectContaining({
          project_id: "",
          name: "品牌视觉基线",
          brief: "克制的工具品牌，蓝灰基调。\n\n备注：圆角，语气专业。",
          platform: "mobile",
          references: [
            { kind: "link", value: "https://acme.example", label: "来源链接" },
            { kind: "link", value: "https://shopify.com", label: "来源链接" },
            { kind: "attachment", attachment_id: "att-md", label: "粘贴的 DESIGN.md" },
          ],
        }),
      ),
    );
    // The pasted DESIGN.md went up as a markdown file with the pasted bytes.
    const pasted = uploadFile.mock.calls[0]?.[0] as File;
    expect(pasted.name).toBe("DESIGN.md");
    expect(await pasted.text()).toBe("# Tokens");
    expect(navigate).toHaveBeenCalledWith("/acme/designs/systems/system-9");
  });

  it("filters the brand wall by category and search, like Open Design's picker", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.click(screen.getByRole("button", { name: /从品牌开始/ }));
    const dialog = await screen.findByRole("dialog");

    // Category nav narrows the wall and hides the quick-pick row.
    await user.click(within(dialog).getByRole("button", { name: "汽车" }));
    expect(within(dialog).queryByRole("group", { name: "热门品牌 · 点击添加" })).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: /Porsche/ })).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /Shopify/ })).not.toBeInTheDocument();

    // Search matches the zh category label too (typing 媒体 finds Vogue).
    await user.click(within(dialog).getByRole("button", { name: "全部" }));
    await user.type(within(dialog).getByLabelText("搜索品牌"), "媒体");
    expect(within(dialog).getByRole("button", { name: /Vogue/ })).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /Porsche/ })).not.toBeInTheDocument();

    // Picking from the wall adds the site and closes the picker.
    await user.clear(within(dialog).getByLabelText("搜索品牌"));
    await user.type(within(dialog).getByLabelText("搜索品牌"), "vogue");
    await user.click(within(dialog).getByRole("button", { name: /Vogue/ }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(within(screen.getByLabelText("已添加的来源链接")).getByText("vogue.com")).toBeInTheDocument();
  });

  it("stages dropped files as attachment references and lets them be removed", async () => {
    const user = userEvent.setup({ applyAccept: false });
    uploadFile.mockResolvedValue({ id: "att-1", filename: "logo-notes.txt", url: "/files/att-1", content_type: "text/plain" });
    renderCreate();

    await user.upload(
      screen.getByLabelText("上传素材文件"),
      new File(["brand notes"], "logo-notes.txt", { type: "text/plain" }),
    );

    const staged = await screen.findByLabelText("已暂存的素材");
    expect(within(staged).getByText("logo-notes.txt")).toBeInTheDocument();
    expect(uploadFile).toHaveBeenCalledTimes(1);

    await user.click(within(staged).getByRole("button", { name: "移除 logo-notes.txt" }));
    expect(screen.queryByLabelText("已暂存的素材")).not.toBeInTheDocument();
  });

  it("rejects a source that is not an https link", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(screen.getByLabelText("GitHub 或网站"), "http://insecure.example");
    expect(screen.getByRole("button", { name: "添加" })).toBeDisabled();
    expect(screen.getByText("请输入 https:// 开头的完整链接。")).toBeInTheDocument();
  });

  it("keeps DESIGN.md 预览 disabled until there is content, then renders it", async () => {
    const user = userEvent.setup();
    renderCreate();

    const previewToggle = screen.getByRole("button", { name: "预览" });
    expect(previewToggle).toBeDisabled();

    await user.type(screen.getByLabelText("粘贴 DESIGN.md"), "## Overview");
    await user.click(screen.getByRole("button", { name: "预览" }));
    expect(screen.getByTestId("design-md-preview")).toHaveTextContent("## Overview");
  });
});

// Canonical for the label shape: protocol/www stripped, GitHub shortened.
describe("sourceLinkLabel", () => {
  it("shortens URLs the way Open Design labels source chips", () => {
    expect(sourceLinkLabel("https://www.acme.example/")).toBe("acme.example");
    expect(sourceLinkLabel("https://github.com/vercel/next.js")).toBe("vercel/next.js");
    expect(sourceLinkLabel("https://docs.acme.example/brand/")).toBe("docs.acme.example/brand");
    expect(sourceLinkLabel("not a url")).toBe("not a url");
  });
});

// Canonical for the ported catalogue: Open Design's fame ordering decides the
// wall and the quick-pick row, and every bucket carries its zh label.
describe("brand references", () => {
  it("leads the wall and quick picks with Open Design's fame ordering", () => {
    expect(QUICK_PICK_BRANDS.map((brand) => brand.name)).toEqual([
      "Shopify",
      "Slack",
      "Stripe",
      "Nike",
      "New Balance",
      "Nespresso",
      "Spotify",
      "Vogue",
    ]);
    expect(BRAND_REFERENCES).toHaveLength(65);
  });

  it("labels every category in Chinese", () => {
    for (const category of BRAND_CATEGORIES) {
      expect(BRAND_CATEGORY_LABELS[category], category).toBeTruthy();
    }
  });
});
