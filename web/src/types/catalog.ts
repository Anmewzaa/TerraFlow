export interface ModuleSummary {
  name: string;
  title: string;
  description: string;
  icon: string;
  versions: string[];
  latest_version: string;
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
