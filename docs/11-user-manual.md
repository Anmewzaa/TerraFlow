# 11 — User manual

For developers who build infrastructure changes and for DevOps (admins) who review
and approve them. TerraFlow stops at a Git branch: the pull request, pipeline, plan
and apply stay in Azure DevOps (ADO).

## Roles

| Role | Comes from | Can |
|---|---|---|
| Developer | Entra developer group | See the projects an admin assigned; build on the canvas; save versions; publish for review |
| Admin (DevOps) | Entra DevOps group | Everything a developer can, in every project; review, approve or reject; manage modules, raw resources, projects, members and users; see the audit log |

Which projects a developer sees is set inside the app (Home, Administration,
Projects, Members), not in Entra. If a project is missing, ask an admin to add you.

In a local build the sign-in page lists made-up users (Ada Admin, Alice and Bob
Developer). Pick one; "Switch user" returns to the list.

## Developer: from idea to a reviewed branch

1. **Home.** Open a project card ("Open canvas"). Admins can create a project; its
   folder holds the Terraform. An existing folder can be imported, and the module
   calls already in it appear as components.
2. **Canvas.** The left panel has two tabs. *Modules* is the approved catalog: drag
   one onto the canvas, or onto a container (a subnet into a virtual network).
   *Resources* lists what already exists in the repository; select one and choose
   "Edit this resource" to change it.
3. **Fill the form.** The form is built from the module's schema. Locked (hatched)
   fields are filled for you, for example the resource group of a chosen network.
   A value shown as a reference (for example `var.name`) is kept as written; use
   "Use a value" only if you mean to replace it.
4. **Arrows** between components mean "this refers to that". TerraFlow decides how to
   write it: a direct reference in the same folder, a `data` block across folders.
5. **Output file.** Each component says which `.tf` file it is written to. New code is
   added to the end of a file; existing files are never replaced.
6. **HCL Preview** shows what would change: new, changed and removed blocks. Turn off
   "Show only what changed" to read the whole repository.
7. **Save** makes a named version (Version 1, 2, …). Your draft also saves on its
   own. Drafts and saved versions are visible to you only.
8. **Changesets.** Choose a saved version and **Publish**. Only one changeset per
   project can wait for review; if another is waiting, Publish says which, with a
   link to it.
9. **Rejected?** Open the changeset, read the reason, revise it and publish again.
10. **Approved.** The changeset shows the branch name. Open it in ADO and continue
    with the pull request there.

### Raw resources

Some Azure resources have no module yet. An admin can prepare a *raw resource* (for
example `azurerm_resource_group`) with default arguments. It appears in the left
panel and is written as a `resource` block. TerraFlow never converts a module call
into a raw resource, and checks only that the type and name look valid, not the
arguments: mistakes show up in the pipeline plan.

### Text you type is text

A value is written as a quoted string exactly as typed. Typing `${var.x}` does not
create a reference; it is written as the literal text. To refer to something, pick
it with the field's reference option or an arrow.

## Admin: reviewing

1. **Review** (left rail) lists changesets waiting for review. Open one to see the
   canvas changes and the exact Terraform diff.
2. **Approve** creates the branch with one commit. **Reject** needs a reason, which the
   developer sees. If separation of duties is on, you cannot review your own change.
3. The page is the same code that Approve writes. If generation fails (a duplicate
   name, a file that no longer matches), Approve refuses and says why; nothing is
   written.
4. The **Audit log** (Home, Administration) records saves, publishes, approvals,
   rejections, project, member and module changes.

## Admin: administration (Home, Administration)

| Task | Where | Notes |
|---|---|---|
| Add a module version | Modules: paste the schema, *Check*, then *Import* | Validation lists every problem at once. A published version cannot be changed; import a new version. Breaking changes against the previous version are listed |
| Retire a module version | Modules: *Deprecate* | It leaves the developer catalog but drafts that use it keep working |
| Raw resource templates | Raw resources | Name, title, resource type, default arguments, default folder |
| Projects | Projects | Create, rename, change folder, delete. Delete removes the project from TerraFlow only; repository files and the audit log stay. Deleting with a changeset waiting needs confirmation |
| Members | Projects, Members | Users appear after they sign in once |
| Users | Users | Read-only list |

## Messages you may meet

| Message | Meaning | What to do |
|---|---|---|
| "Could not load projects/the project/the module catalog: …" | The server answered with an error | Reload; if it stays, tell an admin with the text shown |
| "No projects yet" | You are not assigned to any project | Ask an admin |
| "changeset … is still waiting for review; approve or reject it before publishing another" (409) | One per project | Open the named changeset; wait for the decision |
| "generation failed: …" (422) | The change cannot be written safely | Read the reason: duplicate module name, an edited block that no longer exists, a file that does not parse |
| "invalid environment name" (400) | Environment names are letters, digits, `-` and `_`, up to 32 | Use `dev`, `prod`, `uat-2` |
| "cross-site request refused" (403) | The browser sent the request from another site | Use TerraFlow from its own address |
| "too many requests" (429) | Too many sign-in attempts | Wait half a minute |
| "this action needs the admin role" (403) | Admin-only action | Ask an admin |
| A project or changeset "not found" | It does not exist, you are not a member, or it is someone else's draft | Check the link; ask an admin |
| "Something went wrong" page | A screen failed to draw. Your draft is saved on the server | Reload |

## Limits to know about

- Editing existing code is limited to module calls and plain resource blocks.
  Comments written *inside* an edited block are not kept (comments around it are).
  Blocks using `dynamic` or labelled nested blocks are left untouched.
- Values TerraFlow cannot work out from `var`, `locals` or `.tfvars` show as
  `unknown`, never as blank or zero.
- TerraFlow reads the Terraform files, not Azure. A resource that exists in Azure but
  not in the repository does not appear.
- Sign-in lasts four hours. Role changes in Entra apply after the next sign-in.

## For whoever runs it

- Environment settings: `TF_ALLOWED_ORIGINS` (only if the UI is on another origin),
  `TF_SEPARATION_OF_DUTIES=true`, `TF_INSECURE_COOKIES=true` for plain-HTTP local runs only.
- Behind an ingress that rewrites `Host`, forward the original in `X-Forwarded-Host`.
- Browser tests: `web/e2e/README.md`. Security review: `docs/10-security.md`.
