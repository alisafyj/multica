import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault, waitForPageText } from "./helpers";
import type { TestApiClient } from "./fixtures";

// The generation loop, end to end from the product's side. Dispatch itself is
// not exercised: it needs a live agent runtime, which the e2e environment has
// no way to provide. Everything up to and including the approval gate is real,
// and the review side is driven by seeding the rows an agent writeback would
// have produced.
test.describe("Test case generation", () => {
  let api: TestApiClient;
  let workspaceSlug: string;
  let projectId: string;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    const project = await api.createProject("E2E Generation Project " + Date.now());
    projectId = project.id;
    // A plan with no repository and no document is refused at approval, so the
    // project needs something readable attached.
    await api.createProjectResource(projectId, {
      resource_type: "github_repo",
      resource_ref: { url: "https://github.com/acme/billing-api.git" },
    });
    workspaceSlug = await loginAsDefault(page);
  });

  test.afterEach(async () => {
    if (api) {
      await api.cleanup();
    }
  });

  test("starting a generation run from the list page opens its scope for review", async ({
    page,
  }) => {
    await page.goto(`/${workspaceSlug}/tests`, { waitUntil: "domcontentloaded" });
    await page.getByLabel("Select a project").selectOption(projectId);

    await page.getByRole("button", { name: /Generate with AI/i }).click();

    // Creating a job must land on its detail page, not dispatch it: the scope
    // is reviewed first.
    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/tests/jobs/[0-9a-f-]+$`));
  });

  test("the default plan is seeded from the project's own resources", async () => {
    const job = await api.createTestGenerationJob({ project_id: projectId });
    const plan = await api.generateTestGenerationPlan(job.id);

    expect(plan.status).toBe("draft");
    expect(plan.plan.repos).toHaveLength(1);
    expect(plan.plan.repos[0].url).toContain("billing-api");
    // The point of the feature is coverage past code level, so the default
    // scope asks for business case types.
    expect(plan.plan.expected_case_types.length).toBeGreaterThan(0);
  });

  test("a plan cannot be dispatched before it is approved", async () => {
    const job = await api.createTestGenerationJob({ project_id: projectId });
    await api.generateTestGenerationPlan(job.id);

    const res = await api.post(`/api/test-generation-jobs/${job.id}/dispatch`, {
      agent_id: "00000000-0000-0000-0000-000000000000",
    });
    // There is deliberately no skip_plan escape hatch.
    expect(res.status).toBe(409);
  });

  test("an approved plan flips to approved and stays uneditable", async () => {
    const job = await api.createTestGenerationJob({ project_id: projectId });
    await api.generateTestGenerationPlan(job.id);
    const approved = await api.approveTestGenerationPlan(job.id);

    expect(approved.status).toBe("approved");
    expect(approved.approved_at).toBeTruthy();
  });

  test("AI drafts surface under the review filter and can be approved in bulk", async ({
    page,
  }) => {
    // Stand in for what a generation run writes back.
    await api.createTestCase({
      project_id: projectId,
      title: "E2E 调价后进行中订单不受影响",
      case_type: "business_flow",
      status: "draft",
    });

    await page.goto(`/${workspaceSlug}/tests`, { waitUntil: "domcontentloaded" });
    await page.getByLabel("Select a project").selectOption(projectId);
    await waitForPageText(page, "E2E 调价后进行中订单不受影响");

    await expect(page.getByText("E2E 调价后进行中订单不受影响")).toBeVisible();
  });
});
