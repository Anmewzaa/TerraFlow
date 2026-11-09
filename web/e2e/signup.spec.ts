import { expect, test } from "@playwright/test";
import { DEV_ADMIN, signInAs } from "./helpers";

// Phase 8, M2: the login page, the "waiting for approval" page and the admin's
// approval list. The server's answers are replaced, so no GitHub or Entra is needed.

const json = (status: number, body: unknown) => ({ status, contentType: "application/json", body: JSON.stringify(body) });

test("two providers: the login page offers a button for each", async ({ page }) => {
  await page.route("**/api/me", (r) => r.fulfill(json(401, { error: "not authenticated" })));
  await page.route("**/api/dev/users", (r) => r.fulfill(json(404, {})));
  await page.route("**/api/auth/providers", (r) =>
    r.fulfill(json(200, { providers: [{ id: "entra", label: "Microsoft Entra ID", url: "/auth/login" }, { id: "github", label: "GitHub", url: "/auth/github/login" }] })),
  );
  await page.goto("/");
  await expect(page.getByRole("link", { name: /Sign in with Microsoft Entra ID/ })).toHaveAttribute("href", "/auth/login");
  await expect(page.getByRole("link", { name: /Sign in with GitHub/ })).toHaveAttribute("href", "/auth/github/login");
});

test("a failed sign-in explains itself instead of bouncing to the provider", async ({ page }) => {
  await page.route("**/api/me", (r) => r.fulfill(json(401, { error: "not authenticated" })));
  await page.route("**/api/dev/users", (r) => r.fulfill(json(404, {})));
  await page.route("**/api/auth/providers", (r) => r.fulfill(json(200, { providers: [{ id: "github", label: "GitHub", url: "/auth/github/login" }] })));
  await page.goto("/?signin=rejected");
  await expect(page.getByRole("alert")).toContainText("not approved");
  await expect(page.getByRole("link", { name: /Sign in with GitHub/ })).toBeVisible();
});

test("a pending user sees only the waiting page", async ({ page }) => {
  await page.route("**/api/me", (r) =>
    r.fulfill(json(200, { oid: "github:42", email: "", name: "alice", roles: ["developer"], status: "pending" })),
  );
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Waiting for approval" })).toBeVisible();
  await expect(page.getByText("Signed in as alice")).toBeVisible();
  await expect(page.getByRole("link", { name: "Sign out" })).toHaveAttribute("href", "/auth/logout");
});

test("admin approves a waiting person and rejects another with a reason", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  const user = (oid: string, name: string, status: string) => ({
    oid, name, email: "", provider: "github", status, project_count: 0, created_at: "2026-11-09T10:00:00Z", last_login_at: "2026-11-09T10:00:00Z",
  });
  const rows = [user("github:1", "carol", "active"), user("github:2", "dave", "pending"), user("github:3", "erin", "pending")];
  await page.route("**/api/admin/users", (r) => r.fulfill(json(200, rows)));
  const posted: { url: string; body: unknown }[] = [];
  await page.route("**/api/admin/users/*/status", async (r) => {
    posted.push({ url: decodeURIComponent(new URL(r.request().url()).pathname), body: r.request().postDataJSON() });
    await r.fulfill(json(200, { status: "ok" }));
  });
  await page.getByRole("button", { name: /Admin/ }).first().click();
  await page.getByRole("tab", { name: "users" }).click();
  await expect(page.getByRole("status")).toContainText("2 people are waiting for approval");
  // Waiting people come first.
  await expect(page.getByRole("row").nth(1)).toContainText("dave");

  await page.getByRole("row", { name: /dave/ }).getByRole("button", { name: "Approve" }).click();
  await page.getByRole("row", { name: /erin/ }).getByRole("button", { name: "Reject" }).click();
  const reason = page.getByLabel("Reason to reject erin");
  await reason.fill("not on the team");
  await page.getByRole("button", { name: "Confirm" }).click();
  await expect.poll(() => posted.length).toBe(2);
  expect(posted[0]).toEqual({ url: "/api/admin/users/github:2/status", body: { status: "active", reason: "" } });
  expect(posted[1]).toEqual({ url: "/api/admin/users/github:3/status", body: { status: "rejected", reason: "not on the team" } });
});

test("a refused change shows the server's message", async ({ page }) => {
  await signInAs(page, DEV_ADMIN);
  await page.route("**/api/admin/users", (r) =>
    r.fulfill(json(200, [{ oid: "github:2", name: "dave", email: "", provider: "github", status: "pending", project_count: 0, created_at: "", last_login_at: "" }])),
  );
  await page.route("**/api/admin/users/*/status", (r) => r.fulfill(json(422, { error: "a reason is required" })));
  await page.getByRole("button", { name: /Admin/ }).first().click();
  await page.getByRole("tab", { name: "users" }).click();
  await page.getByRole("button", { name: "Approve" }).click();
  await expect(page.getByRole("alert")).toContainText("a reason is required");
});
