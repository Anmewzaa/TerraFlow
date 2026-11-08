import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "./design-system/tokens.css";
import "./design-system/shell.css";
import "./design-system/aurora.css";
import "./tailwind.css";
import { App } from "./App";

// Apply the saved theme before first paint (falls back to the OS preference).
try {
  const saved = localStorage.getItem("tf-theme");
  if (saved === "light" || saved === "dark") document.documentElement.setAttribute("data-theme", saved);
} catch {
  /* storage unavailable */
}

const queryClient = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
