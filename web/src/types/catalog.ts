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
  /** The cloud provider the module is written for (optional) and its palette category. */
  provider?: string;
  category?: string;
  /** "project" for a module of the project's own catalog; such a module may hide a global one of the same name (D-060). */
  scope?: "global" | "project";
  shadows_global?: boolean;
  /** What the module offers to inputs that ask for a link (x-tf-link.offers, 8.11). */
  offers?: LinkOffer[];
  /** Inputs that take a link, and the kind each asks for. */
  link_fields?: Record<string, string>;
}

export interface LinkOffer {
  kind: string;
  output: string;
  subresource?: string;
  /** The output belongs to the parent module (a subnet is read from its VNet); {name} is this resource's name. */
  via_parent?: boolean;
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
  /** A secret (a password): never typed into the canvas or stored in the repository; only a reference is accepted. */
  secret?: boolean;
  /**
   * Pick an existing resource: by module type (writes its value), or by `link` (a kind of link the
   * target offers; writes module.<name>.<output>). `fills` names an input set from the offer (the
   * sub-resource), `list` wraps the reference in a list (8.11).
   */
  pick?: { module_type?: string; link?: string; output: string; fills?: string; list?: boolean };
  /** Choices for a dropdown: strings or {value, label}; or the name of a shared list (options_from); see lib/options.ts. */
  options?: (string | { value: string; label?: string })[];
  options_from?: string;
  /** Also offer "Other..." so a value that is not listed can be typed. */
  allow_custom?: boolean;
  /** With the cidr-list widget: parent_field names the parent input that holds the address space; suggest lists prefix lengths to offer. */
  cidr?: { parent_field?: string; suggest?: number[] };
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
