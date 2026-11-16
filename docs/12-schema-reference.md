# Schema reference: modules and raw resources

For admins who add to the catalog. The same text is in the admin area under **Docs** (Modules and Raw
resources). The rules below are enforced by the import check (`server/internal/catalog/validate.go`), so a
schema that passes the check can be drawn as a form and generated as HCL.

## Module or raw resource?

| | Module | Raw resource |
|---|---|---|
| What it is | A Terraform module the team agreed to use | A plain provider block, such as `azurerm_resource_group` |
| Form | Drawn from its schema, values checked as people type | Free-form JSON; checked only when the pipeline runs |
| Generated as | `module "<name>" { source = "…" … }` | `resource "<type>" "<name>" { … }` |
| Added by | Import a JSON schema (Admin, Modules) | A form (Admin, Raw resources) |
| Use it for | Everything the team builds more than once | One-off needs with no module yet |

Hard rule 5 of the project: module calls are the default; a raw resource is generated only when a person
adds it explicitly, and TerraFlow never turns a module call into one.

## A module schema

A file `schema.json` (catalog seed: `catalog/modules/<name>/v1/schema.json`). A complete one that passes the
check, which the admin screen offers as **Start from an example**:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "My Resource Group",
  "description": "An Azure resource group with an optional management lock.",
  "type": "object",
  "x-tf-module": { "source": "my-resource-group", "version": "v1",
                   "target_folder": "common", "filename": "rg.tf", "icon": "ResourceGroup" },
  "x-tf-canvas": { "container": true, "domain": "neutral" },
  "x-tf-ui": { "groups": [ { "id": "basics", "label": "Basics", "order": 1 } ] },
  "required": ["name"],
  "properties": {
    "name":     { "type": "string", "title": "Name", "pattern": "^rg-[a-z0-9-]{3,60}$",
                  "x-tf-ui": { "group": "basics", "order": 1 } },
    "location": { "type": "string", "title": "Location", "default": "southeastasia",
                  "x-tf-ui": { "group": "basics", "order": 2 } },
    "lock_level": { "type": "string", "default": "CanNotDelete", "enum": ["", "CanNotDelete", "ReadOnly"],
                    "x-tf-ui": { "group": "basics", "order": 3 } },
    "tags": { "type": "object", "default": {}, "additionalProperties": { "type": "string" },
              "x-tf-ui": { "widget": "key-value-map", "order": 4 } }
  }
}
```

For a resource named `rg_main` the generator writes a call like this in `common/rg.tf` (illustrative: the values are
whatever the person typed in the form):

```hcl
module "rg_main" {
  source = "my-resource-group"

