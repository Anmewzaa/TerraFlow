import { expect, test, type Page } from "@playwright/test";
import { DEV_ADMIN, signInAs } from "./helpers";

// Phase 8 screens: the "New from GitHub" dialog and the admin connections tab. GitHub
// is never contacted: the server's answers are replaced, so these run anywhere.

const json = (status: number, body: unknown) => ({ status, contentType: "application/json", body: JSON.stringify(body) });

const repo = { connection_id: "c1", account: "alice", repo_id: "42", full_name: "alice/infra", default_branch: "main", private: true };

async function openDialog(page: Page) {
  await page.getByRole("button", { name: /New project/ }).first().click();
  return page.getByRole("dialog", { name: "New project from GitHub" });
}

test("GitHub not set up: the dialog says an administrator has to configure it", async ({ page }) => {
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: false, project_source: "github" })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(409, { error: "GitHub is not configured" })));
  await signInAs(page, DEV_ADMIN); // the routes first: the page asks GitHub's state when it loads
  const dlg = await openDialog(page);
  await expect(dlg.getByText("GitHub is not set up on this TerraFlow server")).toBeVisible();
});

test("nobody connected yet: New project goes straight to GitHub to connect", async ({ page }) => {
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true, project_source: "github" })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [])));
  await page.route("**/api/github/authorize", (r) => r.fulfill({ status: 200, contentType: "text/html", body: "<title>GitHub</title>" }));
  await signInAs(page, DEV_ADMIN);
  await page.getByRole("button", { name: /New project/ }).first().click();
  await expect(page).toHaveURL(/\/api\/github\/authorize/);
});

test("back from GitHub with no repository given: offers the link to choose them", async ({ page }) => {
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true, project_source: "github", install_url: "https://github.com/apps/x/installations/new" })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [])));
  await signInAs(page, DEV_ADMIN);
  await page.goto("about:blank");
  await page.goto("/#/projects/new?connected=1");
  const dlg = page.getByRole("dialog", { name: "New project from GitHub" });
  await expect(dlg.getByRole("link", { name: /Choose repositories on GitHub/ })).toHaveAttribute("href", /installations\/new/);
});

test("pick a repository and a folder, then create: the right request is sent", async ({ page }) => {
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true, project_source: "github" })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [repo])));
  await page.route("**/api/github/folders*", (r) => r.fulfill(json(200, { folders: ["iac/AIC", "iac/AIH"], default_branch: "main" })));
  let posted: unknown = null;
  await page.route("**/api/projects/from-repo", async (r) => {
    posted = r.request().postDataJSON();
    await r.fulfill(json(201, { id: "p1", name: "infra", path: "", created_at: new Date().toISOString() }));
  });
  await signInAs(page, DEV_ADMIN); // the routes first: the page asks GitHub's state when it loads
  const dlg = await openDialog(page);
  const submit = dlg.getByRole("button", { name: /Create project/ });
  await expect(submit).toBeDisabled();
  await dlg.getByRole("option", { name: /alice\/infra/ }).click();
  await expect(dlg.getByLabel("Terraform folder")).toBeVisible();
  await expect(submit).toBeDisabled(); // two folders: one must be chosen
  await dlg.getByLabel("Terraform folder").selectOption("iac/AIC");
  await expect(dlg.getByLabel("Project name")).toHaveValue("infra");
  await submit.click();
  await expect.poll(() => posted).toMatchObject({ name: "infra", connection_id: "c1", repo_id: "42", folder: "iac/AIC" });
});

test("a repository without Terraform cannot be used", async ({ page }) => {
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true, project_source: "github" })));
  await page.route("**/api/github/repos", (r) => r.fulfill(json(200, [repo])));
  await page.route("**/api/github/folders*", (r) => r.fulfill(json(200, { folders: [], default_branch: "main" })));
  await signInAs(page, DEV_ADMIN); // the routes first: the page asks GitHub's state when it loads
  const dlg = await openDialog(page);
  await dlg.getByRole("option", { name: /alice\/infra/ }).click();
  await expect(dlg.getByText("This repository has no Terraform yet")).toBeVisible();
  await expect(dlg.getByRole("button", { name: /Create project/ })).toBeDisabled();
});

test("the server's refusal is shown, and a non-JSON failure names the call", async ({ page }) => {
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true, project_source: "github" })));
  await page.route("**/api/github/repos", (r) => r.fulfill({ status: 404, contentType: "text/plain", body: "404 page not found" }));
  await signInAs(page, DEV_ADMIN); // the routes first: the page asks GitHub's state when it loads
  const dlg = await openDialog(page);
  await expect(dlg.getByRole("alert")).toContainText("Request failed (404) /api/github/repos");
});

test("coming back from GitHub with an error opens the dialog with a clear message", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/github/status", (r) => r.fulfill(json(200, { configured: true, project_source: "github" })));
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
  await page.getByRole("button", { name: /People & access/ }).click();
  await page.getByRole("tab", { name: /GitHub connections/ }).click();
  await expect(page.getByText(/A connection is one installation/)).toBeVisible();
  await expect(page.getByText("alice")).toBeVisible();
  await page.getByRole("button", { name: "Disconnect", exact: true }).click(); // asks to confirm
  await page.getByRole("button", { name: /Stop 2 projects and disconnect/ }).click();
  await expect.poll(() => disconnected).toBe(true);
});
