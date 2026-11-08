import { describe, it, expect } from "vitest";
import type { JSONSchema } from "../../types/catalog";

// Extract the widget resolution logic from SchemaForm for testing
function resolveWidget(schema: JSONSchema): string {
  const ui = schema["x-tf-ui"];
  if (ui?.widget) return ui.widget;
  if (schema.enum) return "select";
  const type = Array.isArray(schema.type) ? schema.type[0] : schema.type;
  if (type === "boolean") return "checkbox";
  if (type === "integer" || type === "number") return "number";
  if (type === "array") return "list-of-strings";
  if (type === "object" && schema.additionalProperties) return "key-value-map";
  return "text";
}

describe("resolveWidget", () => {
  it("string → text", () => {
    expect(resolveWidget({ type: "string" })).toBe("text");
  });

  it("boolean → checkbox", () => {
    expect(resolveWidget({ type: "boolean" })).toBe("checkbox");
  });

  it("number → number", () => {
    expect(resolveWidget({ type: "number" })).toBe("number");
  });

  it("integer → number", () => {
    expect(resolveWidget({ type: "integer" })).toBe("number");
  });

  it("enum → select", () => {
    expect(resolveWidget({ type: "string", enum: ["a", "b"] })).toBe("select");
  });

  it("array → list-of-strings", () => {
    expect(resolveWidget({ type: "array", items: { type: "string" } })).toBe("list-of-strings");
  });

  it("object with additionalProperties → key-value-map", () => {
    expect(
      resolveWidget({
        type: "object",
        additionalProperties: { type: "string" },
      })
    ).toBe("key-value-map");
  });

  it("x-tf-ui.widget overrides auto-detection", () => {
    expect(
      resolveWidget({
        type: "string",
        "x-tf-ui": { widget: "pick-existing" },
      })
    ).toBe("pick-existing");
  });

  it("locked widget from metadata", () => {
    expect(
      resolveWidget({
        type: "boolean",
        "x-tf-ui": { widget: "locked", locked_reason: "Policy" },
      })
    ).toBe("locked");
  });

  it("nullable-object widget", () => {
    expect(
      resolveWidget({
        type: ["object", "null"],
        "x-tf-ui": { widget: "nullable-object" },
      })
    ).toBe("nullable-object");
  });
});

describe("schema structure validation", () => {
  it("all sample schemas have required structure", async () => {
    const fs = await import("fs");
    const path = await import("path");
    const catalogDir = path.resolve(__dirname, "../../../../catalog/modules");

    let entries: string[];
    try {
      entries = fs.readdirSync(catalogDir);
    } catch {
      // Skip if catalog dir not found (CI environment)
      return;
    }

    for (const modName of entries) {
      const modPath = path.join(catalogDir, modName);
      if (!fs.statSync(modPath).isDirectory()) continue;

      const versions = fs.readdirSync(modPath);
      for (const ver of versions) {
        const schemaPath = path.join(modPath, ver, "schema.json");
        if (!fs.existsSync(schemaPath)) continue;

        const raw = fs.readFileSync(schemaPath, "utf-8");
        const schema: JSONSchema = JSON.parse(raw);

        expect(schema.title, `${modName}/${ver} missing title`).toBeTruthy();
        expect(schema.properties, `${modName}/${ver} missing properties`).toBeTruthy();
        expect(schema["x-tf-module"], `${modName}/${ver} missing x-tf-module`).toBeTruthy();
        expect(schema["x-tf-ui"], `${modName}/${ver} missing x-tf-ui`).toBeTruthy();
        expect(
          schema["x-tf-ui"]!.groups?.length,
          `${modName}/${ver} has no groups`
        ).toBeGreaterThan(0);

        // Every property that's not hidden should have a title
        for (const [key, prop] of Object.entries(schema.properties!)) {
          if (prop["x-tf-ui"]?.hidden) continue;
          expect(prop.title, `${modName}/${ver} property ${key} missing title`).toBeTruthy();
        }

        // Widget resolution should not throw for any property
        for (const [, prop] of Object.entries(schema.properties!)) {
          expect(() => resolveWidget(prop)).not.toThrow();
        }
      }
    }
  });
});
