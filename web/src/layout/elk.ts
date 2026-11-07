import ELK, { type ElkNode, type ElkExtendedEdge } from "elkjs/lib/elk.bundled.js";
import type { ProjectGraph } from "../types/graph";

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
  // Collect all edges across folders for the top-level ELK graph.
  const allEdges: ElkExtendedEdge[] = [];
  let edgeIdx = 0;
  const edgeMeta: { from: string; to: string; kind: string }[] = [];

  const dataNodeIds = new Set<string>();
  for (const folder of graph.folders) {
    for (const n of folder.nodes) {
      if (n.kind === "data") dataNodeIds.add(n.id);
    }
  }

  for (const folder of graph.folders) {
    for (const edge of folder.edges ?? []) {
      if (dataNodeIds.has(edge.from) || dataNodeIds.has(edge.to)) continue;
      const id = `e${edgeIdx++}`;
      allEdges.push({ id, sources: [edge.from], targets: [edge.to] });
      edgeMeta.push({ from: edge.from, to: edge.to, kind: edge.kind });
    }
  }

  // Group graph nodes by folder; separate container children.
  const folderTopNodes: Record<string, ElkNode[]> = {};
  for (const folder of graph.folders) {
    folderTopNodes[folder.name] = [];
  }

  // Nodes whose container is set → nested inside another node.
  const containerKids: Record<string, ElkNode[]> = {};

  for (const folder of graph.folders) {
    for (const n of folder.nodes) {
      if (n.kind === "data") continue;
      const elkLeaf: ElkNode = {
        id: n.id,
        width: NODE_WIDTH,
        height: NODE_HEIGHT,
      };
      if (n.container) {
        (containerKids[n.container] ??= []).push(elkLeaf);
      } else {
        folderTopNodes[n.folder]!.push(elkLeaf);
      }
    }
  }

  // Attach container children so ELK treats them as compound nodes.
  for (const folder of graph.folders) {
    for (const elkNode of folderTopNodes[folder.name]!) {
      const kids = containerKids[elkNode.id];
      if (kids) {
        elkNode.children = kids;
        elkNode.layoutOptions = {
          "elk.algorithm": "layered",
          "elk.direction": "DOWN",
          "elk.padding": `[top=${PAD_TOP},left=${PAD},bottom=${PAD},right=${PAD}]`,
        };
      }
    }
  }

  // Build folder group nodes.
  const folderGroups: ElkNode[] = graph.folders.map((folder) => ({
    id: `folder:${folder.name}`,
    children: folderTopNodes[folder.name]!,
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "DOWN",
      "elk.padding": `[top=${PAD_TOP},left=${PAD},bottom=${PAD},right=${PAD}]`,
      "elk.spacing.nodeNode": "20",
    },
  }));

  // Hub externals group.
  if (graph.externals.length > 0) {
    folderGroups.push({
      id: "folder:hub",
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
    children: folderGroups,
    edges: allEdges,
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "RIGHT",
      "elk.spacing.nodeNode": "60",
      "elk.layered.spacing.nodeNodeBetweenLayers": "100",
    },
  };

  const result = await elk.layout(elkRoot);

  // Flatten ELK result → LayoutNode[]. Positions are relative to parent
  // (which is what React Flow expects when parentId is set).
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
