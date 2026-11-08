export type ChangesetStatus = "draft" | "saved" | "published" | "approved" | "rejected";

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
  /** Container node this one is placed in. */
  parent_id?: string;
  /** Set from the schema when the node folds into its parent (e.g. subnets). */
  nest_field?: string;
  /** Container colour key (see BLOCK_COLORS). */
  color?: string;
  /** .tf file this resource is written to (default main.tf). */
  filename?: string;
  /** "resource" for a raw provider resource (D-040); absent or "module" for a module call. */
  kind?: "module" | "resource";
  /** Raw resources only, e.g. azurerm_resource_group. */
  resource_type?: string;
  /** Set when the node edits a module call already in the repository (D-036). */
  existing?: { folder: string; filename: string; label: string };
  /** Module source line; kept from the repository for an imported node. */
  source?: string;
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
  /** What the changeset's base canvas generated; empty for a brand-new changeset. */
  base_files?: GeneratedFile[];
  /** .tf files already in the project folder (shown unchanged). */
  repo_files?: GeneratedFile[];
}
