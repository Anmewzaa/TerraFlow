export type NodeKind = "module" | "data" | "resource";
export type NameState = "resolved" | "unknown";
export type EdgeKind = "module-output" | "cross-folder-data" | "nesting";
export type Domain = "network" | "compute" | "data" | "security" | "neutral";

export interface GraphNode {
  id: string;
  folder: string;
  kind: NodeKind;
  type: string;
  domain: Domain;
  name: string | null;
  nameState: NameState;
  source?: string;
  container: string | null;
  inputs: Record<string, unknown>;
}

export interface GraphEdge {
  from: string;
  to: string;
  kind: EdgeKind;
}

export interface GraphFolder {
  name: string;
  nodes: GraphNode[];
  edges: GraphEdge[] | null;
}

export interface GraphExternal {
  id: string;
  kind: string;
  name: string;
  source: string;
}

export interface ProjectGraph {
  project: string;
  env: string;
  folders: GraphFolder[];
  externals: GraphExternal[];
}
