import { describe, expect, it } from "vitest";
import { exportFile, parseSchema, readImport, suggestName, versionOf } from "./schemaEditor";

const schema = JSON.stringify({ title: "W", type: "object", "x-tf-module": { source: "git::https://x.invalid/m.git//modules/Widget", version: "v2" }, properties: { name: { type: "string" } } });

describe("schema editor helpers", () => {
  it("names a schema after its module source and reads its version", () => {
    expect(suggestName(schema)).toBe("widget");
    expect(suggestName("{", "fallback")).toBe("fallback");
    expect(versionOf(schema)).toBe("v2");
    expect(versionOf("{}")).toBe("");
  });

  it("tells empty, broken and non-object text apart", () => {
    expect(parseSchema("  ")).toBeNull();
    expect(parseSchema("{")).toMatchObject({ ok: false });
    expect(parseSchema("[1]")).toEqual({ ok: false, error: "a schema is a JSON object" });
    expect(parseSchema(schema)).toMatchObject({ ok: true });
  });

  it("an export can be read back into another project unchanged", () => {
    const file = exportFile("team-widget", schema);
    expect(file.filename).toBe("team-widget-v2.schema.json");
    const back = readImport(file.content, file.filename);
    expect(back.name).toBe("team-widget");
    expect(JSON.parse(back.schema)).toEqual(JSON.parse(schema));
  });

  it("a bare schema file gets its name from the module source, else from the file name", () => {
    expect(readImport(schema, "anything.json").name).toBe("widget");
    expect(readImport(JSON.stringify({ type: "object", properties: {} }), "Storage-Account-v1.schema.json").name).toBe("storage-account");
    // A schema that happens to have an input called "schema" is not mistaken for an export.
    const tricky = JSON.stringify({ type: "object", properties: { schema: { type: "object" } }, schema: {} });
    expect(readImport(tricky, "tricky.json").schema).toBe(tricky);
    expect(readImport("not json", "x.json")).toEqual({ name: "x", schema: "not json" });
  });
});
