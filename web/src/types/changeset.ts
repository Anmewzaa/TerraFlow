export type ChangesetStatus = "draft" | "published" | "approved" | "rejected";

export interface Changeset {
  id: string;
  project_id: string;
  env: string;
  user_oid: string;
  title: string;
  status: ChangesetStatus;
  canvas: CanvasState;
  reject_reason: string | null;
  reviewer_oid: string | null;
  reviewed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface CanvasState {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
}

export interface CanvasNode {
  id: string;
  module_name: string;
  module_version: string;
  instance_name: string;
  target_folder: string;
  values: Record<string, unknown>;
  position_x: number;
  position_y: number;
}

export interface CanvasEdge {
  id: string;
  from: string;
  to: string;
  kind: string;
}

export interface GeneratedFile {
  folder: string;
  filename: string;
  content: string;
}

export interface GenerateOutput {
  files: GeneratedFile[];
}
