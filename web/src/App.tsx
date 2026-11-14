import { useState, useEffect, useCallback } from "react";
import { useAuth } from "./hooks/useAuth";
import { useProjects } from "./hooks/useProjects";
import { useGraph } from "./hooks/useGraph";
import { useChangesetList } from "./hooks/useReview";
import { publishChangesetById, useChangeset } from "./hooks/useChangeset";
import { useCatalog } from "./hooks/useCatalog";
import { reviseChangeset } from "./hooks/useReview";
import { useHclPreview } from "./hooks/useHclPreview";
import { AuthGuard } from "./components/AuthGuard";
import { NavRail } from "./components/NavRail";
import { newRawResource } from "./lib/rawResource";
import { useRawResources, type RawResourceTemplate } from "./hooks/useRawResources";
import { PublishDialog } from "./components/PublishDialog";
import { ResourcesPanel } from "./components/ResourcesPanel";
import { TopBar } from "./components/TopBar";
import { EditableCanvas } from "./components/EditableCanvas";
import { Palette } from "./components/palette/Palette";
import { useProviders } from "./hooks/useProviders";
import { HclPreviewPanel } from "./components/HclPreviewPanel";
import { DraftOrDetailPanel } from "./components/DetailPanel";
import { canApproveRole, useTeam } from "./hooks/useGitHub";
import { CreateStackDialog, ApplyStackDialog, StackList } from "./components/StackTemplates";
import { mergeVariables, useStackTemplates, type StackSummary } from "./hooks/useStackTemplates";
import { putGeneration } from "./hooks/useGeneration";
import { useChangesetFindings, useServerFindings } from "./hooks/useValidation";
import { ChangesetBar } from "./components/ChangesetBar";
import { CanvasLegend } from "./components/CanvasLegend";
import { HomePage } from "./components/HomePage";
import { ReviewView } from "./components/ReviewView";
import { ChangesetsView } from "./components/ChangesetsView";
import type { ModuleSummary, CatalogModule } from "./types/catalog";
import type { GraphNode, GraphExternal } from "./types/graph";
import { toCanvasNode, type ImportedNode } from "./lib/importExisting";
import type { CanvasNode } from "./types/changeset";
import { GraphContext } from "./contexts/GraphContext";
import { GenerationContext } from "./contexts/GenerationContext";
import { ProjectContext } from "./contexts/ProjectContext";
import { catalogPaths } from "./lib/catalogApi";
import { useGeneration } from "./hooks/useGeneration";
import { buildProjectHash, parseProjectHash, type ProjectView } from "./lib/projectHash";
import { ProblemsList } from "./components/ProblemsList";
import { EnvCompareView } from "./components/EnvCompareView";
import { ProjectPage } from "./components/ProjectPage";
import { targetFor } from "./lib/generation";

const DEFAULT_ENVS = ["dev", "staging", "prod"];

