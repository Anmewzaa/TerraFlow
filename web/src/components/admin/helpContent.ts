/* What the Docs drawer in the admin area says. Keep it in step with docs/12-schema-reference.md
 * and server/internal/catalog/validate.go (the rules the import check enforces). */

export type HelpTopic = "modules" | "resources" | "providers" | "generation" | "projects" | "people";

/** A small module schema that passes the import check; "Start from an example" fills it in. */
export const EXAMPLE_NAME = "my-resource-group";

export const EXAMPLE_SCHEMA = {
  $schema: "https://json-schema.org/draft/2020-12/schema",
  title: "My Resource Group",
  description: "An Azure resource group with an optional management lock.",
  type: "object",
  "x-tf-module": {
    source: "my-resource-group",
    version: "v1",
    target_folder: "common",
    filename: "rg.tf",
    icon: "ResourceGroup",
  },
  "x-tf-canvas": { container: true, domain: "neutral" },
  "x-tf-ui": { groups: [{ id: "basics", label: "Basics", order: 1 }] },
  required: ["name"],
  properties: {
    name: {
      type: "string",
      title: "Name",
      description: "Convention: rg-<project>-<env>-<purpose>.",
      pattern: "^rg-[a-z0-9-]{3,60}$",
      "x-tf-ui": { group: "basics", order: 1 },
    },
    location: {
      type: "string",
      title: "Location",
      default: "southeastasia",
      "x-tf-ui": { group: "basics", order: 2 },
    },
    lock_level: {
      type: "string",
      title: "Management lock",
      default: "CanNotDelete",
      enum: ["", "CanNotDelete", "ReadOnly"],
      "x-tf-ui": { group: "basics", order: 3 },
    },
    tags: {
      type: "object",
      title: "Tags",
      default: {},
      additionalProperties: { type: "string" },
      "x-tf-ui": { widget: "key-value-map", order: 4 },
    },
  },
};

export interface FieldDoc {
  key: string;
  where: string;
  meaning: string;
  required?: boolean;
}

/** Every key a module schema can use, where it goes and what it does. */
export const SCHEMA_FIELDS: FieldDoc[] = [
  { key: "title, description", where: "top level", meaning: "Name and one-line description shown in the catalog and on the form.", required: true },
  { key: "type: object", where: "top level", meaning: "Always an object: its properties are the module's inputs.", required: true },
  { key: "properties", where: "top level", meaning: "Every input of the module. One entry per Terraform variable.", required: true },
  { key: "required", where: "top level", meaning: "Names of the inputs a person must fill in; shown as incomplete until they are." },
  { key: "x-tf-module.source", where: "x-tf-module", meaning: "The module path written as `source = …` in the generated call.", required: true },
  { key: "x-tf-module.version", where: "x-tf-module", meaning: "v1, v1.2 or v1.2.3. A published version never changes; publish a new one.", required: true },
  { key: "x-tf-module.target_folder", where: "x-tf-module", meaning: "Folder the module call goes to by default (common, network…). A person can change it per resource." },
  { key: "x-tf-module.filename", where: "x-tf-module", meaning: "File the call is written to by default, ending in .tf (rg.tf). Empty means main.tf." },
  { key: "x-tf-module.icon", where: "x-tf-module", meaning: "Icon name for the canvas and the palette." },
  { key: "x-tf-module.provider", where: "x-tf-module", meaning: "Optional: the cloud provider the module is written for (azurerm). Only groups and filters the catalog; a module needs no provider version." },
  { key: "x-tf-canvas.category", where: "x-tf-canvas", meaning: "Optional palette group (compute, network, storage…). Without it the colour domain decides." },
  { key: "x-tf-canvas.container", where: "x-tf-canvas", meaning: "true when other resources can be placed inside it (a virtual network holds subnets)." },
  { key: "x-tf-canvas.parents", where: "x-tf-canvas", meaning: "Catalog names this resource may be placed under." },
  { key: "x-tf-canvas.parent_required", where: "x-tf-canvas", meaning: "true when it makes no sense without its parent." },
  { key: "x-tf-canvas.nest.field", where: "x-tf-canvas", meaning: "Folds the resource into the parent's input of that name (a subnet becomes an entry of `subnets`) instead of a module call." },
  { key: "x-tf-canvas.domain", where: "x-tf-canvas", meaning: "Colour group on the canvas: network, data, compute, security, neutral…" },
  { key: "x-tf-ui.groups", where: "top level x-tf-ui", meaning: "Sections of the form: id, label, order." },
  { key: "type, title, description, default", where: "each property", meaning: "Kind of value, the label, the help text and the starting value.", required: true },
  { key: "enum", where: "each property", meaning: "The only values allowed; shown as a drop-down." },
  { key: "pattern, minLength, maxLength, minimum, maximum", where: "each property", meaning: "Checks applied while a person types; the pattern is how naming conventions are enforced." },
  { key: "items, additionalProperties", where: "each property", meaning: "What is inside a list, or the value type of a map (tags)." },
  { key: "x-tf-ui.group, order", where: "each property", meaning: "Which form section and in what order." },
  { key: "x-tf-ui.widget", where: "each property", meaning: "How the field is drawn (see the list below). Empty means the plain input for the type." },
  { key: "x-tf-ui.pick", where: "each property", meaning: "With widget pick-existing: { module_type, output } says which resources can be picked and which output is used." },
];

export const WIDGETS: { id: string; use: string }[] = [
  { id: "key-value-map", use: "tags and other string maps." },
  { id: "list-of-strings", use: "a list of text values, such as address prefixes." },
  { id: "list-of-objects", use: "a list whose entries are objects (rules, disks)." },
  { id: "nested-map-of-objects", use: "a map whose values are objects." },
  { id: "nullable-object", use: "an optional block that is off until a person turns it on." },
  { id: "pick-existing", use: "choose another resource (a resource group) instead of typing its name; writes a reference." },
  { id: "from-parent", use: "takes the value from the parent on the canvas." },
  { id: "locked", use: "shows a value that cannot be changed on the canvas." },
];

/** Keywords the form cannot draw; the import check rejects them. */
export const UNSUPPORTED_KEYWORDS = ["$ref", "oneOf", "anyOf", "allOf", "not", "if", "patternProperties", "dependentSchemas"];

export const RAW_RESOURCE_FIELDS: FieldDoc[] = [
  { key: "name", where: "identifier", meaning: "Lower-case id used by TerraFlow; cannot change after it is created.", required: true },
  { key: "title", where: "shown", meaning: "Name in the palette.", required: true },
  { key: "resource_type", where: "Terraform", meaning: "The provider resource, for example azurerm_resource_group. Written as `resource \"<type>\" \"<name>\" {}`.", required: true },
  { key: "description", where: "shown", meaning: "One line that tells a developer when to use it." },
  { key: "provider", where: "provider", meaning: "The cloud provider, taken from the resource type (azurerm_ means azurerm). A project that chose providers only sees matching resources." },
  { key: "category", where: "palette", meaning: "Group in the palette: compute, network, storage…" },
  { key: "min_version, max_version", where: "provider", meaning: "The provider versions (major.minor, for example 3.0) it exists in. A project whose version is outside the range does not see it." },
  { key: "target_folder", where: "layout", meaning: "Folder it goes to by default; empty means the first folder of the project." },
  { key: "default_values", where: "arguments", meaning: "JSON of the arguments the block starts with. {\"$expr\": \"var.location\"} writes a reference; {\"$blocks\": [...]} writes nested blocks." },
];
