import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "./design-system/tokens.css";
import "./design-system/shell.css";
import "./design-system/aurora.css";
import "./design-system/polish.css";
import "./tailwind.css";
import { App } from "./App";
import { ThemeToggle } from "./components/ThemeToggle";
import { ErrorBoundary } from "./components/ErrorBoundary";

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
      <ErrorBoundary>
        <App />
        <div className="fixed right-4 top-3 z-[60]">
          <ThemeToggle />
        </div>
      </ErrorBoundary>
    </QueryClientProvider>
  </StrictMode>,
);
