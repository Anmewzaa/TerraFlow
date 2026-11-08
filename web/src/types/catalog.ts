/** Schema hints (x-tf-canvas) that drive nesting on the canvas. */
export interface CanvasHints {
  /** Can hold other resources (resource group, virtual network, subnet). */
  container?: boolean;
  /** Module names this resource may be placed in. Empty/absent = anywhere. */
  parents?: string[];
  /** Must be placed in a parent; cannot sit on the bare canvas. */
  parent_required?: boolean;
  /** Folded into the parent module's input map instead of being a module call. */
  nest?: { field: string };
  /** A field filled from the parent, e.g. subnet_id = module.<vnet>.subnet_ids["<name>"]. */
  parent_ref?: { field: string; expr: string };
  domain?: string;
}

export interface ModuleSummary {
  name: string;
  title: string;
  description: string;
  icon: string;
  versions: string[];
  latest_version: string;
  canvas?: CanvasHints;
  /** Required input names from the schema. */
  required?: string[];
}

export interface CatalogModule {
  id: string;
  name: string;
  version: string;
  title: string;
  description: string;
  icon: string;
  target_folder: string;
  schema: JSONSchema;
}

export interface JSONSchema {
  title?: string;
  description?: string;
  type?: string | string[];
  properties?: Record<string, JSONSchema>;
  required?: string[];
  enum?: (string | number | boolean)[];
  default?: unknown;
  pattern?: string;
  minimum?: number;
  maximum?: number;
  items?: JSONSchema;
  additionalProperties?: JSONSchema | boolean;
  "x-tf-module"?: {
    source: string;
    version: string;
    target_folder: string;
    icon: string;
    /** Default .tf file for this module's calls, e.g. rg.tf. */
    filename?: string;
  };
  "x-tf-ui"?: TFUIMetadata;
}

export interface TFUIMetadata {
  groups?: FieldGroup[];
  group?: string;
  order?: number;
  widget?: string;
  hidden?: boolean;
  locked_reason?: string;
  pick?: { module_type: string; output: string };
}

export interface FieldGroup {
  id: string;
  label: string;
  order: number;
}

export interface Draft {
  id: string;
  project_id: string;
  module_id: string;
  user_oid: string;
  values: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}
