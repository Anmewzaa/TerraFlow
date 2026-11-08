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
import { referenceEdges } from "../lib/draftEdges";
import { rawIssues } from "../lib/rawResource";
import { parentsFirst } from "../lib/valueText";
import { findDropTarget, hintMap, layoutDrafts, placeAutoRoots, resolveParents, type HintMap } from "../lib/canvasLayout";
import { defaultValues, nodeIssues } from "../lib/validate";
import { ResourceNode } from "./nodes/ResourceNode";
import { FolderGroupNode } from "./nodes/FolderGroupNode";
import { ExternalNode } from "./nodes/ExternalNode";
import { DraftNode } from "./nodes/DraftNode";
import { DraftContainerNode } from "./nodes/DraftContainerNode";

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
}: Props) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const reactFlowRef = useRef<ReactFlowInstance | null>(null);
  const hints = useMemo(() => hintMap(modules), [modules]);
  const titles = useMemo(() => new Map(modules.map((m) => [m.name, m.title])), [modules]);
  const byName = useMemo(() => new Map(modules.map((m) => [m.name, m])), [modules]);

  useEffect(() => {
    buildMergedGraph(graph, canvas, onEditNode, onRemoveNode, hints, titles, byName).then(({ nodes: n, edges: e }) => {
      setNodes(n);
      setEdges(e);
    });
  }, [graph, canvas, setNodes, setEdges, onEditNode, onRemoveNode, hints, titles, byName]);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (node.type === "draft" || node.type === "draftContainer") {
        onEditNode(node.id);
        return;
      }
      onNodeSelect?.(node.id);
    },
    [onEditNode, onNodeSelect]
  );

  const onDragOver = useCallback((e: React.DragEvent) => {
    if (e.dataTransfer.types.includes("application/terraflow-module")) {
      e.preventDefault();
      e.dataTransfer.dropEffect = "copy";
    }
  }, []);

  const onDrop = useCallback(
    async (e: React.DragEvent) => {
      e.preventDefault();
      const raw = e.dataTransfer.getData("application/terraflow-module");
      if (!raw) return;

      const { name, version } = JSON.parse(raw) as { name: string; version: string; title: string };
      const schema = await getModuleSchema(name, version);
      if (!schema) return;

      const targetFolder = schema.target_folder || name;
      const instanceName = generateInstanceName(name, canvas.nodes);

      const position = reactFlowRef.current?.screenToFlowPosition({
        x: e.clientX,
        y: e.clientY,
      }) ?? { x: 100, y: 100 };

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
        ...(!h.nest && schema.schema?.["x-tf-module"]?.filename ? { filename: schema.schema["x-tf-module"].filename } : {}),
      };

      onAddNode(node);
      onEditNode(node.id);
    },
    [canvas.nodes, getModuleSchema, onAddNode, onEditNode, hints]
  );

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
) {
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
        issues: cn.kind === "resource" ? rawIssues(cn) : nodeIssues(cn, parentOf.get(cn.id), byName.get(cn.module_name)),
      },
      style: anchor && anchor.isGroup ? { width: anchor.width, height: anchor.height } : { width: p.w, height: p.h },
      draggable: !p.parentId && !anchor,
    });
  }

  // Edges for module.<name> references in draft values, so a new or edited
  // resource shows what it depends on, including repository resources.
  const flowIds = new Set(flowNodes.map((n) => n.id));
  const existingIds = new Set(nodeMap.keys());
  for (const re of referenceEdges(canvas.nodes, existingIds)) {
    if (!flowIds.has(re.from) || !flowIds.has(re.to)) continue;
    flowEdges.push({
      id: re.id,
      source: re.from,
      target: re.to,
      type: "default",
      style: { stroke: "var(--tf-color-action)" },
    });
  }

  return { nodes: parentsFirst(flowNodes), edges: flowEdges };
}

function formatTitle(name: string): string {
  return name.replace(/[-_]/g, " ").replace(/(^|\s)\S/g, (c) => c.toUpperCase());
}
