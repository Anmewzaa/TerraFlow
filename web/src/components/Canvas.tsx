import { useCallback, useEffect } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  type Node,
  type Edge,
  type NodeTypes,
  type EdgeTypes,
  useNodesState,
  useEdgesState,
  Panel,
  BackgroundVariant,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { ProjectGraph, GraphNode } from "../types/graph";
import { computeLayout } from "../layout/elk";
import { ResourceNode } from "./nodes/ResourceNode";
import { FolderGroupNode } from "./nodes/FolderGroupNode";
import { ExternalNode } from "./nodes/ExternalNode";

const nodeTypes: NodeTypes = {
  resource: ResourceNode,
  folderGroup: FolderGroupNode,
  external: ExternalNode,
};

const edgeTypes: EdgeTypes = {};

interface CanvasProps {
  graph: ProjectGraph;
  onNodeSelect?: (nodeId: string) => void;
}

export function Canvas({ graph, onNodeSelect }: CanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  useEffect(() => {
    buildFlowGraph(graph).then(({ nodes: n, edges: e }) => {
      setNodes(n);
      setEdges(e);
    });
  }, [graph, setNodes, setEdges]);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      onNodeSelect?.(node.id);
    },
    [onNodeSelect]
  );

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={onNodeClick}
      fitView
      fitViewOptions={{ padding: 0.35, maxZoom: 1 }}
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
          borderRadius: "var(--tf-radius-control)",
          fontSize: "var(--tf-text-small)",
          color: "var(--tf-color-text-secondary)",
          border: "1px solid var(--tf-color-border)",
        }}>
          {graph.project} / {graph.env}
        </div>
      </Panel>
    </ReactFlow>
  );
}

async function buildFlowGraph(graph: ProjectGraph) {
  const layout = await computeLayout(graph);

  const nodeMap = new Map<string, GraphNode>();
  const dataNodeIds = new Set<string>();
  for (const folder of graph.folders) {
    for (const node of folder.nodes) {
      nodeMap.set(node.id, node);
      if (node.kind === "data") dataNodeIds.add(node.id);
    }
  }

  const flowNodes: Node[] = [];
  const flowEdges: Edge[] = [];

  for (const ln of layout.nodes) {
    if (dataNodeIds.has(ln.id)) continue;

    if (ln.id === "group:hub") {
      flowNodes.push({
        id: ln.id,
        type: "folderGroup",
        position: { x: ln.x, y: ln.y },
        data: { label: "hub (external)" },
        style: { width: ln.width, height: ln.height },
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
      });
    } else if (graphNode) {
      flowNodes.push({
        id: ln.id,
        type: ln.isGroup ? "folderGroup" : "resource",
        position: { x: ln.x, y: ln.y },
        parentId: ln.parentId,
        extent: ln.parentId ? "parent" as const : undefined,
        data: ln.isGroup ? { label: graphNode.name ?? graphNode.type, node: graphNode } : { node: graphNode },
        ...(ln.isGroup ? { style: { width: ln.width, height: ln.height } } : {}),
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

  return { nodes: flowNodes, edges: flowEdges };
}
