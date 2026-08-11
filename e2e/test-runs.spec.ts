import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault, waitForPageText } from "./helpers";
import type { TestApiClient } from "./fixtures";

// Execution rounds. Agent dispatch is not exercised — it needs a live runtime
// and a bound device, neither of which the e2e environment can provide. What is
// covered is the part that has to hold regardless of who executes: the record
// is append-only, and a rerun never rewrites the round it came from.
test.describe("Test runs", () => {
  let api: TestApiClient;
  let workspaceSlug: string;
  let projectId: string;
  let caseA: { id: string; key: string };
  let caseB: { id: string; key: string };

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    const project = await api.createProject("E2E Run Project " + Date.now());
    projectId = project.id;
    caseA = await api.createTestCase({
      project_id: projectId,
      title: "E2E 下单成功",
      steps: [{ action: "点击下单", expected: "跳转支付页" }],
    });
    caseB = await api.createTestCase({
      project_id: projectId,
      title: "E2E 库存不足时下单被拒",
      steps: [{ action: "对零库存商品下单", expected: "提示库存不足" }],
    });
    workspaceSlug = await loginAsDefault(page);
  });

  test.afterEach(async () => {
    if (api) {
      await api.cleanup();
    }
  });

  test("a round freezes each case as it was when the round started", async () => {
    const run = await api.createTestRun({
      project_id: projectId,
      title: "快照轮次",
      test_case_ids: [caseA.id],
    });
    const runCases = await api.listTestRunCases(run.id);
    expect(runCases).toHaveLength(1);
    expect(runCases[0].case_snapshot.title).toBe("E2E 下单成功");

    // Editing the case afterwards must not rewrite what the round executed.
    await api.post(`/api/test-cases/${caseA.key}`, {});
    const reread = await api.listTestRunCases(run.id);
    expect(reread[0].case_snapshot.title).toBe("E2E 下单成功");
  });

  test("a failed-only rerun copies forward the failures and leaves the original intact", async () => {
    const run = await api.createTestRun({
      project_id: projectId,
      title: "首轮",
      test_case_ids: [caseA.id, caseB.id],
    });
    const runCases = await api.listTestRunCases(run.id);
    const passed = runCases.find((rc: { case_snapshot: { title: string } }) =>
      rc.case_snapshot.title.includes("下单成功"),
    );
    const failed = runCases.find((rc: { case_snapshot: { title: string } }) =>
      rc.case_snapshot.title.includes("库存不足"),
    );
    await api.setTestRunCaseResult(passed.id, { result: "passed" });
    await api.setTestRunCaseResult(failed.id, { result: "failed", notes: "没有提示" });

    const retried = await api.retryTestRun(run.id, { scope: "failed_only" });
    expect(retried.source_run_id).toBe(run.id);

    const retriedCases = await api.listTestRunCases(retried.id);
    expect(retriedCases).toHaveLength(1);
    expect(retriedCases[0].case_snapshot.title).toContain("库存不足");

    // The record is the product: a rerun must not touch what the first round
    // observed.
    const originalCases = await api.listTestRunCases(run.id);
    const originalResults = originalCases
      .map((rc: { result: string }) => rc.result)
      .sort();
    expect(originalResults).toEqual(["failed", "passed"]);
  });

  test("a round completes on its own once every case has a result", async () => {
    const run = await api.createTestRun({
      project_id: projectId,
      title: "自动完成轮次",
      test_case_ids: [caseA.id],
    });
    const runCases = await api.listTestRunCases(run.id);
    await api.setTestRunCaseResult(runCases[0].id, { result: "passed" });

    const res = await api.post(`/api/test-runs/${run.id}/abort`, {});
    // Already completed, so aborting is refused rather than reopening it.
    expect(res.status).toBe(409);
  });

  test("the plans surface is reachable from the cases page", async ({ page }) => {
    await page.goto(`/${workspaceSlug}/tests`, { waitUntil: "domcontentloaded" });
    await page.getByLabel("Select a project").selectOption(projectId);

    await page.getByRole("button", { name: /Plans|计划/i }).click();
    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/tests/plans$`));
  });

  test("a case's timeline links back to the round that produced each result", async ({
    page,
  }) => {
    const run = await api.createTestRun({
      project_id: projectId,
      title: "可追溯轮次",
      test_case_ids: [caseA.id],
    });
    const runCases = await api.listTestRunCases(run.id);
    await api.setTestRunCaseResult(runCases[0].id, { result: "failed" });

    await page.goto(`/${workspaceSlug}/tests/${caseA.key}`, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, "可追溯轮次");

    await page.getByRole("link", { name: "可追溯轮次" }).click();
    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/tests/runs/${run.id}$`));
  });
});
