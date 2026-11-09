/// <reference types="vitest/config" />
import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// changeOrigin must stay false: the server refuses a write whose Origin is not its
// Host (D-047), and a rewritten Host would make every POST from the dev page fail.
const apiTarget = { target: process.env.TF_API_TARGET ?? "http://localhost:8080", changeOrigin: false };

// The Go server embeds and serves the built SPA; keep output in dist/.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": path.resolve(__dirname, "src") } },
  build: { outDir: "dist", sourcemap: true },
  test: { environment: "node", include: ["src/**/*.test.ts", "src/**/*.test.tsx"] },
  server: {
    proxy: {
      "/api": apiTarget,
      "/auth": apiTarget,
    },
  },
});
