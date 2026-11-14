import { useCallback, useEffect, useMemo, useRef } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  Panel,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  type NodeTypes,
  type ReactFlowInstance,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { ProjectGraph, GraphNode } from "../types/graph";
import type { CanvasState, CanvasNode } from "../types/changeset";
import type { CatalogModule, ModuleSummary } from "../types/catalog";
import { serverProblems, type Finding } from "../lib/findings";
import { referenceEdges } from "../lib/draftEdges";
import { rawIssues } from "../lib/rawResource";
import { parentsFirst } from "../lib/valueText";
import { findDropTarget, hintMap, layoutDrafts, placeAutoRoots, resolveParents, type HintMap } from "../lib/canvasLayout";
import { defaultValues, nodeIssues, suppliedFields, type Issue } from "../lib/validate";
import { targetFor } from "../lib/generation";
import { useGenerationSettings } from "../contexts/GenerationContext";
import { ResourceNode } from "./nodes/ResourceNode";
import { FolderGroupNode } from "./nodes/FolderGroupNode";
import { ExternalNode } from "./nodes/ExternalNode";
import { DraftNode } from "./nodes/DraftNode";
import { DraftContainerNode } from "./nodes/DraftContainerNode";

/** How a kind of link is named on its arrow. */
const LINK_LABEL: Record<string, string> = { private_link: "private link", private_dns_zone: "private DNS" };

const nodeTypes: NodeTypes = {
  resource: ResourceNode,
  folderGroup: FolderGroupNode,
  external: ExternalNode,
  draft: DraftNode,
  draftContainer: DraftContainerNode,
};

interface Props {
  graph: ProjectGraph;
  canvas: CanvasState;
  onAddNode: (node: CanvasNode) => void;
  onUpdateNode: (id: string, updates: Partial<CanvasNode>) => void;
  onRemoveNode: (id: string) => void;
  onEditNode: (id: string) => void;
  onNodeSelect?: (nodeId: string) => void;
  getModuleSchema: (name: string, version: string) => Promise<CatalogModule | null>;
  /** Catalog summaries; their x-tf-canvas hints drive nesting. */
  modules: ModuleSummary[];
  /** What the server says about the canvas (D-065, 8.13): shown as badges on the nodes. */
  findings?: Finding[];
  /** A request from the palette to add a module without dragging; the nonce makes each request new. */
  addRequest?: { name: string; version: string; nonce: number } | null;
  /** Picking resources for a stack template (8.15): while active, a click on a draft resource picks it instead of opening it. */
  picking?: { active: boolean; ids: Set<string>; toggle: (id: string) => void };
}

