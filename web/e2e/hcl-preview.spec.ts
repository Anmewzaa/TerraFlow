import { expect, test } from "@playwright/test";
import { DEV_ADMIN, signInAs } from "./helpers";

test("HCL preview shows the project's Terraform with syntax colours", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.getByRole("button", { name: /Open canvas/ }).first().click();
  await page.locator('button[title="HCL Preview"]').click();
  await expect(page.getByText("Nothing changed yet.")).toBeVisible();
  await page.getByLabel("Show only what changed").uncheck(); // read the files already in the repo
  await expect(page.locator("code [class*=token]").first()).toBeVisible(); // the highlighter ran
  expect((await page.locator("code:has([class*=token])").first().innerText()).trim().length).toBeGreaterThan(20);
});
