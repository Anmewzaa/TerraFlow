import type { CanvasNode } from "../types/changeset";
import type { Issue } from "./validate";

/** A raw provider resource has no catalog schema (D-040): its arguments are free-form. */
export const RESOURCE_TYPE = /^[a-z][a-z0-9]*_[a-z0-9_]+$/;
export const BLOCK_NAME = /^[A-Za-z_][A-Za-z0-9_-]*$/;

export function isRawResource(n: Pick<CanvasNode, "kind">): boolean {
  return n.kind === "resource";
}

export function newRawResource(
  folder: string,
  index: number,
  template?: { resource_type: string; default_values?: Record<string, unknown> },
): CanvasNode {
  return {
    id: `raw-${Date.now().toString(36)}-${index}`,
    kind: "resource",
    resource_type: template?.resource_type ?? "",
    module_name: "",
    module_version: "",
    instance_name: "",
    target_folder: folder,
    values: structuredClone(template?.default_values ?? {}),
    // 0,0 = "no spot yet": the canvas puts it in free space (placeAutoRoots).
    position_x: 0,
    position_y: 0,
  };
}

/** What is still wrong on a raw resource node. */
export function rawIssues(n: CanvasNode): Issue[] {
  const issues: Issue[] = [];
  if (!RESOURCE_TYPE.test(n.resource_type ?? "")) {
    issues.push({ text: "Resource type is required, e.g. azurerm_resource_group", field: "resource_type" });
  }
  if (!BLOCK_NAME.test(n.instance_name)) {
    issues.push({ text: "Name is required (letters, digits, _ and -; must not start with a digit)", field: "instance_name" });
  }
  if (!n.target_folder) issues.push({ text: "Choose a folder", field: "target_folder" });
  return issues;
}

/**
 * Parse the arguments text. Arguments are a JSON object: plain values are
 * written as they are, {"$expr": "var.x"} is written unquoted, and
 * {"block_name": {"$blocks": [{...}]}} writes nested blocks.
 */
export function parseValues(text: string): { values: Record<string, unknown> } | { error: string } {
  const t = text.trim();
  if (t === "") return { values: {} };
  try {
    const v: unknown = JSON.parse(t);
    if (v === null || typeof v !== "object" || Array.isArray(v)) {
      return { error: "Arguments must be a JSON object, e.g. {\"name\": \"rg-demo\"}" };
    }
    return { values: v as Record<string, unknown> };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Not valid JSON" };
  }
}
