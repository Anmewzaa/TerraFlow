import { useState, useEffect, useCallback } from "react";
import { useAuth } from "./hooks/useAuth";
import { useProjects } from "./hooks/useProjects";
import { useGraph } from "./hooks/useGraph";
import { useChangeset } from "./hooks/useChangeset";
import { useCatalog } from "./hooks/useCatalog";
import { reviseChangeset } from "./hooks/useReview";
import { useHclPreview } from "./hooks/useHclPreview";
import { AuthGuard } from "./components/AuthGuard";
import { NavRail } from "./components/NavRail";
import { TopBar } from "./components/TopBar";
import { EditableCanvas } from "./components/EditableCanvas";
import { CatalogPanel } from "./components/catalog/CatalogPanel";
import { HclPreviewPanel } from "./components/HclPreviewPanel";
import { DraftOrDetailPanel } from "./components/DetailPanel";
import { ChangesetBar } from "./components/ChangesetBar";
import { CanvasLegend } from "./components/CanvasLegend";
import { ThemeToggle } from "./components/ThemeToggle";
import { HomePage } from "./components/HomePage";
import { ReviewView } from "./components/ReviewView";
import { ChangesetsView } from "./components/ChangesetsView";
import { AuditView } from "./components/AuditView";
import { AdminView } from "./components/AdminView";
import type { ModuleSummary, CatalogModule } from "./types/catalog";
import type { GraphNode, GraphExternal } from "./types/graph";
import { GraphContext } from "./contexts/GraphContext";

const ENV_OPTIONS = ["dev", "staging", "prod"];

export function App() {
  const { user, loading: authLoading } = useAuth();
  const { projects, loading: projectsLoading, error: projectsError, create, remove } = useProjects();
  const [projectId, setProjectId] = useState<string | null>(() => {
    const m = window.location.hash.match(/^#\/p\/(.+)$/);
    return m ? decodeURIComponent(m[1]) : null;
  });
  const [env, setEnv] = useState("dev");
  const [selectedModule, setSelectedModule] = useState<ModuleSummary | null>(null);
  const [activeView, setActiveView] = useState("canvas");
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  const [selectedExistingNode, setSelectedExistingNode] = useState<GraphNode | GraphExternal | null>(null);
  const [showPreview, setShowPreview] = useState(false);
  const [reviewTick, setReviewTick] = useState(0);
  const [draftReload, setDraftReload] = useState(0);
  const { modules } = useCatalog();

  // Keep the URL hash in sync so a refresh stays on the same project.
  useEffect(() => {
    window.location.hash = projectId ? `/p/${encodeURIComponent(projectId)}` : "";
  }, [projectId]);

  // Browser back/forward changes the hash; follow it.
  useEffect(() => {
    const onHash = () => {
      const m = window.location.hash.match(/^#\/p\/(.+)$/);
      setProjectId(m ? decodeURIComponent(m[1]) : null);
    };
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  // A stale/deleted project id falls back to Home.
  useEffect(() => {
    if (projectId && !projectsLoading && !projects.some((p) => p.id === projectId)) {
      setProjectId(null);
    }
  }, [projectId, projects, projectsLoading]);

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
  } = useChangeset(projectId, env, draftReload);

  const { preview, loading: previewLoading, error: previewError, loadPreview } =
    useHclPreview(projectId, changeset?.id ?? null);

  // Keep an open preview in step with edits: refresh once a save has landed.
  useEffect(() => {
    if (showPreview && !saving) loadPreview();
  }, [saving]);

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
    setReviewTick((t) => t + 1);
  }, [publish]);

  const handleViewChange = useCallback((v: string) => {
    if (v === "hcl") {
      setActiveView("canvas");
      setShowPreview(true);
      loadPreview();
      return;
    }
    setActiveView(v);
  }, [loadPreview]);

  const projectName = projects.find((p) => p.id === projectId)?.name ?? "Project";
  const userInitials = user
    ? (user.name ?? user.email ?? "U").slice(0, 2).toUpperCase()
    : "U";

  const showRightPanel = !!(editingDraftNode || selectedExistingNode);

  if (!projectId) {
    return (
      <AuthGuard user={user} loading={authLoading}>
        <HomePage
          projects={projects}
          loading={projectsLoading}
          error={projectsError}
          userName={user?.name ?? undefined}
          onOpen={setProjectId}
          onCreate={create}
          onDelete={remove}
        />
        <ThemeToggle />
      </AuthGuard>
    );
  }

  return (
    <AuthGuard user={user} loading={authLoading}>
      <GraphContext.Provider value={{ graph: graph ?? null, draftNodes: canvas.nodes }}>
        <div className="tf-shell">
          <NavRail
            activeView={activeView}
            onViewChange={handleViewChange}
            userInitials={userInitials}
            onHome={() => setProjectId(null)}
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

            {activeView === "changesets" && projectId && (
              <ChangesetsView
                projectId={projectId}
                refreshKey={reviewTick}
                onRevise={async (csId, csEnv) => {
                  await reviseChangeset(projectId, csId);
                  setEnv(csEnv);
                  setDraftReload((k) => k + 1);
                  setActiveView("canvas");
                }}
              />
            )}
            {activeView === "review" && projectId && (
              <ReviewView
                projectId={projectId}
                user={user}
                refreshKey={reviewTick}
                onDecided={() => setReviewTick((t) => t + 1)}
              />
            )}
            {activeView === "admin" && <AdminView />}
            {activeView === "audit" && projectId && (
              <AuditView projectId={projectId} refreshKey={reviewTick} />
            )}

            <div
              className="tf-workspace"
              style={{ display: ["changesets", "review", "audit", "admin"].includes(activeView) ? "none" : undefined }}
            >
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
                {/* Always the editable canvas: the first drop must work on an empty draft too. */}
                {graph && (
                  <EditableCanvas
                    graph={graph}
                    canvas={canvas}
                    onAddNode={addNode}
                    onUpdateNode={updateNode}
                    onRemoveNode={removeNode}
                    onEditNode={handleEditNode}
                    onNodeSelect={handleNodeSelect}
                    getModuleSchema={getModuleSchema}
                    modules={modules}
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
                  allNodes={canvas.nodes}
                  modules={modules}
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
