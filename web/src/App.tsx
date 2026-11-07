import { useState } from "react";
import { useGraph } from "./hooks/useGraph";
import { Canvas } from "./components/Canvas";

const DEFAULT_DIR = "samples/iac/project-1";

export function App() {
  const [env, setEnv] = useState("dev");
  const { data: graph, isLoading, error } = useGraph(DEFAULT_DIR, env);

  return (
    <div style={{ height: "100vh", display: "flex", flexDirection: "column" }}>
      <header
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--tf-space-3)",
          padding: "var(--tf-space-2) var(--tf-space-4)",
          borderBottom: "1px solid var(--tf-color-border)",
          background: "var(--tf-color-surface)",
          flexShrink: 0,
        }}
      >
        <h1 style={{ fontSize: "var(--tf-text-body)", margin: 0, fontWeight: 600 }}>
          TerraFlow
        </h1>
        <span style={{ color: "var(--tf-color-text-muted)", fontSize: "var(--tf-text-small)" }}>
          Visualizer
        </span>
        <div style={{ marginLeft: "auto", display: "flex", gap: "var(--tf-space-2)" }}>
          {["dev", "prod"].map((e) => (
            <button
              key={e}
              onClick={() => setEnv(e)}
              className={env === e ? "tf-btn" : "tf-btn-ghost"}
              style={{
                padding: "var(--tf-space-1) var(--tf-space-3)",
                fontSize: "var(--tf-text-small)",
                cursor: "pointer",
                borderRadius: "var(--tf-radius-s)",
                border: env === e ? "none" : "1px solid var(--tf-color-border)",
                background: env === e ? "var(--tf-color-action)" : "transparent",
                color: env === e ? "#fff" : "var(--tf-color-text-secondary)",
                fontWeight: 500,
              }}
            >
              {e}
            </button>
          ))}
        </div>
      </header>
      <main style={{ flex: 1, position: "relative" }}>
        {isLoading && (
          <div style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--tf-color-text-muted)",
          }}>
            Loading graph…
          </div>
        )}
        {error && (
          <div style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--tf-color-bad)",
          }}>
            Error: {error.message}
          </div>
        )}
        {graph && <Canvas graph={graph} />}
      </main>
    </div>
  );
}
