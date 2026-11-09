import fs from "node:fs";
import path from "node:path";
import type { Page } from "@playwright/test";

const axeSource = fs.readFileSync(path.join(process.cwd(), "node_modules/axe-core/axe.min.js"), "utf8");

export const DEV_ADMIN = "Ada Admin";
export const DEV_DEVELOPER = "Bob Developer";

/** Signs in on the local sign-in page by picking a made-up user. */
export async function signInAs(page: Page, name: string, theme: "light" | "dark" = "light") {
  await page.addInitScript((t) => {
    try {
      localStorage.setItem("tf-theme", t);
    } catch {
      /* storage may be blocked */
    }
  }, theme);
  await page.goto("/");
  await page.getByText(name).first().click();
  await page.waitForLoadState("load");
  await page.waitForTimeout(1200); // let the screen fetch and draw (the app keeps connections open, so no networkidle)
}

export type Violation = { id: string; impact: string | null; nodes: number; target: string };

/** Runs axe-core on the current page and returns the violations. */
export async function axeViolations(page: Page): Promise<Violation[]> {
  await page.waitForLoadState("load");
  await page.waitForTimeout(1200); // let the screen fetch and draw (the app keeps connections open, so no networkidle)
  await page.evaluate(axeSource);
  return page.evaluate(async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const res = await (window as any).axe.run(document, { resultTypes: ["violations"] });
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return res.violations.map((v: any) => ({ id: v.id, impact: v.impact, nodes: v.nodes.length, target: String(v.nodes[0].target).slice(0, 100) }));
  });
}
