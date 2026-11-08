import { useCallback, useEffect, useRef, useState } from "react";
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

import type { ProjectGraph, GraphNode, GraphExternal } from "../types/graph";
import type { CanvasState, CanvasNode } from "../types/changeset";
import type { CatalogModule } from "../types/catalog";
import { ResourceNode } from "./nodes/ResourceNode";
import { FolderGroupNode } from "./nodes/FolderGroupNode";
import { ExternalNode } from "./nodes/ExternalNode";
import { DraftNode } from "./nodes/DraftNode";
import { NodeDetail } from "./NodeDetail";

const nodeTypes: NodeTypes = {
  resource: ResourceNode,
  folderGroup: FolderGroupNode,
  external: ExternalNode,
  draft: DraftNode,
};

interface Props {
  graph: ProjectGraph;
  canvas: CanvasState;
  onAddNode: (node: CanvasNode) => void;
  onUpdateNode: (id: string, updates: Partial<CanvasNode>) => void;
  onRemoveNode: (id: string) => void;
  onEditNode: (id: string) => void;
  getModuleSchema: (name: string, version: string) => Promise<CatalogModule | null>;
}

export function EditableCanvas({
  graph,
  canvas,
  onAddNode,
  onUpdateNode,
  onRemoveNode,
  onEditNode,
  getModuleSchema,
}: Props) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const [selectedNode, setSelectedNode] = useState<GraphNode | GraphExternal | null>(null);
  const reactFlowRef = useRef<ReactFlowInstance | null>(null);

  // Build the merged graph: existing (read-only) + draft nodes.
  useEffect(() => {
    buildMergedGraph(graph, canvas, onEditNode, onRemoveNode).then(({ nodes: n, edges: e }) => {
      setNodes(n);
      setEdges(e);
    });
  }, [graph, canvas, setNodes, setEdges, onEditNode, onRemoveNode]);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (node.type === "draft") return;
      for (const folder of graph.folders) {
        const found = folder.nodes.find((n) => n.id === node.id);
        if (found) { setSelectedNode(found); return; }
      }
      const ext = graph.externals.find((e) => e.id === node.id);
      if (ext) setSelectedNode(ext);
    },
    [graph]
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

      const node: CanvasNode = {
        id: `draft-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        module_name: name,
        module_version: version,
        instance_name: instanceName,
        target_folder: targetFolder,
        values: {},
        position_x: position.x,
        position_y: position.y,
      };

      onAddNode(node);
      onEditNode(node.id);
    },
    [canvas.nodes, getModuleSchema, onAddNode, onEditNode]
  );

  // Update node positions when dragged on canvas.
  const onNodeDragStop = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (node.type === "draft") {
        onUpdateNode(node.id, {
          position_x: node.position.x,
          position_y: node.position.y,
        });
      }
    },
    [onUpdateNode]
  );

  return (
    <div style={{ width: "100%", height: "100%", display: "flex" }}>
      <div style={{ flex: 1, position: "relative" }}>
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
      </div>
      {selectedNode && (
        <NodeDetail node={selectedNode} onClose={() => setSelectedNode(null)} />
      )}
    </div>
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

  // Existing nodes (read-only).
  for (const ln of layout.nodes) {
    if (dataNodeIds.has(ln.id)) continue;

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

  // Existing edges.
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

  // Draft nodes.
  for (const cn of canvas.nodes) {
    flowNodes.push({
      id: cn.id,
      type: "draft",
      position: { x: cn.position_x, y: cn.position_y },
      data: { canvasNode: cn, onEdit: onEdit, onRemove: onRemove },
      draggable: true,
    });
  }

  return { nodes: flowNodes, edges: flowEdges };
}