export function App() {
  const { user, devUsers, devMode, providers, signinError, loading: authLoading } = useAuth();
  const { projects, loading: projectsLoading, error: projectsError, create, remove, refresh: refreshProjects } = useProjects();
  const [projectId, setProjectId] = useState<string | null>(() => {
    return parseProjectHash(window.location.hash).projectId;
  });
  const [env, setEnv] = useState("dev");
  const [addRequest, setAddRequest] = useState<{ name: string; version: string; nonce: number } | null>(null);
  const [selectedModule, setSelectedModule] = useState<ModuleSummary | null>(null);
  const [activeView, setActiveView] = useState<string>(() => parseProjectHash(window.location.hash).view);
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  // A repository resource opened for editing stays out of the draft until a value really changes.
  const [staged, setStaged] = useState<CanvasNode | null>(null);
  const [selectedExistingNode, setSelectedExistingNode] = useState<GraphNode | GraphExternal | null>(null);
  const [leftTab, setLeftTab] = useState<"modules" | "resources" | "stacks">("modules");
  const [reviewTick, setReviewTick] = useState(0);
  const [draftReload, setDraftReload] = useState(0);
  const { modules, loading: catalogLoading, error: catalogError, reload: reloadCatalog } = useCatalog(projectId);
  const { providers: registry, categories } = useProviders();
  const { settings: generation, reload: reloadGeneration } = useGeneration(projectId);
  const envOptions = generation.environments?.length ? generation.environments : DEFAULT_ENVS;
  // An environment the project does not have (the list changed, or another project was opened) is replaced.
  useEffect(() => {
    if (!envOptions.includes(env)) setEnv(envOptions.includes("dev") ? "dev" : envOptions[0]);
  }, [envOptions, env]);
  const { templates: rawTemplates, reload: reloadRaw } = useRawResources(projectId);

  // Keep the URL hash in sync so a refresh stays on the same project.
  // The screen is in it too: the rail views, and the project page with its tab.
  useEffect(() => {
    const cur = parseProjectHash(window.location.hash);
    const view = (["canvas", "changesets", "review", "hcl", "compare", "project"].includes(activeView) ? activeView : "canvas") as ProjectView;
    const next = buildProjectHash(projectId, view, cur.projectId === projectId ? cur.tab : "project");
    if (window.location.hash !== `#${next}` && !(next === "" && window.location.hash === "")) window.location.hash = next;
  }, [projectId, activeView]);

  // Browser back/forward changes the hash; follow it.
  useEffect(() => {
    const onHash = () => {
      const h = parseProjectHash(window.location.hash);
      setProjectId(h.projectId);
      if (h.projectId) setActiveView(h.view);
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
    addNodes,
    updateNode,
    removeNode,
    saveVersion,
  } = useChangeset(projectId, env, draftReload);

  // "Copy to <env>" on the compare screen: open that environment's draft and add the proposed node to it.
  const [pendingCopy, setPendingCopy] = useState<{ node: ImportedNode; env: string } | null>(null);
  useEffect(() => {
    if (!pendingCopy || env !== pendingCopy.env || csLoading || !changeset || changeset.env !== pendingCopy.env) return;
    addNode(toCanvasNode(pendingCopy.node, canvas.nodes.length));
    setPendingCopy(null);
  }, [pendingCopy, env, csLoading, changeset, canvas.nodes.length, addNode]);

  const { preview, loading: previewLoading, error: previewError, loadPreview } =
    useHclPreview(projectId, changeset?.id ?? null);

  // Keep an open preview in step with edits: refresh once a save has landed.
  useEffect(() => {
    if (activeView === "hcl" && !saving) loadPreview();
  }, [saving]);

  const getModuleSchema = useCallback(
    async (name: string, version: string): Promise<CatalogModule | null> => {
      try {
        const res = await fetch(`/api${catalogPaths(projectId).read}/${encodeURIComponent(name)}/${encodeURIComponent(version)}`);
        if (!res.ok) return null;
        return res.json();
      } catch {
        return null;
      }
    },
    [projectId]
  );

  const editingDraftNode = editingNodeId
    ? canvas.nodes.find((n) => n.id === editingNodeId) ?? (staged?.id === editingNodeId ? staged : undefined)
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

  // "Edit this resource": bring the repository's module call onto the canvas as
  // a node that points back at its block, so approving replaces only that block.
  const handleEditExisting = useCallback(async (g: GraphNode) => {
    if (!projectId) return;
    if (canvas.nodes.some((n) => n.id === g.id)) {
      setEditingNodeId(g.id);
      setSelectedExistingNode(null);
      return;
    }
    try {
      const res = await fetch(`/api/projects/${projectId}/import`);
      if (!res.ok) throw new Error((await res.json().catch(() => null))?.error ?? `HTTP ${res.status}`);
      const { nodes } = (await res.json()) as { nodes: ImportedNode[] };
      const found = nodes.find((n) => n.id === g.id);
      if (!found) throw new Error("This resource is no longer in the repository files.");
      setStaged(toCanvasNode(found, canvas.nodes.length, modules.find((m) => m.name === found.module_name)));
      setEditingNodeId(found.id);
      setSelectedExistingNode(null);
    } catch (err) {
      setNotice(err instanceof Error ? err.message : "Could not open this resource for editing");
    }
  }, [projectId, canvas.nodes, modules]);

  // A raw provider resource: placed on the canvas empty and filled in the right panel.
  const handleAddRaw = useCallback((t: RawResourceTemplate) => {
    const folders = graph?.folders.map((f) => f.name) ?? [];
    const folder = folders.includes(t.target_folder) ? t.target_folder : folders[0] ?? "";
    const node = newRawResource(folder, canvas.nodes.length, t);
    // A project that keeps everything in one place puts raw resources there too.
    if (generation.layout.mode === "single") {
      const to = targetFor(generation, "", folder);
      node.target_folder = to.folder;
      if (to.filename) node.filename = to.filename;
    }
    addNode(node);
    setEditingNodeId(node.id);
    setSelectedExistingNode(null);
  }, [graph, canvas.nodes.length, addNode, generation]);

  // The first real change to a staged resource moves it into the draft; an unchanged form does not.
  const updateEditingNode = useCallback((id: string, updates: Partial<CanvasNode>) => {
    if (staged && staged.id === id && !canvas.nodes.some((n) => n.id === id)) {
      const changed = Object.entries(updates).some(
        ([k, v]) => JSON.stringify(v) !== JSON.stringify((staged as unknown as Record<string, unknown>)[k]),
      );
      if (!changed) return;
      addNode({ ...staged, ...updates });
      setStaged(null);
      return;
    }
    updateNode(id, updates);
  }, [staged, canvas.nodes, addNode, updateNode]);

  const handleCloseRightPanel = useCallback(() => {
    setStaged(null);
    setEditingNodeId(null);
    setSelectedExistingNode(null);
  }, []);

  // The saved version being published from the Changesets page.
  const [publishTarget, setPublishTarget] = useState<{ id: string; title: string; count: number; env: string } | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  // Save a version of the draft; it is published later from Changesets.
  const handleSave = useCallback(async () => {
    const r = await saveVersion();
    if (typeof r === "string") {
      setNotice(r);
      return;
    }
    setReviewTick((t) => t + 1);
    setNotice(`Saved "${r.title}". Publish it from Changesets when it is ready for review.`);
  }, [saveVersion]);

  // Publish the chosen saved version, then show the Review queue where the approver will see it.
  const submitPublish = useCallback(async (title: string) => {
    if (!publishTarget || !projectId) return "Nothing to publish.";
    const result = await publishChangesetById(projectId, publishTarget.id, title);
    if (result === true) {
      setReviewTick((t) => t + 1);
      setNotice(`Sent "${title}" for review.`);
      setActiveView("review");
    }
    return result;
  }, [publishTarget, projectId]);

  // One changeset at a time waits for review: the earlier one is decided first.
  const { rows: changesetRows } = useChangesetList(projectId, reviewTick);
  // The oldest one first, which is the one the server names when it refuses.
  const pendingReview =
    changesetRows
      .filter((r) => r.status === "published")
      .sort((a, b) => String(a.updated_at).localeCompare(String(b.updated_at)))[0] ?? null;

  const handleViewChange = useCallback((v: string) => {
    // HCL Preview is a page of its own, reached from the nav (or "Review diff").
    if (v === "hcl") loadPreview();
    setActiveView(v);
  }, [loadPreview]);

  // What the server says about the canvas as it is now (D-065); asked again after each edit.
  const { team } = useTeam(projectId);
  const canManageProject = !!user?.roles?.includes("devops") || canApproveRole(team?.my_role);
  // Stack templates (8.15): pick resources on the canvas, make a template, apply it to one or more environments.
  const stacks = useStackTemplates(projectId);
  const [picking, setPicking] = useState(false);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [createOpen, setCreateOpen] = useState(false);
  const [applyTarget, setApplyTarget] = useState<StackSummary | null>(null);
  const [pendingStacks, setPendingStacks] = useState<{ env: string; nodes: CanvasNode[] }[]>([]);
  useEffect(() => {
    const next = pendingStacks[0];
    if (!next) return;
    if (env !== next.env) {
      setEnv(next.env);
      return;
    }
    if (csLoading || !changeset || changeset.env !== next.env) return;
    addNodes(next.nodes); // the ids are already unique, and the nodes point at each other by id
    setPendingStacks((p) => p.slice(1));
  }, [pendingStacks, env, csLoading, changeset, addNodes]);
  const serverFindings = useServerFindings(projectId, canvas, env);
  const publishFindings = useChangesetFindings(projectId, publishTarget?.id ?? null);

  const projectName = projects.find((p) => p.id === projectId)?.name ?? "Project";
  const userInitials = user
    ? (user.name ?? user.email ?? "U").slice(0, 2).toUpperCase()
    : "U";

  const showRightPanel = !!(editingDraftNode || selectedExistingNode);

  if (!projectId) {
    return (
      <AuthGuard user={user} loading={authLoading} devUsers={devUsers} providers={providers} signinError={signinError}>
        <HomePage
          projects={projects}
          loading={projectsLoading}
          error={projectsError}
          user={user}
          devSignIn={devMode}
          onProjectsChanged={refreshProjects}
          onOpen={setProjectId}
          onCreate={create}
          onDelete={remove}
        />
      </AuthGuard>
    );
  }

  return (
    <AuthGuard user={user} loading={authLoading} devUsers={devUsers} providers={providers} signinError={signinError}>
      <GraphContext.Provider value={{ graph: graph ?? null, draftNodes: canvas.nodes, modules }}>
      <ProjectContext.Provider value={projectId}>
      <GenerationContext.Provider value={generation}>
        <div className="tf-shell">
          <NavRail
            activeView={activeView}
            onViewChange={handleViewChange}
            userInitials={userInitials}
            onHome={() => setProjectId(null)}
          />

          <main className="tf-main-area">
            <h1 className="sr-only">{projectName} · {activeView}</h1>
            {/* One grid row for the top bar and the notice: a second child would take the 1fr row
                and push the view below it to an implicit row at the bottom of the page. */}
            <div>
            <TopBar
              projectName={projectName}
              env={env}
              envOptions={envOptions}
              onEnvChange={setEnv}
              draftCount={canvas.nodes.length}
              saving={saving}
              onSave={handleSave}
              user={user}
              devSignIn={devMode}
            />

            {notice && (
              <div role="status" className="flex items-center justify-between gap-3 border-b border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] px-4 py-2 text-sm">
                <span>{notice}</span>
                <button className="tf-btn tf-btn--ghost" onClick={() => setNotice(null)}>Dismiss</button>
              </div>
            )}
            </div>
            {createOpen && (
              <CreateStackDialog
                canvas={canvas}
                picked={canvas.nodes.filter((n) => picked.has(n.id)).map((n) => n.id)}
                onCreate={async (name, selected, params) => {
                  await stacks.create(name, canvas, selected, params);
                  setPicking(false);
                  setPicked(new Set());
                }}
                onClose={() => setCreateOpen(false)}
              />
            )}
            {applyTarget && projectId && (
              <ApplyStackDialog
                summary={applyTarget}
                load={stacks.get}
                envs={envOptions}
                currentEnv={env}
                canvasNodes={canvas.nodes}
                graph={graph ?? null}
                onClose={() => setApplyTarget(null)}
                onApply={async (body, chosen) => {
                  const made: { env: string; nodes: CanvasNode[] }[] = [];
                  const patch: { name: string; values: Record<string, string> }[] = [];
                  const startX = canvas.nodes.reduce((m, n) => Math.max(m, n.position_x), 0) + 320;
                  for (const e of chosen) {
                    const r = await stacks.apply(applyTarget.id, {
                      ...body, env: e, offset_x: startX, offset_y: 0,
                      draft_labels: e === env ? canvas.nodes.map((n) => `${n.target_folder}/${n.instance_name}`) : [],
                    });
                    made.push({ env: e, nodes: r.nodes });
                    patch.push(...r.variables);
                  }
                  // Every environment was accepted: now save the variables and add the nodes.
                  if (patch.length > 0) {
                    await putGeneration(projectId, { ...generation, variables: mergeVariables(generation.variables ?? [], patch) });
                    await reloadGeneration();
                  }
                  for (const m of made) {
                    if (m.env === env) addNodes(m.nodes);
                    else setPendingStacks((p) => [...p, m]);
                  }
                  setNotice(`Added ${made.reduce((s, m) => s + m.nodes.length, 0)} resources to the draft of ${chosen.join(", ")}.`);
                }}
              />
            )}
            {publishTarget && (
              <PublishDialog
                draftCount={publishTarget.count}
                env={publishTarget.env}
                initialTitle={publishTarget.title}
                onPublish={submitPublish}
                onClose={() => setPublishTarget(null)}
                blockedBy={pendingReview ? { id: pendingReview.id, title: pendingReview.title } : null}
                onOpenReview={() => setActiveView("review")}
                findings={publishFindings.findings}
              />
            )}

            {activeView === "changesets" && projectId && (
              <ChangesetsView
                projectId={projectId}
                refreshKey={reviewTick}
                onPublish={(row) =>
                  setPublishTarget({ id: row.id, title: row.title, count: row.canvas?.nodes?.length ?? 0, env: row.env })
                }
                onRevise={async (csId, csEnv) => {
                  await reviseChangeset(projectId, csId);
                  setEnv(csEnv);
                  setDraftReload((k) => k + 1);
                  setActiveView("canvas");
                }}
              />
            )}
            {activeView === "project" && projectId && (
              <ProjectPage
                projectId={projectId}
                user={user}
                modules={modules}
                isAdmin={!!user?.roles?.includes("devops")}
                onSaved={reloadGeneration}
                onCatalogChanged={() => {
                  reloadCatalog();
                  reloadRaw();
                }}
                onDeleted={() => {
                  setProjectId(null);
                  refreshProjects();
                }}
              />
            )}
            {activeView === "compare" && projectId && (
              <EnvCompareView
                projectId={projectId}
                envs={envOptions}
                current={env}
                onCopy={(node, to) => {
                  setPendingCopy({ node: node as ImportedNode, env: to });
                  setEnv(to);
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

            <div
              className="tf-workspace"
              style={{ display: ["changesets", "review", "hcl", "project", "compare"].includes(activeView) ? "none" : undefined }}
            >
              {/* Catalog sidebar */}
              {activeView === "canvas" && (
                <aside className="flex w-[280px] shrink-0 flex-col overflow-hidden border-r border-[var(--tf-color-border)] bg-[var(--tf-color-surface)]">
                  <div role="tablist" className="flex border-b border-[var(--tf-color-border)]">
                    {(["modules", "resources", "stacks"] as const).map((t) => (
                      <button
                        key={t}
                        role="tab"
                        aria-selected={leftTab === t}
                        onClick={() => setLeftTab(t)}
                        className={`flex-1 px-3 py-2.5 text-sm font-medium ${
                          leftTab === t
                            ? "border-b-2 border-[var(--tf-color-action)] text-[var(--tf-color-text)]"
                            : "text-[var(--tf-color-text-muted)] hover:text-[var(--tf-color-text)]"
                        }`}
                      >
                        {t === "modules" ? "Palette" : t === "resources" ? "In repository" : "Stacks"}
                      </button>
                    ))}
                  </div>
                  {leftTab === "modules" ? (
                    <Palette
                      modules={modules}
                      loading={catalogLoading}
                      error={catalogError}
                      rawTemplates={rawTemplates}
                      projectProviders={generation.providers}
                      registry={registry}
                      categories={categories}
                      selected={selectedModule?.name ?? null}
                      disabled={!graph}
                      onSelect={setSelectedModule}
                      onAddRaw={handleAddRaw}
                      onAdd={(name, version) => setAddRequest({ name, version, nonce: Date.now() })}
                    />
                  ) : leftTab === "stacks" ? (
                    <StackList
                      templates={stacks.templates}
                      canManage={canManageProject}
                      picking={picking}
                      pickedCount={picked.size}
                      onStartPicking={() => {
                        if (!picking) {
                          setPicking(true);
                          setPicked(new Set());
                          handleCloseRightPanel();
                        } else if (picked.size > 0) setCreateOpen(true);
                      }}
                      onCancelPicking={() => { setPicking(false); setPicked(new Set()); }}
                      onApply={setApplyTarget}
                      onDelete={(t) => { if (window.confirm(`Delete the stack template "${t.name}"? Resources already made from it stay.`)) void stacks.remove(t.id); }}
                    />
                  ) : (
                    <ResourcesPanel
                      graph={graph ?? null}
                      drafts={canvas.nodes}
                      selectedId={editingNodeId ?? selectedExistingNode?.id ?? null}
                      onSelect={handleNodeSelect}
                      drift={{ projectId, env, canManage: !!user?.roles?.includes("devops") || canApproveRole(team?.my_role) }}
                    />
                  )}
                </aside>
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
                    Could not load the project: {error.message}
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
                    findings={serverFindings}
                    addRequest={addRequest}
                    picking={{ active: picking, ids: picked, toggle: (id) => setPicked((s) => { const n = new Set(s); if (n.has(id)) n.delete(id); else n.add(id); return n; }) }}
                  />
                )}

                {activeView === "canvas" && <ProblemsList findings={serverFindings} nodes={canvas.nodes} onSelect={handleEditNode} />}

                <CanvasLegend />

                <ChangesetBar
                  draftCount={canvas.nodes.length}
                  onReviewDiff={() => handleViewChange("hcl")}
                  onSave={handleSave}
                />
              </div>

              {/* Unified right panel — info OR edit form, same visual structure */}
              {showRightPanel && (
                <DraftOrDetailPanel
                  editingDraftNode={editingDraftNode ?? undefined}
                  selectedExistingNode={selectedExistingNode ?? undefined}
                  projectId={projectId}
                  onClose={handleCloseRightPanel}
                  onRemoveNode={(id) => { if (canvas.nodes.some((n) => n.id === id)) removeNode(id); handleCloseRightPanel(); }}
                  onUpdateNode={updateEditingNode}
                  onEditExisting={handleEditExisting}
                  folders={graph?.folders.map((f) => f.name) ?? []}
                  allNodes={canvas.nodes}
                  modules={modules}
                  findings={serverFindings}
                />
              )}

            </div>

            {activeView === "hcl" && (
              <HclPreviewPanel
                full
                preview={preview}
                loading={previewLoading}
                error={previewError}
                onLoad={loadPreview}
                onClose={() => setActiveView("canvas")}
              />
            )}
          </main>
        </div>

      </GenerationContext.Provider>
      </ProjectContext.Provider>
      </GraphContext.Provider>
    </AuthGuard>
  );
}
