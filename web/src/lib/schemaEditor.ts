/** Pure helpers of the schema editor (D-060): naming, the export file and reading it back. */

/** The catalog name a schema most likely wants: its module source, else the fallback. */
export function suggestName(schemaText: string, fallback = ""): string {
  try {
    const src = (JSON.parse(schemaText) as { "x-tf-module"?: { source?: string } })["x-tf-module"]?.source;
    if (src) return src.split("/").pop()!.toLowerCase();
  } catch {
    /* not JSON yet */
  }
  return fallback;
}

/** The version a schema declares (x-tf-module.version), or "". */
export function versionOf(schemaText: string): string {
  try {
    const v = (JSON.parse(schemaText) as { "x-tf-module"?: { version?: unknown } })["x-tf-module"]?.version;
    return typeof v === "string" ? v : "";
  } catch {
    return "";
  }
}

export type Parsed = { ok: true; value: unknown } | { ok: false; error: string } | null;

/** The schema text as JSON: null while empty, an error while it is not a JSON object yet. */
export function parseSchema(text: string): Parsed {
  if (!text.trim()) return null;
  try {
    const value: unknown = JSON.parse(text);
    if (typeof value !== "object" || value === null || Array.isArray(value)) return { ok: false, error: "a schema is a JSON object" };
    return { ok: true, value };
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : "not valid JSON" };
  }
}

/** The file a schema is exported to, so it can be imported into another project or catalog. */
export function exportFile(name: string, schemaText: string): { filename: string; content: string } {
  const version = versionOf(schemaText);
  const content = JSON.stringify({ terraflow: "module-schema", name, schema: JSON.parse(schemaText) }, null, 2) + "\n";
  return { filename: `${name || "module"}${version ? `-${version}` : ""}.schema.json`, content };
}

/**
 * Reads a file given to the editor: an export of this editor ({ name, schema }) or a bare
 * schema as it lives in a module repository. A bare schema gets its name from its module
 * source, else from the file name.
 */
export function readImport(text: string, filename = ""): { name: string; schema: string } {
  const fromFile = filename.replace(/(\.schema)?\.json$/i, "").replace(/-v\d+(\.\d+){0,2}$/, "").toLowerCase();
  try {
    const obj = JSON.parse(text) as { name?: unknown; schema?: unknown; properties?: unknown };
    if (obj && typeof obj === "object" && typeof obj.schema === "object" && obj.schema !== null && obj.properties === undefined) {
      const schema = JSON.stringify(obj.schema, null, 2);
      return { name: typeof obj.name === "string" && obj.name ? obj.name : suggestName(schema, fromFile), schema };
    }
  } catch {
    /* not JSON: hand the text over as it is, the editor says what is wrong */
  }
  return { name: suggestName(text, fromFile), schema: text };
}

/** Starts a download of text made in the browser. */
export function download(filename: string, content: string): void {
  const url = URL.createObjectURL(new Blob([content], { type: "application/json" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
