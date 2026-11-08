import { useState, useEffect, useCallback } from "react";
import { useAuth } from "./hooks/useAuth";
import { useProjects } from "./hooks/useProjects";
import { useGraph } from "./hooks/useGraph";
import { useChangeset } from "./hooks/useChangeset";
import { useHclPreview } from "./hooks/useHclPreview";
import { AuthGuard } from "./components/AuthGuard";
import { UserMenu } from "./components/UserMenu";
import { ProjectSelector } from "./components/ProjectSelector";
import { Canvas } from "./components/Canvas";
import { EditableCanvas } from "./components/EditableCanvas";
import { CatalogPanel } from "./components/catalog/CatalogPanel";
import { ModuleForm } from "./components/catalog/ModuleForm";
import { HclPreviewPanel } from "./components/HclPreviewPanel";
import type { ModuleSummary, CatalogModule } from "./types/catalog";
import { GraphContext } from "./contexts/GraphContext";

export function App() {
  const { user, loading: authLoading } = useAuth();
  const { projects, loading: projectsLoading } = useProjects();
  const [projectId, setProjectId] = useState<string | null>(null);
  const [env, setEnv] = useState("dev");
  const [selectedModule, setSelectedModule] = useState<ModuleSummary | null>(null);
  const [showCatalog, setShowCatalog] = useState(false);
  const [editMode, setEditMode] = useState(false);
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  const [showPreview, setShowPreview] = useState(false);

  useEffect(() => {
    if (projects.length > 0 && !projectId) {
      setProjectId(projects[0].id);
    }
  }, [projects, projectId]);

  const { data: graph, isLoading, error } = useGraph(projectId, env);

  const {
    changeset,
    canvas,
    loading: csLoading,
    saving,
    addNode,
    updateNode,
    removeNode,
    publish,
  } = useChangeset(projectId, env);

  const { preview, loading: previewLoading, error: previewError, loadPreview } =
    useHclPreview(projectId, changeset?.id ?? null);

  const getModuleSchema = useCallback(
    async (name: string, version: string): Promise<CatalogModule | null> => {
      try {
        const res = await fetch(`/api/catalog/${name}/${version}`);
        if (!res.ok) return null;
        return res.json();
      } catch {
        return null;
      }
    },
    []
  );

  const editingDraftNode = editingNodeId
    ? canvas.nodes.find((n) => n.id === editingNodeId)
    : null;

  const handleEditNode = useCallback((id: string) => {
    setEditingNodeId(id);
  }, []);

  const handlePublish = useCallback(async () => {
    const title = prompt("Changeset title:");
    if (!title) return;
    await publish(title);
  }, [publish]);

  return (
    <AuthGuard user={user} loading={authLoading}>
      <GraphContext.Provider value={graph ?? null}>
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
              onClick={() => { setEditMode(!editMode); if (!editMode) setShowCatalog(true); }}
              style={{
                padding: "var(--tf-space-1) var(--tf-space-3)",
                fontSize: "var(--tf-text-small)",
                cursor: "pointer",
                borderRadius: "var(--tf-radius-s)",
                border: editMode ? "none" : "1px solid var(--tf-color-accent)",
                background: editMode ? "var(--tf-color-accent)" : "transparent",
                color: editMode ? "#fff" : "var(--tf-color-accent)",
                fontWeight: 600,
              }}
            >
              {editMode ? "Editing" : "Edit"}
            </button>
            {editMode && canvas.nodes.length > 0 && (
              <>
                <button
                  onClick={() => { setShowPreview(!showPreview); if (!showPreview) loadPreview(); }}
                  style={{
                    padding: "var(--tf-space-1) var(--tf-space-3)",
                    fontSize: "var(--tf-text-small)",
                    cursor: "pointer",
                    borderRadius: "var(--tf-radius-s)",
                    border: "1px solid var(--tf-color-border)",
                    background: showPreview ? "var(--tf-color-action)" : "transparent",
                    color: showPreview ? "#fff" : "var(--tf-color-text-secondary)",
                    fontWeight: 500,
                  }}
                >
                  HCL Preview
                </button>
                <button
                  onClick={handlePublish}
                  style={{
                    padding: "var(--tf-space-1) var(--tf-space-3)",
                    fontSize: "var(--tf-text-small)",
                    cursor: "pointer",
                    borderRadius: "var(--tf-radius-s)",
                    border: "none",
                    background: "var(--tf-color-good)",
                    color: "#fff",
                    fontWeight: 600,
                  }}
                >
                  Publish
                </button>
              </>
            )}
            {!editMode && (
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
            )}
            {saving && (
              <span style={{ fontSize: "var(--tf-text-xs)", color: "var(--tf-color-text-muted)" }}>
                Saving…
              </span>
            )}
            {user && <UserMenu user={user} />}
          </div>
        </header>
        <main style={{ flex: 1, display: "flex", position: "relative", overflow: "hidden" }}>
          {(showCatalog || editMode) && (
            <CatalogPanel
              onSelect={setSelectedModule}
              selected={selectedModule?.name ?? null}
            />
          )}
          {!editMode && showCatalog && selectedModule && (
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
          {editMode && editingDraftNode && (
            <div
              style={{
                width: 400,
                borderRight: "1px solid var(--tf-color-border)",
                overflow: "auto",
                flexShrink: 0,
                display: "flex",
                flexDirection: "column",
              }}
            >
              <div style={{
                padding: "var(--tf-space-3)",
                borderBottom: "1px solid var(--tf-color-border)",
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
              }}>
                <div>
                  <div style={{ fontWeight: 600, fontSize: "var(--tf-text-body)" }}>
                    {editingDraftNode.instance_name}
                  </div>
                  <div style={{ fontSize: "var(--tf-text-small)", color: "var(--tf-color-text-muted)" }}>
                    {editingDraftNode.module_name} / {editingDraftNode.module_version}
                    <span style={{ marginLeft: "var(--tf-space-2)" }}>→ {editingDraftNode.target_folder}/</span>
                  </div>
                </div>
                <button
                  onClick={() => setEditingNodeId(null)}
                  style={{
                    padding: "var(--tf-space-1) var(--tf-space-2)",
                    fontSize: "var(--tf-text-small)",
                    cursor: "pointer",
                    borderRadius: "var(--tf-radius-s)",
                    border: "1px solid var(--tf-color-border)",
                    background: "transparent",
                    color: "var(--tf-color-text-secondary)",
                  }}
                >
                  Close
                </button>
              </div>
              <div style={{ flex: 1, overflow: "auto" }}>
                <ModuleForm
                  moduleName={editingDraftNode.module_name}
                  moduleVersion={editingDraftNode.module_version}
                  projectId={projectId}
                  draftNodeId={editingDraftNode.id}
                  initialValues={editingDraftNode.values}
                  onValuesChange={(values) => updateNode(editingDraftNode.id, { values })}
                />
              </div>
            </div>
          )}
          <div style={{ flex: 1, position: "relative" }}>
            {(isLoading || projectsLoading || csLoading) && (
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
            {graph && !editMode && <Canvas graph={graph} />}
            {graph && editMode && (
              <EditableCanvas
                graph={graph}
                canvas={canvas}
                onAddNode={addNode}
                onUpdateNode={updateNode}
                onRemoveNode={removeNode}
                onEditNode={handleEditNode}
                getModuleSchema={getModuleSchema}
              />
            )}
          </div>
          {showPreview && editMode && (
            <HclPreviewPanel
              preview={preview}
              loading={previewLoading}
              error={previewError}
              onLoad={loadPreview}
              onClose={() => setShowPreview(false)}
            />
          )}
        </main>
      </div>
      </GraphContext.Provider>
    </AuthGuard>
  );
}
