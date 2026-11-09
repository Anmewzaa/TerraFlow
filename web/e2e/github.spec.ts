import { expect, test, type Page } from "@playwright/test";
import { DEV_ADMIN, signInAs } from "./helpers";

// Phase 8 screens: the "New from GitHub" dialog and the admin connections tab. GitHub
// is never contacted: the server's answers are replaced, so these run anywhere.

const json = (status: number, body: unknown) => ({ status, contentType: "application/json", body: JSON.stringify(body) });

const repo = { connection_id: "c1", account: "alice", repo_id: "42", full_name: "alice/infra", default_branch: "main", private: true };

async function openDialog(page: Page) {
  await page.getByRole("button", { name: /New from GitHub/ }).first().click();
  return page.getByRole("dialog", { name: "New project from GitHub" });
}

test("GitHub not set up: the dialog says an administrator has to configure it", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: false })));
  const dlg = await openDialog(page);
  await expect(dlg.getByText("GitHub is not set up on this TerraFlow server")).toBeVisible();
});

test("no repositories yet: offers Connect GitHub and the install link", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true, install_url: "https://github.com/apps/x/installations/new" })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [])));
  const dlg = await openDialog(page);
  await expect(dlg.getByRole("button", { name: /Connect GitHub/ })).toBeVisible();
  await expect(dlg.getByRole("link", { name: /Install the App/ })).toHaveAttribute("href", /installations\/new/);
});

test("pick a repository and a folder, then create: the right request is sent", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [repo])));
  await page.route("**/api/github/folders*", (r) => r.fulfill(json(200, { folders: ["iac/AIC", "iac/AIH"], default_branch: "main" })));
  let posted: unknown = null;
  await page.route("**/api/projects/from-repo", async (r) => {
    posted = r.request().postDataJSON();
    await r.fulfill(json(201, { id: "p1", name: "infra", path: "", created_at: new Date().toISOString() }));
  });
  const dlg = await openDialog(page);
  const submit = dlg.getByRole("button", { name: /Create project/ });
  await expect(submit).toBeDisabled();
  await dlg.getByLabel("Repository").selectOption({ label: "alice/infra (private)" });
  await expect(dlg.getByLabel("Terraform folder")).toBeVisible();
  await expect(submit).toBeDisabled(); // two folders: one must be chosen
  await dlg.getByLabel("Terraform folder").selectOption("iac/AIC");
  await expect(dlg.getByLabel("Project name")).toHaveValue("infra");
  await submit.click();
  await expect.poll(() => posted).toEqual({ name: "infra", connection_id: "c1", repo_id: "42", folder: "iac/AIC" });
});

test("a repository without Terraform cannot be used", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [repo])));
  await page.route("**/api/github/folders*", (r) => r.fulfill(json(200, { folders: [], default_branch: "main" })));
  const dlg = await openDialog(page);
  await dlg.getByLabel("Repository").selectOption({ index: 1 });
  await expect(dlg.getByText("No folder in this repository holds Terraform")).toBeVisible();
  await expect(dlg.getByRole("button", { name: /Create project/ })).toBeDisabled();
});

test("the server's refusal is shown, and a non-JSON failure names the call", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true })));
  await page.route("**/api/github/repos", (r) => r.fulfill({ status: 404, contentType: "text/plain", body: "404 page not found" }));
  const dlg = await openDialog(page);
  await expect(dlg.getByRole("alert")).toContainText("Request failed (404) /api/github/repos");
});

test("coming back from GitHub with an error opens the dialog with a clear message", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [])));
  // A fresh page load, as after GitHub sends the browser back (the hash is read once).
  await page.goto("about:blank");
  await page.goto("/#/projects/new?error=no_installation");
  await expect(page.getByRole("dialog", { name: "New project from GitHub" }).getByRole("alert")).toContainText("not installed on any account you can reach");
});

test("admin connections tab lists connections and can disconnect one", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  const rows = [{ id: "c1", provider: "github", account_login: "alice", status: "active", account_type: "User", projects: 2, created_by: "u1", created_at: new Date().toISOString() }];
  await page.route("**/api/admin/connections", (r) => r.fulfill(json(200, rows)));
  let disconnected = false;
  await page.route("**/api/admin/connections/c1/disconnect", (r) => {
    disconnected = true;
    return r.fulfill(json(200, { ...rows[0], status: "disconnected" }));
  });
  await page.getByRole("button", { name: /Admin/ }).first().click();
  await page.getByRole("tab", { name: "connections" }).click();
  await expect(page.getByText("GitHub connections")).toBeVisible();
  await expect(page.getByText("alice")).toBeVisible();
  await page.getByRole("button", { name: "Disconnect", exact: true }).click(); // asks to confirm
  await page.getByRole("button", { name: "Disconnect 2 projects" }).click();
  await expect.poll(() => disconnected).toBe(true);
});
