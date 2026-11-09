import { expect, test, type Page } from "@playwright/test";
import { DEV_ADMIN, signInAs } from "./helpers";

// Phase 7 loading, error and empty states: what the person sees when the server
// answers with an error or with nothing. An unresolved value is never shown as
// healthy or blank (hard rule 8).

const json = (status: number, body: unknown) => ({ status, contentType: "application/json", body: JSON.stringify(body) });
const boom = json(500, { error: "boom" });

async function open(page: Page) {
  await page.getByRole("button", { name: /Open canvas/ }).first().click();
}

test("projects fail to load: says so, keeps the page usable", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/projects", (r) => r.fulfill(boom));
  await page.reload();
  await expect(page.getByText("Could not load projects: boom")).toBeVisible();
  await expect(page.getByRole("button", { name: "New project" })).toBeVisible();
});

test("no projects: an empty state with the next step", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/projects", (r) => r.fulfill(json(200, [])));
  await page.reload();
  await expect(page.getByText("No projects yet")).toBeVisible();
});

test("graph fails to load: names the problem", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/graph*", (r) => r.fulfill(boom));
  await open(page);
  await expect(page.getByText("Could not load the project: boom")).toBeVisible();
});

test("graph is slow: shows a loading message first", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/graph*", async (r) => {
    await new Promise((x) => setTimeout(x, 2500));
    await r.continue();
  });
  await open(page);
  await expect(page.getByText("Loading graph...")).toBeVisible();
  await expect(page.getByText("Loading graph...")).toBeHidden({ timeout: 15_000 });
});

test("catalog fails to load: the panel says so and the page survives", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/catalog", (r) => r.fulfill(boom));
  await open(page);
  await expect(page.getByText("Could not load the module catalog: boom")).toBeVisible();
  await expect(page.getByText("Something went wrong")).toHaveCount(0);
});

test("changesets fail to load: the error is shown and 'No changesets yet' is not", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/changesets/", (r) => r.fulfill(boom));
  await open(page);
  await page.locator('button[title="Changesets"]').click();
  await expect(page.getByText("boom")).toBeVisible();
  await expect(page.getByText("No changesets yet")).toHaveCount(0);
});
