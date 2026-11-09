import { defineConfig } from "@playwright/test";

// Browser tests run against a stack that is already up (see e2e/README.md):
// the Go server with TF_AUTH_DISABLED=true and the Vite dev server.
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  retries: 0,
  workers: 1, // the tests share one database and one dev user list
  reporter: [["list"]],
  use: {
    baseURL: process.env.TF_E2E_URL ?? "http://127.0.0.1:5173",
    viewport: { width: 1440, height: 900 },
    trace: "off",
  },
});
