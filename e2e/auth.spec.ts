import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault, openWorkspaceMenu, waitForPageText } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login", { waitUntil: "domcontentloaded" });
    await waitForPageText(page, "Sign-in failed");

    await expect(page.getByText("Sign-in failed", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry" })).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Email" })).toHaveCount(0);
  });

  test("login and redirect to /issues", async ({ page }) => {
    const workspaceSlug = await loginAsDefault(page);

    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/issues$`));
    await expect(page.getByRole("button", { name: "New Issue" })).toBeVisible();
  });

  test("unauthenticated user is redirected to /login", async ({ page, context }) => {
    const api = await createTestApi();
    const [workspace] = await api.getWorkspaces();
    if (!workspace) {
      throw new Error("E2E workspace was not created");
    }
    await context.clearCookies();

    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await page.waitForURL("**/login", { timeout: 10000, waitUntil: "domcontentloaded" });
    await waitForPageText(page, "Sign-in failed");
  });

  test("logout redirects to /login", async ({ page }) => {
    await loginAsDefault(page);

    // Open the workspace dropdown menu
    await openWorkspaceMenu(page);

    const logoutResponse = page.waitForResponse(
      (response) =>
        response.url().endsWith("/auth/logout") &&
        response.request().method() === "POST",
    );
    await page.getByRole("menuitem", { name: "Log out" }).click();
    await expect((await logoutResponse).status()).toBe(200);

    await page.waitForURL("**/logout", { timeout: 10000 });
    await expect(page).toHaveURL(/\/logout/);
  });
});