export function EditableCanvas({
  graph,
  canvas,
  onAddNode,
  onUpdateNode,
  onRemoveNode,
  onEditNode,
  onNodeSelect,
  getModuleSchema,
  modules,
  findings,
  addRequest,
  picking,
}: Props) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const reactFlowRef = useRef<ReactFlowInstance | null>(null);
  const generation = useGenerationSettings();
  const hints = useMemo(() => hintMap(modules), [modules]);
  const titles = useMemo(() => new Map(modules.map((m) => [m.name, m.title])), [modules]);
  const byName = useMemo(() => new Map(modules.map((m) => [m.name, m])), [modules]);

  useEffect(() => {
    buildMergedGraph(graph, canvas, onEditNode, onRemoveNode, hints, titles, byName, findings ?? []).then(({ nodes: n, edges: e }) => {
      setNodes(n.map((x) => (picking?.ids.has(x.id) ? { ...x, data: { ...x.data, picked: true } } : x)));
      setEdges(e);
    });
  }, [graph, canvas, setNodes, setEdges, onEditNode, onRemoveNode, hints, titles, byName, findings, picking?.ids]);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (node.type === "draft" || node.type === "draftContainer") {
        if (picking?.active) picking.toggle(node.id);
        else onEditNode(node.id);
        return;
      }
      onNodeSelect?.(node.id);
    },
    [onEditNode, onNodeSelect, picking]
  );

  const onDragOver = useCallback((e: React.DragEvent) => {
    if (e.dataTransfer.types.includes("application/terraflow-module")) {
      e.preventDefault();
      e.dataTransfer.dropEffect = "copy";
    }
  }, []);

  // Put a module on the canvas at a position in canvas coordinates (a drop, or the palette's Add).
  const placeModule = useCallback(
    async (name: string, version: string, position: { x: number; y: number }) => {
      const schema = await getModuleSchema(name, version);
      if (!schema) return;

      const moduleFile = schema.schema?.["x-tf-module"]?.filename;
      const to = targetFor(generation, name, schema.target_folder || name, moduleFile);
      const targetFolder = to.folder;
      const instanceName = generateInstanceName(name, canvas.nodes);

      const h = hints.get(name) ?? {};
      const placed = layoutDrafts(canvas.nodes, hints);
      const target = findDropTarget(placed, canvas.nodes, position.x, position.y, name, hints);

      // Always place the node. If it is incomplete (needs a parent, or required
      // inputs) it gets a red border until it is configured in the right panel.
      const values: Record<string, unknown> = defaultValues(
        schema.schema?.properties as Record<string, { default?: unknown }> | undefined,
      );
      if (target && h.parent_ref) values[h.parent_ref.field] = { $expr: h.parent_ref.expr };

      const node: CanvasNode = {
        id: `draft-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        module_name: name,
        module_version: version,
        instance_name: instanceName,
        target_folder: targetFolder,
        values,
        position_x: position.x,
        position_y: position.y,
        ...(target ? { parent_id: target.id } : {}),
        ...(h.nest ? { nest_field: h.nest.field } : {}),
        ...(h.container ? { color: "cyan" } : {}),
        ...(!h.nest && to.filename ? { filename: to.filename } : {}),
      };

      onAddNode(node);
      onEditNode(node.id);
    },
    [canvas.nodes, getModuleSchema, onAddNode, onEditNode, hints, generation]
  );

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const raw = e.dataTransfer.getData("application/terraflow-module");
      if (!raw) return;
      const { name, version } = JSON.parse(raw) as { name: string; version: string; title: string };
      const position = reactFlowRef.current?.screenToFlowPosition({ x: e.clientX, y: e.clientY }) ?? { x: 100, y: 100 };
      void placeModule(name, version, position);
    },
    [placeModule],
  );

  // "Add" in the palette (a button or Enter) for people who do not drag: the centre of what is in view.
  const lastAdd = useRef(0);
  useEffect(() => {
    if (!addRequest || addRequest.nonce === lastAdd.current) return;
    lastAdd.current = addRequest.nonce;
    const el = document.querySelector(".react-flow");
    const r = el?.getBoundingClientRect();
    const position = r && reactFlowRef.current ? reactFlowRef.current.screenToFlowPosition({ x: r.left + r.width / 2, y: r.top + r.height / 2 }) : { x: 100, y: 100 };
    void placeModule(addRequest.name, addRequest.version, position);
  }, [addRequest, placeModule]);

  const onNodeDragStop = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if ((node.type === "draft" || node.type === "draftContainer") && !node.parentId) {
        onUpdateNode(node.id, {
          position_x: node.position.x,
          position_y: node.position.y,
        });
      }
    },
    [onUpdateNode]
  );

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={onNodeClick}
      onNodeDragStop={onNodeDragStop}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onInit={(instance) => { reactFlowRef.current = instance; }}
      fitView
      minZoom={0.1}
      maxZoom={2}
      proOptions={{ hideAttribution: true }}
    >
      <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
      <Controls />
      <Panel position="top-left">
        <div style={{
          padding: "var(--tf-space-2) var(--tf-space-3)",
          background: "var(--tf-color-surface)",
          borderRadius: "var(--tf-radius-m)",
          fontSize: "var(--tf-text-small)",
          color: "var(--tf-color-text-secondary)",
          border: "1px solid var(--tf-color-border)",
        }}>
          {graph.project} / {graph.env}
          {canvas.nodes.length > 0 && (
            <span style={{ marginLeft: "var(--tf-space-2)", color: "var(--tf-color-accent)" }}>
              +{canvas.nodes.length} draft
            </span>
          )}
        </div>
      </Panel>
    </ReactFlow>
  );
}

function generateInstanceName(moduleName: string, existingNodes: CanvasNode[]): string {
  const base = moduleName.replace(/-/g, "_");
  const existing = existingNodes.filter((n) => n.module_name === moduleName);
  if (existing.length === 0) return base;
  return `${base}_${existing.length + 1}`;
}

async function buildMergedGraph(
  graph: ProjectGraph,
  canvas: CanvasState,
  onEdit: (id: string) => void,
  onRemove: (id: string) => void,
  hints: HintMap,
  titles: Map<string, string>,
  byName: Map<string, ModuleSummary>,
  findings: Finding[],
) {
  const linkFields = new Map<string, Record<string, string>>();
  for (const m of byName.values()) if (m.link_fields) linkFields.set(m.name, m.link_fields);
  const { computeLayout: computeLayoutFn } = await import("../layout/elk");
  const layout = await computeLayoutFn(graph);

  const dataNodeIds = new Set<string>();
  const nodeMap = new Map<string, GraphNode>();
  for (const folder of graph.folders) {
    for (const node of folder.nodes) {
      nodeMap.set(node.id, node);
      if (node.kind === "data") dataNodeIds.add(node.id);
    }
  }

  const flowNodes: Node[] = [];
  const flowEdges: Edge[] = [];

  // An imported draft takes the place of the repository node it edits (same id).
  const replaced = new Set(canvas.nodes.filter((n) => n.existing).map((n) => n.id));

  // Where the repository node sat: the draft that replaces it takes its place, so
  // the nodes inside it (parentId) stay where they are.
  const anchors = new Map(layout.nodes.filter((l) => replaced.has(l.id)).map((l) => [l.id, l]));

  for (const ln of layout.nodes) {
    if (dataNodeIds.has(ln.id) || replaced.has(ln.id)) continue;

    if (ln.id === "group:hub") {
      flowNodes.push({
        id: ln.id,
        type: "folderGroup",
        position: { x: ln.x, y: ln.y },
        data: { label: "hub (external)" },
        style: { width: ln.width, height: ln.height },
        draggable: false,
      });
      continue;
    }

    const graphNode = nodeMap.get(ln.id);
    const ext = graph.externals.find((e) => e.id === ln.id);

    if (ext) {
      flowNodes.push({
        id: ln.id,
        type: "external",
        position: { x: ln.x, y: ln.y },
        parentId: ln.parentId,
        extent: ln.parentId ? "parent" as const : undefined,
        data: { external: ext },
        draggable: false,
      });
    } else if (graphNode) {
      flowNodes.push({
        id: ln.id,
        type: ln.isGroup ? "folderGroup" : "resource",
        position: { x: ln.x, y: ln.y },
        parentId: ln.parentId,
        extent: ln.parentId ? "parent" as const : undefined,
        data: ln.isGroup
          ? { label: graphNode.name ?? graphNode.type, node: graphNode }
          : { node: graphNode },
        ...(ln.isGroup ? { style: { width: ln.width, height: ln.height } } : {}),
        draggable: false,
      });
    }
  }

  for (const le of layout.edges) {
    if (dataNodeIds.has(le.from) || dataNodeIds.has(le.to)) continue;
    flowEdges.push({
      id: le.id,
      source: le.from,
      target: le.to,
      type: "default",
      animated: le.kind === "cross-folder-data",
      style: {
        stroke: le.kind === "cross-folder-data"
          ? "var(--tf-color-accent)"
          : "var(--tf-color-text-muted)",
        strokeDasharray: le.kind === "cross-folder-data" ? "5 3" : undefined,
      },
      label: le.kind === "cross-folder-data" ? "data" : undefined,
    });
  }

  // New nodes without a spot go to free space right of everything drawn so far.
  const drawn = layout.nodes.filter((l) => !l.parentId && !replaced.has(l.id));
  const origin = { x: drawn.reduce((m, l) => Math.max(m, l.x + l.width), 0) + 80, y: 0 };
  const firstPass = layoutDrafts(canvas.nodes, hints);
  const draftNodes = placeAutoRoots(canvas.nodes, firstPass, origin);
  const placed = draftNodes === canvas.nodes ? firstPass : layoutDrafts(draftNodes, hints);
  const parentOf = resolveParents(draftNodes, hints);
  const byId = new Map(draftNodes.map((n) => [n.id, n]));
  const childCount = new Map<string, number>();
  for (const p of placed) if (p.parentId) childCount.set(p.parentId, (childCount.get(p.parentId) ?? 0) + 1);
  // Repository nodes inside an imported draft container count too, or its empty hint shows over them.
  for (const ln of layout.nodes) {
    if (ln.parentId && !dataNodeIds.has(ln.id)) childCount.set(ln.parentId, (childCount.get(ln.parentId) ?? 0) + 1);
  }

  for (const p of placed) {
    const cn = byId.get(p.id)!;
    const anchor = anchors.get(cn.id);
    const title = cn.kind === "resource" ? cn.resource_type || "Raw resource" : titles.get(cn.module_name) ?? formatTitle(cn.module_name);
    flowNodes.push({
      id: cn.id,
      type: p.container ? "draftContainer" : "draft",
      position: anchor ? { x: anchor.x, y: anchor.y } : { x: p.x, y: p.y },
      ...(anchor
        ? anchor.parentId ? { parentId: anchor.parentId, extent: "parent" as const } : {}
        : p.parentId ? { parentId: p.parentId, extent: "parent" as const } : {}),
      data: {
        canvasNode: cn, title, onEdit, onRemove,
        childCount: childCount.get(cn.id) ?? 0,
        domain: hints.get(cn.module_name)?.domain,
        ...nodeProblems(cn, cn.kind === "resource" ? rawIssues(cn) : nodeIssues(cn, parentOf.get(cn.id), byName.get(cn.module_name), suppliedFields(canvas.nodes, cn.id)), findings),
      },
      style: anchor && anchor.isGroup ? { width: anchor.width, height: anchor.height } : { width: p.w, height: p.h },
      draggable: !p.parentId && !anchor,
    });
  }

  // Edges for module.<name> references in draft values, so a new or edited
  // resource shows what it depends on, including repository resources.
  const flowIds = new Set(flowNodes.map((n) => n.id));
  const existingIds = new Set(nodeMap.keys());
  for (const re of referenceEdges(canvas.nodes, existingIds, linkFields)) {
    if (!flowIds.has(re.from) || !flowIds.has(re.to)) continue;
    flowEdges.push({
      id: re.id,
      source: re.from,
      target: re.to,
      type: "default",
      style: re.link ? { stroke: "var(--tf-color-action)", strokeDasharray: "6 4" } : { stroke: "var(--tf-color-action)" },
      ...(re.link ? { label: LINK_LABEL[re.link] ?? re.link.replace(/_/g, " "), labelStyle: { fontSize: 11 } } : {}),
    });
  }

  return { nodes: parentsFirst(flowNodes), edges: flowEdges };
}

/** The panel's own issues plus what the server adds for the node, and its policy warnings. */
function nodeProblems(cn: CanvasNode, local: Issue[], findings: Finding[]): { issues: Issue[]; warnings: Issue[] } {
  const { errors, warnings } = serverProblems(findings, cn.id, local.map((i) => i.text));
  return { issues: [...local, ...errors.map((text) => ({ text }))], warnings: warnings.map((text) => ({ text })) };
}

function formatTitle(name: string): string {
  return name.replace(/[-_]/g, " ").replace(/(^|\s)\S/g, (c) => c.toUpperCase());
}
