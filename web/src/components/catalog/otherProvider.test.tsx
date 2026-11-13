import fs from "node:fs";
import path from "node:path";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { JSONSchema } from "../../types/catalog";
import { SchemaForm } from "./SchemaForm";

// Exit test of M3 (D-060): the form is drawn from the schema alone, so a module for a
// provider other than azurerm renders with no code change. The fixture is the one the
// server validates (server/internal/catalog/testdata).
describe("a schema for another Terraform provider", () => {
  const file = path.resolve(__dirname, "../../../../server/internal/catalog/testdata/aws-s3-bucket.schema.json");
  const schema = JSON.parse(fs.readFileSync(file, "utf8")) as JSONSchema;

  it("renders every input of the schema", () => {
    const html = renderToStaticMarkup(createElement(SchemaForm, { schema, values: { storage_class: "STANDARD" }, onChange: () => {} }));
    for (const label of ["Basics", "Bucket name", "Versioning", "Storage class", "GLACIER", "Tags"]) {
      expect(html).toContain(label);
    }
  });
});
