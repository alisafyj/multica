import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault, waitForPageText } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Test cases", () => {
  let api: TestApiClient;
  let workspaceSlug: string;
  let projectTitle: string;
  let projectId: string;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    projectTitle = "E2E Test Case Project " + Date.now();
    const project = await api.createProject(projectTitle);
    projectId = project.id;
    workspaceSlug = await loginAsDefault(page);
  });

  test.afterEach(async () => {
    if (api) {
      await api.cleanup();
    }
  });

  test("the Tests tab is reachable from the sidebar", async ({ page }) => {
    await page.goto(`/${workspaceSlug}/issues`, { waitUntil: "domcontentloaded" });
    await page.getByRole("link", { name: "Tests" }).click();
    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/tests$`));
  });

  test("shows the empty state for a project with no cases", async ({ page }) => {
    await page.goto(`/${workspaceSlug}/tests`, { waitUntil: "domcontentloaded" });
    await page.getByLabel("Select a project").selectOption(projectId);
    await waitForPageText(page, "No test cases yet");
  });

  test("lists a case with its key and lets the detail page edit it", async ({ page }) => {
    const created = await api.createTestCase({
      project_id: projectId,
      title: "E2E 下单成功",
      module: "订单",
      priority: "p1",
      steps: [
        { action: "打开订单页", expected: "列表可见" },
        { action: "点击下单", expected: "跳转支付页" },
      ],
    });

    await page.goto(`/${workspaceSlug}/tests`, { waitUntil: "domcontentloaded" });
    await page.getByLabel("Select a project").selectOption(projectId);
    await waitForPageText(page, created.key);
    await waitForPageText(page, "E2E 下单成功");

    await page.getByRole("link", { name: "E2E 下单成功" }).click();
    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/tests/${created.key}$`));

    // Steps arrive as editable rows, not a markdown blob.
    await expect(page.getByRole("textbox", { name: "Action" })).toHaveCount(2);

    const title = page.getByRole("textbox", { name: "Title" });
    await title.fill("E2E 下单成功（已改）");
    await page.getByRole("button", { name: "Save" }).click();

    await page.reload({ waitUntil: "domcontentloaded" });
    await waitForPageText(page, "E2E 下单成功（已改）");
  });

  test("approve is offered only for a draft case and moves it to active", async ({ page }) => {
    const draft = await api.createTestCase({
      project_id: projectId,
      title: "E2E 待审用例",
      status: "draft",
    });

    await page.goto(`/${workspaceSlug}/tests/${draft.key}`, { waitUntil: "domcontentloaded" });
    const approve = page.getByRole("button", { name: "Approve" });
    await expect(approve).toBeVisible();
    await approve.click();

    await expect(approve).toBeHidden();
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "Approve" })).toBeHidden();
  });

  test("a cross-repo case shows its repository summary in the list", async ({ page }) => {
    const admin = await api.createProjectResource(projectId, {
      resource_type: "github_repo",
      resource_ref: { url: "https://github.com/acme/admin-web" },
    });
    const mobile = await api.createProjectResource(projectId, {
      resource_type: "github_repo",
      resource_ref: { url: "https://github.com/acme/mobile-app" },
    });
    await api.createTestCase({
      project_id: projectId,
      title: "E2E 后台调价后移动端展示新价",
      scope: "cross_repo",
      repos: [
        { project_resource_id: admin.id, alias: "admin-web", role: "driver" },
        { project_resource_id: mobile.id, alias: "mobile-app", role: "verifier" },
      ],
    });

    await page.goto(`/${workspaceSlug}/tests`, { waitUntil: "domcontentloaded" });
    await page.getByLabel("Select a project").selectOption(projectId);
    await waitForPageText(page, "admin-web(driver), mobile-app(verifier)");
  });

  test("deleting a case returns to the list and drops the row", async ({ page }) => {
    const created = await api.createTestCase({
      project_id: projectId,
      title: "E2E 待删除用例",
    });

    await page.goto(`/${workspaceSlug}/tests/${created.key}`, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Delete" }).click();

    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/tests$`));
    await expect(page.getByText("E2E 待删除用例")).toHaveCount(0);
  });
});
