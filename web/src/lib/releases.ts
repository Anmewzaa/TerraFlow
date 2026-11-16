/* What each version of TerraFlow added, improved, fixed or removed. This list is the one place the version
 * shown on the Home page comes from: the first entry is the current version. Add a release at the top
 * when something ships; never rewrite an older one. Dates are the day the work was finished. */

export type ChangeKind = "Added" | "Improved" | "Fixed" | "Removed";

export interface Release {
  version: string;
  date: string;
  title: string;
  changes: { kind: ChangeKind; text: string }[];
}

export const RELEASES: Release[] = [
  {
    version: "v1.0.1",
    date: "2026-11-16",
    title: "Screens in the README and the landing tour match the current interface",
    changes: [
      { kind: "Fixed", text: "The caption under the screenshots on the landing page now keeps its space from the image." },
      { kind: "Improved", text: "The README opens with the landing page and screens of the current interface, and the landing tour shows the same screens." },
    ],
  },
  {
    version: "v1.0.0",
    date: "2026-11-16",
    title: "The first full release: a new front door, a working compare and a layout for every screen size",
    changes: [
      { kind: "Added", text: "A landing page before sign-in with the product, screens from the running app (they follow light and dark) and the release notes; signing in goes straight on to your projects." },
      { kind: "Added", text: "Compare environments can compare the canvas drafts of two environments, so work that is not published yet can be compared too. The repository comparison stays, and says so when the repository has nothing to compare." },
      { kind: "Added", text: "Success and error messages and confirmations are SweetAlert2 notices that follow the theme." },
      { kind: "Added", text: "A preview of each project's resources on its card in the project list." },
      { kind: "Improved", text: "Layout for phones and tablets: the rail becomes a bar at the bottom, the top bar is one row, and the palette, changesets and compare stack instead of squeezing." },
      { kind: "Improved", text: "Projects, admin and audit use a side menu and the same background as the landing page; the profile and sign-out sit at the bottom of the rail; the theme switch is always at the top right." },
      { kind: "Improved", text: "Resources that can hold others (resource group, virtual network, subnet) look different from single resources; the palette is one clean row (provider, version, modules or raw resources) with no category chips; the canvas no longer zooms in on entry; the audit log has date, action and text filters and ten rows a page; the documentation pages are redrawn." },
      { kind: "Added", text: "The template gallery in the Stacks tab: browse shared templates and import one into the project. A public example, a virtual network with two subnets, ships with it." },
      { kind: "Added", text: "Resource groups, virtual networks and subnets can be dragged larger, and the size is kept with the canvas." },
      { kind: "Added", text: "Installs on Kubernetes with Helm in two versions: the local one (the app and its own Postgres, made-up users) and the GitHub one (the same, with sign-in with GitHub and projects on your repositories). The server now serves the screens itself." },
      { kind: "Added", text: "Resources inside a container can be dropped and moved anywhere in it, and the container grows to hold them." },
      { kind: "Improved", text: "The changeset page shows the generated files with the same viewer as the HCL preview, and a gallery template is used in one step (the Use button, or drag it onto the canvas)." },
      { kind: "Improved", text: "Changesets and review explain themselves: what this version is, what happens next, what each resource adds and which files will be written." },
      { kind: "Fixed", text: "Signing in from the landing page now goes straight to your projects, and the project preview shows what is on the canvas when nothing is committed yet." },
      { kind: "Fixed", text: "The list of problems no longer covers the canvas zoom controls, and the labels of the navigation rail are no longer hidden behind the page." },
    ],
  },
  {
    version: "v0.9.0",
    date: "2026-11-16",
    title: "Address checks, a template gallery and a calmer interface",
    changes: [
      { kind: "Added", text: "Subnet address checks: a subnet must lie inside its virtual network's address space and may not overlap another subnet. A CIDR calculator in the form shows the addresses in each block and the next free block." },
      { kind: "Added", text: "Shared template gallery (backend): private and public stack templates, admin approval of public ones, export and import with a scan for secrets and subscription ids." },
      { kind: "Added", text: "Choose the branch when creating a project from a GitHub repository." },
      { kind: "Added", text: "Dropdowns defined in a module's schema (options, a shared list of Azure regions, and an Other choice), and resource icons from a manifest instead of code." },
      { kind: "Added", text: "A drifting-glyph background behind Changesets, HCL preview, Compare and Settings, in light and dark, which can be switched off." },
      { kind: "Improved", text: "Changesets and Review are one page: the list of changes on the left, the diff and the approve or reject buttons on the right, with a progress line from saved to decided." },
      { kind: "Improved", text: "Compare environments redesigned with summary tiles, search, a swap button and a side-by-side table that highlights what differs." },
      { kind: "Improved", text: "Loading states: the Terraform preview shows its stages, other panels show placeholders; focus rings, scrollbars and the navigation rail follow the theme." },
      { kind: "Fixed", text: "The subnet name is no longer locked to snet-…; a project can require it under Settings > Checks." },
      { kind: "Fixed", text: "List and key-value inputs were narrower than other fields, and the form was indented twice inside the side panel." },
      { kind: "Removed", text: "The separate Review page: its address now opens Changesets." },
    ],
  },
  {
    version: "v0.8.0",
    date: "2026-11-14",
    title: "Environments, links and checks",
    changes: [
      { kind: "Added", text: "Validation moved to the server: save, publish and approve refuse what the canvas cannot safely write, and the screens show the same findings." },
      { kind: "Added", text: "Environments and a flexible folder layout, compare two environments, and copy a resource from one to another." },
      { kind: "Added", text: "Links between resources (a private endpoint to its target and DNS zone), seed network and data modules, project policy checks and module-level checks." },
      { kind: "Added", text: "Repository drift (what changed in the repository since TerraFlow last wrote it) and stack templates: make one from a selection, fill its slots, apply it to environments." },
      { kind: "Improved", text: "The branch a project works on can be chosen and is checked, and a missing or moved base branch is reported." },
      { kind: "Fixed", text: "Subnets placed inside a virtual network were reported as missing; a project variable used as a map key was written as plain text." },
    ],
  },
  {
    version: "v0.7.0",
    date: "2026-11-10",
    title: "GitHub, sign-in and your own catalog",
    changes: [
      { kind: "Added", text: "Projects live on your own GitHub repositories through a GitHub App, with owner, co-owner and contributor roles." },
      { kind: "Added", text: "Sign in with GitHub or Microsoft Entra, sign-up approval, and an admin who manages people and connections." },
      { kind: "Added", text: "Module catalog in two scopes (global and per project) and a schema editor that drafts a form from a pasted variables.tf." },
      { kind: "Improved", text: "Cloud providers are data an admin edits, not code." },
    ],
  },
  {
    version: "v0.6.0",
    date: "2026-11-09",
    title: "More of the workflow, and security",
    changes: [
      { kind: "Added", text: "Raw resources (a provider resource added by hand), importing existing blocks from the repository, and editing them." },
      { kind: "Added", text: "Security hardening: secrets are never typed into the canvas, request limits, and an audit trail of who published, approved or changed what." },
      { kind: "Improved", text: "Interface version 2: a left rail, a detail panel, and the module palette." },
    ],
  },
  {
    version: "v0.5.0",
    date: "2026-11-08",
    title: "Review, branch and audit",
    changes: [
      { kind: "Added", text: "Save a canvas as a changeset, send it for review, and approve it to create a branch with the generated Terraform. TerraFlow stops at the branch." },
      { kind: "Added", text: "Schema-driven forms for every module, and the module-call generator with golden-file tests." },
    ],
  },
  {
    version: "v0.4.0",
    date: "2026-11-08",
    title: "Edit on the canvas and generate Terraform",
    changes: [
      { kind: "Added", text: "Canvas editing: drag modules from the catalog onto the canvas, edit them in a side panel, and keep the work as a draft changeset." },
      { kind: "Added", text: "The HCL generator: the draft becomes module calls in the team's style, merged into existing files instead of replacing them, with golden-file and round-trip tests." },
      { kind: "Added", text: "The Harbor design system and a new interface: a left rail, a top bar, and light and dark themes." },
      { kind: "Fixed", text: "A resource group and a virtual network drawn from a draft appear as containers, and picking an existing resource includes drafts." },
    ],
  },
  {
    version: "v0.3.0",
    date: "2026-11-08",
    title: "A catalog of modules and forms drawn from schemas",
    changes: [
      { kind: "Added", text: "The module catalog: each module has a JSON Schema, a version, and a place in a category." },
      { kind: "Added", text: "Schema-driven forms: the inputs of a module are drawn from its schema, so adding a module needs no screen code." },
      { kind: "Added", text: "Drafts that save as you work." },
    ],
  },
  {
    version: "v0.2.0",
    date: "2026-11-07",
    title: "Sign-in, projects and a database",
    changes: [
      { kind: "Added", text: "Sign-in with Microsoft Entra through the server (the browser never holds a token), and roles for administrators and developers." },
      { kind: "Added", text: "Projects, kept in Postgres with migrations, and the first project screens." },
      { kind: "Added", text: "The HTTP server (chi) with request ids and JSON errors." },
    ],
  },
  {
    version: "v0.1.0",
    date: "2026-11-07",
    title: "See the infrastructure you already have",
    changes: [
      { kind: "Added", text: "A parser that reads the Terraform in a repository and builds a graph of its resources, without any Azure access." },
      { kind: "Added", text: "A read-only visualiser: resources drawn on a canvas with automatic layout, nested the way they sit in Azure (resources inside a resource group)." },
      { kind: "Fixed", text: "Data sources are hidden from the canvas, and the automatic layout places nodes correctly." },
    ],
  },
];

/** The version shown on the Home page: the newest release. */
export const APP_VERSION = RELEASES[0]!.version;