  location   = "southeastasia"
  lock_level = "CanNotDelete"
  name       = "rg-aic-dev-app"
  tags = {
    owner = "platform"
  }
}
```

Attributes are written sorted by name, with `tags` last.

### Top level

| Key | Required | Meaning |
|---|---|---|
| `title`, `description` | yes | Name and one line shown in the palette and on the form |
| `type` | yes | Always `object`; its `properties` are the module's inputs |
| `properties` | yes | One entry per input (per Terraform variable) |
| `required` | no | Inputs a person must fill in; shown as incomplete until they are |

### `x-tf-module`

| Key | Meaning |
|---|---|
| `source` | The module path written as `source = …` |
| `version` | `v1`, `v1.2` or `v1.2.3`. A published version never changes; publish a new one |
| `target_folder` | Default folder of the module call (`common`, `network`…); a person can change it per resource |
| `filename` | Default file, ending in `.tf` (`rg.tf`); empty means `main.tf` |
| `icon` | Icon name for the palette and the canvas |
| `provider` | Optional: the cloud provider the module is written for (`azurerm`). It only groups and filters the catalog; a module needs no provider version |

### `x-tf-canvas`

| Key | Meaning |
|---|---|
| `container` | `true` when other resources can be placed inside it (a virtual network holds subnets) |
| `parents` | Catalog names it may be placed under |
| `parent_required` | `true` when it makes no sense without a parent |
| `nest.field` | Folds it into the parent's input of that name (a subnet becomes an entry of `subnets`) instead of a module call |
| `domain` | Colour group: `network`, `data`, `compute`, `security`, `neutral`… |
| `category` | Optional palette group (`compute`, `network`, `storage`…, see `GET /catalog/meta`). Without it the domain decides |

### Each property

| Key | Meaning |
|---|---|
| `type`, `title`, `description`, `default` | Kind of value, label, help text, starting value |
| `enum` | The only values allowed; drawn as a drop-down |
| `pattern`, `minLength`, `maxLength`, `minimum`, `maximum` | Checked while typing; the pattern is how naming conventions are enforced |
| `items`, `additionalProperties` | Content of a list, or the value type of a map |
| `x-tf-ui.group`, `x-tf-ui.order` | Form section and order (sections are declared in the top-level `x-tf-ui.groups`) |
| `x-tf-ui.widget` | How it is drawn (below) |
| `x-tf-ui.options`, `x-tf-ui.options_from`, `x-tf-ui.allow_custom` | A dropdown defined in the schema: a list of `"value"` or `{ "value", "label" }`, or the name of a shared list (`azure-regions`); `allow_custom` adds "Other..." so an unlisted value can be typed. An input named `location` gets the Azure regions unless the schema says otherwise (`"widget": "text"` turns it off). A plain `enum` is still a dropdown |
| `x-tf-checks` (network) | `cidr` (every literal of `field` is a CIDR block starting at its first address), `cidr_within_parent` (`field` lies inside `parent_field` of the resource it is placed in; a subnet in its network's `address_space`), `cidr_no_overlap` (blocks do not overlap each other or the same input of sibling resources of the same module). A reference or a project variable is skipped, never reported. Errors |
| `x-tf-ui.widget: cidr-list`, `x-tf-ui.cidr` | A list of CIDR blocks with a calculator: what each block holds, the problems above as you type, and, when `cidr.parent_field` names the parent's address space, the free blocks to add (`cidr.suggest` lists the prefix lengths to offer) |
| `x-tf-module.icon` | The resource icon: a name in `web/src/design-system/azure-icons.json` (kebab case, for example `RouteTable`), a `/azure-icons/<file>.svg` path, or a small SVG data URI. Without one the canvas draws a neutral glyph |
| `x-tf-ui.pick` | With `pick-existing`: `{ "module_type": "resource-group", "output": "name" }` |

### Widgets

`key-value-map` (tags), `list-of-strings`, `list-of-objects`, `nested-map-of-objects`, `nullable-object` (an optional
block, off until turned on), `pick-existing` (choose another resource instead of typing its name; writes a
reference, for example `data.…` across folders), `from-parent`, `locked`. An unknown widget is only a warning: the
form falls back to the plain input for the type.

### Not supported

`$ref`, `oneOf`, `anyOf`, `allOf`, `not`, `if`, `patternProperties`, `dependentSchemas`. The form cannot draw them, so
the import check rejects them. Maximum nesting depth is 8.

### Publishing

1. Admin > Modules > **Add a module** > start from an example, upload, or paste.
2. **Check** lists every problem with its JSON pointer (`/properties/name/pattern`), warnings as tips, and, for a
   known module, the changes since the previous version with breaking ones marked.
3. **Import and publish.** The same name and version with different content is refused; change `x-tf-module.version`.
4. **Deprecate** hides a version from the palette without deleting it; resources already on a canvas keep working.

## A raw resource

| Field | Required | Meaning |
|---|---|---|
| `name` | yes | Lower-case identifier; cannot change later |
| `title` | yes | Name in the palette |
| `resource_type` | yes | The exact provider type, e.g. `azurerm_resource_group` |
| `description` | no | When a developer should use it |
| `target_folder` | no | Default folder; empty means the first folder of the project |
| `provider` | no | Taken from the resource type (`azurerm_` means `azurerm`) and must agree with it |
| `category` | no | Palette group |
| `min_version`, `max_version` | no | Provider versions (major.minor, e.g. `3.0`) the resource exists in; a project whose provider version is outside the range does not see it |
| `default_values` | no | JSON the block starts with. `{"$expr": "var.location"}` writes a reference; `{"ip_configuration": {"$blocks": [{…}]}}` writes nested blocks |

## What generation writes today (and does not)

Only `.tf` files, and only module calls, the `data` blocks that cross-folder references need, and explicit raw
`resource` blocks. `variables.tf`, `outputs.tf`, `providers.tf`, `backend.tf` and `.tfvars` are not written; values are
literals in the module call. The parser does read `variables`, `locals` and `env/<env>.tfvars` to resolve names.
Writing those files, project variables and layout settings are designed in `docs/11-project-settings.md`.
