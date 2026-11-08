import { useState, useEffect, useCallback } from "react";
import { useAuth } from "./hooks/useAuth";
import { useProjects } from "./hooks/useProjects";
import { useGraph } from "./hooks/useGraph";
import { useChangeset } from "./hooks/useChangeset";
import { useHclPreview } from "./hooks/useHclPreview";
import { AuthGuard } from "./components/AuthGuard";
import { NavRail } from "./components/NavRail";
import { TopBar } from "./components/TopBar";
import { Canvas } from "./components/Canvas";
import { EditableCanvas } from "./components/EditableCanvas";
import { CatalogPanel } from "./components/catalog/CatalogPanel";
import { HclPreviewPanel } from "./components/HclPreviewPanel";
import { DraftOrDetailPanel } from "./components/DetailPanel";
import { ChangesetBar } from "./components/ChangesetBar";
import { CanvasLegend } from "./components/CanvasLegend";
import { ThemeToggle } from "./components/ThemeToggle";
import type { ModuleSummary, CatalogModule } from "./types/catalog";
import type { GraphNode, GraphExternal } from "./types/graph";
import { GraphContext } from "./contexts/GraphContext";

const ENV_OPTIONS = ["dev", "staging", "prod"];

export function App() {
  const { user, loading: authLoading } = useAuth();
  const { projects, loading: projectsLoading } = useProjects();
  const [projectId, setProjectId] = useState<string | null>(null);
  const [env, setEnv] = useState("dev");
  const [selectedModule, setSelectedModule] = useState<ModuleSummary | null>(null);
  const [activeView, setActiveView] = useState("canvas");
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  const [selectedExistingNode, setSelectedExistingNode] = useState<GraphNode | GraphExternal | null>(null);
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
    setSelectedExistingNode(null);
  }, []);

  const handleNodeSelect = useCallback((nodeId: string) => {
    if (!graph) return;
    // Check if it's a draft node
    const draft = canvas.nodes.find((n) => n.id === nodeId);
    if (draft) {
      setEditingNodeId(nodeId);
      setSelectedExistingNode(null);
      return;
    }
    // Look up existing graph node
    for (const folder of graph.folders) {
      const found = folder.nodes.find((n) => n.id === nodeId);
      if (found) {
        setSelectedExistingNode(found);
        setEditingNodeId(null);
        return;
      }
    }
    const ext = graph.externals.find((e) => e.id === nodeId);
    if (ext) {
      setSelectedExistingNode(ext);
      setEditingNodeId(null);
    }
  }, [graph, canvas.nodes]);

  const handleCloseRightPanel = useCallback(() => {
    setEditingNodeId(null);
    setSelectedExistingNode(null);
  }, []);

  const handlePublish = useCallback(async () => {
    const title = prompt("Changeset title:");
    if (!title) return;
    await publish(title);
  }, [publish]);

  const projectName = projects.find((p) => p.id === projectId)?.name ?? "Project";
  const userInitials = user
    ? (user.name ?? user.email ?? "U").slice(0, 2).toUpperCase()
    : "U";

  const showRightPanel = !!(editingDraftNode || selectedExistingNode);

  return (
    <AuthGuard user={user} loading={authLoading}>
      <GraphContext.Provider value={{ graph: graph ?? null, draftNodes: canvas.nodes }}>
        <div className="tf-shell">
          <NavRail
            activeView={activeView}
            onViewChange={setActiveView}
            userInitials={userInitials}
          />

          <div className="tf-main-area">
            <TopBar
              projectName={projectName}
              env={env}
              envOptions={ENV_OPTIONS}
              onEnvChange={setEnv}
              draftCount={canvas.nodes.length}
              saving={saving}
              onHclPreview={() => {
                setShowPreview(!showPreview);
                if (!showPreview) loadPreview();
              }}
              onPublish={handlePublish}
            />

            <div className="tf-workspace">
              {/* Catalog sidebar */}
              {activeView === "canvas" && (
                <CatalogPanel
                  onSelect={setSelectedModule}
                  selected={selectedModule?.name ?? null}
                />
              )}

              {/* Canvas area */}
              <div style={{ flex: 1, position: "relative", overflow: "hidden" }}>
                {(isLoading || projectsLoading || csLoading) && (
                  <div style={{
                    position: "absolute", inset: 0,
                    display: "flex", alignItems: "center", justifyContent: "center",
                    color: "var(--tf-color-text-muted)",
                    zIndex: 2,
                  }}>
                    Loading graph...
                  </div>
                )}
                {error && (
                  <div style={{
                    position: "absolute", inset: 0,
                    display: "flex", alignItems: "center", justifyContent: "center",
                    color: "var(--tf-color-error)",
                    zIndex: 2,
                  }}>
                    Error: {error.message}
                  </div>
                )}
                {graph && canvas.nodes.length === 0 && (
                  <Canvas graph={graph} onNodeSelect={handleNodeSelect} />
                )}
                {graph && canvas.nodes.length > 0 && (
                  <EditableCanvas
                    graph={graph}
                    canvas={canvas}
                    onAddNode={addNode}
                    onUpdateNode={updateNode}
                    onRemoveNode={removeNode}
                    onEditNode={handleEditNode}
                    onNodeSelect={handleNodeSelect}
                    getModuleSchema={getModuleSchema}
                  />
                )}

                <CanvasLegend />

                <ChangesetBar
                  draftCount={canvas.nodes.length}
                  onReviewDiff={() => {
                    setShowPreview(!showPreview);
                    if (!showPreview) loadPreview();
                  }}
                  onPublish={handlePublish}
                />
              </div>

              {/* Unified right panel — info OR edit form, same visual structure */}
              {showRightPanel && (
                <DraftOrDetailPanel
                  editingDraftNode={editingDraftNode ?? undefined}
                  selectedExistingNode={selectedExistingNode ?? undefined}
                  projectId={projectId}
                  onClose={handleCloseRightPanel}
                  onRemoveNode={(id) => { removeNode(id); handleCloseRightPanel(); }}
                  onUpdateNode={updateNode}
                />
              )}

              {/* HCL Preview panel */}
              {showPreview && (
                <HclPreviewPanel
                  preview={preview}
                  loading={previewLoading}
                  error={previewError}
                  onLoad={loadPreview}
                  onClose={() => setShowPreview(false)}
                />
              )}
            </div>
          </div>
        </div>

        <ThemeToggle />
      </GraphContext.Provider>
    </AuthGuard>
  );
}
