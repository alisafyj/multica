import { type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";
const DEFAULT_E2E_EMAIL = "e2e@multica.ai";
const DEFAULT_E2E_WORKSPACE = "e2e-workspace";
const FRONTEND_ORIGIN = new URL(
  process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN ?? "http://localhost:3000",
).origin;

export async function authenticatePage(
  page: Page,
  api: TestApiClient,
  workspaceSlug?: string,
) {
  const { token, csrfToken, expiresAt } = api.getBrowserSession();
  const cookies = [
    { name: "multica_auth", value: token, url: FRONTEND_ORIGIN, httpOnly: true, sameSite: "Strict" as const, expires: expiresAt },
    { name: "multica_csrf", value: csrfToken, url: FRONTEND_ORIGIN, sameSite: "Strict" as const, expires: expiresAt },
    { name: "multica_logged_in", value: "1", url: FRONTEND_ORIGIN, sameSite: "Lax" as const, expires: expiresAt },
  ];
  if (workspaceSlug) {
    cookies.push({ name: "last_workspace_slug", value: workspaceSlug, url: FRONTEND_ORIGIN, sameSite: "Lax", expires: expiresAt });
  }
  await page.context().addCookies(cookies);
}

/**
 * Log in as the default E2E user and ensure the workspace exists first.
 * Creates a local SSO test identity, then injects the same HttpOnly auth and
 * CSRF cookies used by the web app.
 *
 * Returns the E2E workspace slug so callers can build workspace-scoped URLs.
 */
export async function loginAsDefault(page: Page): Promise<string> {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  const workspace = await api.ensureWorkspace(
    "E2E Workspace",
    DEFAULT_E2E_WORKSPACE,
  );

  await authenticatePage(page, api, workspace.slug);
  await page.goto(`/${workspace.slug}/issues`);
  await page.waitForURL("**/issues", { timeout: 10000 });
  return workspace.slug;
}

/**
 * Create a TestApiClient logged in as the default E2E user.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export async function createTestApi(): Promise<TestApiClient> {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  await api.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);
  return api;
}

export async function openWorkspaceMenu(page: Page) {
  await page.getByRole("button", { name: /E2E Workspace/ }).click();
  // Wait for dropdown to appear
  await page.locator('[class*="popover"]').waitFor({ state: "visible" });
}
