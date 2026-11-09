import { expect, test } from "@playwright/test";
import { axeViolations, DEV_ADMIN, DEV_DEVELOPER, signInAs } from "./helpers";

// Phase 7 accessibility: no serious or critical axe finding on the main screens,
// in both themes. Moderate and minor findings are listed in the failure message
// only when a serious one exists, so they do not hide it.

for (const theme of ["light", "dark"] as const) {
  test.describe(`axe, ${theme} theme`, () => {
    async function expectNoSerious(page: import("@playwright/test").Page, where: string) {
      const found = await axeViolations(page);
      const bad = found.filter((v) => v.impact === "serious" || v.impact === "critical");
      expect(bad, `${where}: ${JSON.stringify(found)}`).toEqual([]);
      // Every page has exactly one main landmark and an h1.
      expect(found.filter((v) => v.id === "landmark-one-main" || v.id === "page-has-heading-one"), where).toEqual([]);
    }

    test("sign-in page", async ({ page }) => {
      await page.addInitScript((t) => localStorage.setItem("tf-theme", t), theme);
      await page.goto("/");
      await page.getByText(DEV_ADMIN).waitFor();
      await expectNoSerious(page, "sign-in");
    });

    test("home, admin and audit pages", async ({ page }) => {
      await signInAs(page, DEV_ADMIN, theme);
      await expectNoSerious(page, "home");
      await page.getByRole("button", { name: /^Admin/ }).first().click();
      await page.getByRole("tab").first().waitFor();
      await expectNoSerious(page, "admin");
      for (const tab of await page.getByRole("tab").allInnerTexts()) {
        await page.getByRole("tab", { name: tab.trim() }).click();
        await expectNoSerious(page, `admin/${tab.trim()}`);
      }
      await page.goto("/");
      await page.getByRole("button", { name: /^Audit log/ }).first().click();
      await expectNoSerious(page, "audit");
    });

    test("project canvas and its views", async ({ page }) => {
      await signInAs(page, DEV_ADMIN, theme);
      await page.getByRole("button", { name: /Open canvas/ }).first().click();
      await page.locator('button[title="Canvas"]').waitFor();
      await expectNoSerious(page, "canvas");
      for (const view of ["Changesets", "HCL Preview", "Review"]) {
        await page.locator(`button[title="${view}"]`).click();
        await expectNoSerious(page, view);
      }
    });

    test("a developer with no project sees an empty home", async ({ page }) => {
      await signInAs(page, DEV_DEVELOPER, theme);
      await expectNoSerious(page, "developer home");
    });
  });
}
