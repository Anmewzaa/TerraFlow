import ELK from "elkjs/lib/elk.bundled.js";
import type { ProjectGraph, GraphNode, GraphEdge } from "../types/graph";

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
const GROUP_PADDING = 40;

export async function computeLayout(graph: ProjectGraph): Promise<LayoutResult> {
  const allNodes: GraphNode[] = [];
  const allEdges: GraphEdge[] = [];

  for (const folder of graph.folders) {
    allNodes.push(...folder.nodes);
    if (folder.edges) allEdges.push(...folder.edges);
  }

  // Build ELK graph with folder groups and container nesting.
  const folderChildren: Record<string, ReturnType<typeof buildElkNode>[]> = {};
  const containerChildren: Record<string, ReturnType<typeof buildElkNode>[]> = {};

  // Initialize folder groups.
  for (const folder of graph.folders) {
    folderChildren[folder.name] = [];
  }

  // Sort nodes: containers first, then assign children.
  for (const node of allNodes) {
    const elkNode = buildElkNode(node);
    if (node.container) {
      if (!containerChildren[node.container]) {
        containerChildren[node.container] = [];
      }
      containerChildren[node.container].push(elkNode);
    } else {
      folderChildren[node.folder]?.push(elkNode);
    }
  }

  // Add external nodes as a special group.
  const externalNodes = graph.externals.map((ext) => ({
    id: ext.id,
    width: NODE_WIDTH,
    height: NODE_HEIGHT,
    labels: [{ text: ext.name }],
  }));

  // Build folder group nodes.
  const topLevelChildren = graph.folders.map((folder) => {
    const children = (folderChildren[folder.name] ?? []).map((n) => {
      // Attach container children if this is a container node.
      const nested = containerChildren[n.id];
      if (nested) {
        return {
          ...n,
          children: nested,
          layoutOptions: {
            "elk.algorithm": "layered",
            "elk.direction": "DOWN",
            "elk.padding": `[top=${GROUP_PADDING},left=${GROUP_PADDING},bottom=${GROUP_PADDING},right=${GROUP_PADDING}]`,
          },
        };
      }
      return n;
    });

    return {
      id: `folder:${folder.name}`,
      labels: [{ text: folder.name }],
      children,
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "DOWN",
        "elk.padding": `[top=${GROUP_PADDING + 20},left=${GROUP_PADDING},bottom=${GROUP_PADDING},right=${GROUP_PADDING}]`,
      },
    };
  });

  if (externalNodes.length > 0) {
    topLevelChildren.push({
      id: "folder:hub",
      labels: [{ text: "hub (external)" }],
      children: externalNodes,
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "DOWN",
        "elk.padding": `[top=${GROUP_PADDING + 20},left=${GROUP_PADDING},bottom=${GROUP_PADDING},right=${GROUP_PADDING}]`,
      },
    });
  }

  // Build edges.
  const elkEdges = allEdges.map((edge, i) => ({
    id: `e${i}`,
    sources: [edge.from],
    targets: [edge.to],
  }));

  const elkGraph = {
    id: "root",
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "RIGHT",
      "elk.spacing.nodeNode": "40",
      "elk.layered.spacing.nodeNodeBetweenLayers": "80",
    },
    children: topLevelChildren,
    edges: elkEdges,
  };

  const layouted = await elk.layout(elkGraph as Parameters<typeof elk.layout>[0]);

  // Flatten to LayoutNode/LayoutEdge.
  const resultNodes: LayoutNode[] = [];
  const resultEdges: LayoutEdge[] = allEdges.map((e, i) => ({
    id: `e${i}`,
    from: e.from,
    to: e.to,
    kind: e.kind,
  }));

  // Add external edges (from data nodes to externals).
  // These are implied: data nodes with provider alias reference externals.

  function flattenNodes(
    children: typeof layouted.children,
    offsetX: number,
    offsetY: number,
    parentId?: string
  ) {
    for (const child of children ?? []) {
      const x = (child.x ?? 0) + offsetX;
      const y = (child.y ?? 0) + offsetY;
      const isGroup = (child.children?.length ?? 0) > 0;

      resultNodes.push({
        id: child.id,
        x,
        y,
        width: child.width ?? NODE_WIDTH,
        height: child.height ?? NODE_HEIGHT,
        isGroup,
        parentId,
      });

      if (child.children) {
        flattenNodes(child.children, x, y, child.id);
      }
    }
  }

  flattenNodes(layouted.children, 0, 0);

  return { nodes: resultNodes, edges: resultEdges };
}

function buildElkNode(node: GraphNode) {
  return {
    id: node.id,
    width: NODE_WIDTH,
    height: NODE_HEIGHT,
    labels: [{ text: node.name ?? node.id }],
  };
}
