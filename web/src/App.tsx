import { useState, useEffect } from "react";
import { useAuth } from "./hooks/useAuth";
import { useProjects } from "./hooks/useProjects";
import { useGraph } from "./hooks/useGraph";
import { AuthGuard } from "./components/AuthGuard";
import { UserMenu } from "./components/UserMenu";
import { ProjectSelector } from "./components/ProjectSelector";
import { Canvas } from "./components/Canvas";
import { CatalogPanel } from "./components/catalog/CatalogPanel";
import { ModuleForm } from "./components/catalog/ModuleForm";
import type { ModuleSummary } from "./types/catalog";

export function App() {
  const { user, loading: authLoading } = useAuth();
  const { projects, loading: projectsLoading } = useProjects();
  const [projectId, setProjectId] = useState<string | null>(null);
  const [env, setEnv] = useState("dev");
  const [selectedModule, setSelectedModule] = useState<ModuleSummary | null>(null);
  const [showCatalog, setShowCatalog] = useState(false);

  useEffect(() => {
    if (projects.length > 0 && !projectId) {
      setProjectId(projects[0].id);
    }
  }, [projects, projectId]);

  const { data: graph, isLoading, error } = useGraph(projectId, env);

  return (
    <AuthGuard user={user} loading={authLoading}>
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
          {!projectsLoading && (
            <ProjectSelector
              projects={projects}
              selected={projectId}
              onSelect={setProjectId}
            />
          )}
          <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: "var(--tf-space-3)" }}>
            <div style={{ display: "flex", gap: "var(--tf-space-2)" }}>
              {["dev", "prod"].map((e) => (
                <button
                  key={e}
                  onClick={() => setEnv(e)}
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
            <button
              onClick={() => setShowCatalog(!showCatalog)}
              style={{
                padding: "var(--tf-space-1) var(--tf-space-3)",
                fontSize: "var(--tf-text-small)",
                cursor: "pointer",
                borderRadius: "var(--tf-radius-s)",
                border: showCatalog ? "none" : "1px solid var(--tf-color-border)",
                background: showCatalog ? "var(--tf-color-action)" : "transparent",
                color: showCatalog ? "#fff" : "var(--tf-color-text-secondary)",
                fontWeight: 500,
              }}
            >
              Catalog
            </button>
            {user && <UserMenu user={user} />}
          </div>
        </header>
        <main style={{ flex: 1, display: "flex", position: "relative", overflow: "hidden" }}>
          {showCatalog && (
            <CatalogPanel
              onSelect={setSelectedModule}
              selected={selectedModule?.name ?? null}
            />
          )}
          {showCatalog && selectedModule && (
            <div
              style={{
                width: 400,
                borderRight: "1px solid var(--tf-color-border)",
                overflow: "auto",
                flexShrink: 0,
              }}
            >
              <ModuleForm
                moduleName={selectedModule.name}
                moduleVersion={selectedModule.latest_version}
                projectId={projectId}
              />
            </div>
          )}
          <div style={{ flex: 1, position: "relative" }}>
            {(isLoading || projectsLoading) && (
              <div style={{
                position: "absolute", inset: 0,
                display: "flex", alignItems: "center", justifyContent: "center",
                color: "var(--tf-color-text-muted)",
              }}>
                Loading graph…
              </div>
            )}
            {error && (
              <div style={{
                position: "absolute", inset: 0,
                display: "flex", alignItems: "center", justifyContent: "center",
                color: "var(--tf-color-bad)",
              }}>
                Error: {error.message}
              </div>
            )}
            {graph && <Canvas graph={graph} />}
          </div>
        </main>
      </div>
    </AuthGuard>
  );
}
