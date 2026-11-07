import ELK, { type ElkNode, type ElkExtendedEdge } from "elkjs/lib/elk.bundled.js";
import type { ProjectGraph, GraphNode } from "../types/graph";

const elk = new ELK();

export interface LayoutNode {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  isGroup: boolean;
  parentId?: string;
}

export interface LayoutEdge {
  id: string;
  from: string;
  to: string;
  kind: string;
}

export interface LayoutResult {
  nodes: LayoutNode[];
  edges: LayoutEdge[];
}

const NODE_WIDTH = 260;
const NODE_HEIGHT = 72;
const PAD = 40;
const PAD_TOP = 60;

export async function computeLayout(graph: ProjectGraph): Promise<LayoutResult> {
  // Collect all visible (non-data) nodes.
  const allNodes: GraphNode[] = [];
  const dataNodeIds = new Set<string>();

  for (const folder of graph.folders) {
    for (const n of folder.nodes) {
      if (n.kind === "data") {
        dataNodeIds.add(n.id);
      } else {
        allNodes.push(n);
      }
    }
  }

  // Collect visible edges.
  const allEdges: ElkExtendedEdge[] = [];
  const edgeMeta: { from: string; to: string; kind: string }[] = [];
  let edgeIdx = 0;
  for (const folder of graph.folders) {
    for (const edge of folder.edges ?? []) {
      if (dataNodeIds.has(edge.from) || dataNodeIds.has(edge.to)) continue;
      const id = `e${edgeIdx++}`;
      allEdges.push({ id, sources: [edge.from], targets: [edge.to] });
      edgeMeta.push({ from: edge.from, to: edge.to, kind: edge.kind });
    }
  }

  // Build container tree: parent → children.
  const childrenOf = new Map<string, GraphNode[]>();
  const topLevel: GraphNode[] = [];

  for (const n of allNodes) {
    if (n.container) {
      const kids = childrenOf.get(n.container) ?? [];
      kids.push(n);
      childrenOf.set(n.container, kids);
    } else {
      topLevel.push(n);
    }
  }

  // Recursively build ELK nodes from the container tree.
  function buildElkNode(n: GraphNode): ElkNode {
    const kids = childrenOf.get(n.id);
    const elkNode: ElkNode = {
      id: n.id,
      width: NODE_WIDTH,
      height: NODE_HEIGHT,
    };
    if (kids && kids.length > 0) {
      elkNode.children = kids.map(buildElkNode);
      elkNode.layoutOptions = {
        "elk.algorithm": "layered",
        "elk.direction": "DOWN",
        "elk.padding": `[top=${PAD_TOP},left=${PAD},bottom=${PAD},right=${PAD}]`,
        "elk.spacing.nodeNode": "20",
      };
    }
    return elkNode;
  }

  const topElkNodes = topLevel.map(buildElkNode);

  // Hub externals as a separate group.
  if (graph.externals.length > 0) {
    topElkNodes.push({
      id: "group:hub",
      children: graph.externals.map((ext) => ({
        id: ext.id,
        width: NODE_WIDTH,
        height: NODE_HEIGHT,
      })),
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "DOWN",
        "elk.padding": `[top=${PAD_TOP},left=${PAD},bottom=${PAD},right=${PAD}]`,
        "elk.spacing.nodeNode": "20",
      },
    });
  }

  const elkRoot: ElkNode = {
    id: "root",
    children: topElkNodes,
    edges: allEdges,
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "RIGHT",
      "elk.spacing.nodeNode": "60",
      "elk.layered.spacing.nodeNodeBetweenLayers": "100",
    },
  };

  const result = await elk.layout(elkRoot);

  // Flatten ELK result → LayoutNode[]. Positions are relative to parent.
  const layoutNodes: LayoutNode[] = [];

  function walk(children: ElkNode[] | undefined, parentId?: string) {
    for (const child of children ?? []) {
      const isGroup = (child.children?.length ?? 0) > 0;
      layoutNodes.push({
        id: child.id,
        x: child.x ?? 0,
        y: child.y ?? 0,
        width: child.width ?? NODE_WIDTH,
        height: child.height ?? NODE_HEIGHT,
        isGroup,
        parentId,
      });
      if (child.children) {
        walk(child.children, child.id);
      }
    }
  }

  walk(result.children);

  const layoutEdges: LayoutEdge[] = edgeMeta.map((m, i) => ({
    id: `e${i}`,
    ...m,
  }));

  return { nodes: layoutNodes, edges: layoutEdges };
}
