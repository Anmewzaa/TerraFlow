# 11 — User manual

For developers who build infrastructure changes and for DevOps (admins) who review
and approve them. TerraFlow stops at a Git branch: the pull request, pipeline, plan
and apply stay with your team in GitHub (or the git provider you use).

## Signing in and getting approved

Sign in with GitHub or Microsoft Entra ID, whichever buttons the page shows. The first
time, your account is **pending**: you see a "Waiting for approval" page until an admin
approves you (press *Check again* afterwards; you do not need to sign in again). A
rejected or suspended account sees a message on the sign-in page. A GitHub account and an
Entra account are two different people to TerraFlow, even with the same email.

## Roles

There are two kinds of role.

**Project roles** (set per project; the person who creates a project is its owner):

| Role | Can |
|---|---|
| Owner | Everything in the project: build, approve, invite, set roles, disconnect the repository, transfer or delete the project |
| Co-owner | Build, approve, invite, set roles below owner; cannot delete or transfer |
| Contributor | Build on the canvas, save versions, publish for review; cannot approve |

The owner may approve their own change, unless the project requires a second reviewer
(Team screen). An invited person must already be approved; inviting someone lets them
open branches on the project's repository through TerraFlow.

**Admin** (the operator: `TF_ADMIN_USERS`, or the Entra DevOps group): approves people,
sees every project's details, manages connections, the global catalog, raw resources and
users, and reads the audit log. Opening a project the admin is not a member of is allowed
but recorded each time (`support.access`).

In a local build the sign-in page lists made-up users (Ada Admin, Alice and Bob
Developer). Pick one; "Switch user" returns to the list.

## Developer: from idea to a reviewed branch

1. **Home.** Open a project card ("Open canvas"). Anyone approved can create a project
   with **New from GitHub**: connect GitHub, pick a repository you can access, pick the
   folder that holds the Terraform, name it. You become its owner. The module calls
   already in the folder appear as components. (**New local project** uses a folder of the
   machine TerraFlow runs on; it is for development.)
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
10. **Approved.** The changeset shows the branch name. Find it in the repository on GitHub and open
    the pull request there. Protect the default branch of the repository: TerraFlow only
    ever creates a new branch.

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

## Owner and co-owner: team and reviewing

The **Team** screen lists the project's people. Invite someone who has signed in (by id or
email), change a role, remove a person, switch on *second reviewer*, and (owner only)
transfer the project.

1. **Review** (left rail) lists changesets waiting for review. Owners and co-owners review
   their project's; admins review any. Open one to see the
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
| Members | Projects, Members | Only approved users can be assigned; the owner is never removed here |
| GitHub connections | Connections | Every connected account with its project count; *Check* asks GitHub whether the installation still exists; *Disconnect* stops its projects from reading and branching until the App is installed again |
| Users | Users | Waiting people come first. *Approve*; *Reject* (a reason is required); *Suspend* an active user (a reason is required; their sessions end at once); *Restore*. You cannot change your own account. Every change is in the audit log |

## Messages you may meet

| Message | Meaning | What to do |
|---|---|---|
| "Could not load projects/the project/the module catalog: …" | The server answered with an error | Reload; if it stays, tell an admin with the text shown |
| "No projects yet" | You are not a member of any project | Create one from GitHub, or ask a project owner to invite you |
| "Waiting for approval" | An admin has not approved your account yet | Ask an admin; press *Check again* afterwards |
| "your account is pending" (403) | Same, seen by a script or an old tab | Same |
| "Sign-up is by invitation only" / "Sign-up is full for now" | The server does not take new people right now | Ask an admin |
| "changeset … is still waiting for review; approve or reject it before publishing another" (409) | One per project | Open the named changeset; wait for the decision |
| "generation failed: …" (422) | The change cannot be written safely | Read the reason: duplicate module name, an edited block that no longer exists, a file that does not parse |
| "invalid environment name" (400) | Environment names are letters, digits, `-` and `_`, up to 32 | Use `dev`, `prod`, `uat-2` |
| "cross-site request refused" (403) | The browser sent the request from another site | Use TerraFlow from its own address |
| "too many requests" (429) | Too many sign-in attempts from your address | Wait half a minute |
| "The TerraFlow GitHub App is not installed on any account you can reach" | Connecting a repository needs the App installed on the account that owns it | Install the App there (the dialog has a link), then connect again |
| "Request failed (404) /api/…" | The server gave no message; the path says which call failed | Tell an admin with the text shown |
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
- Sign-in lasts four hours. Role changes in Entra apply after the next sign-in; an admin's
  suspend or reject applies at once.
- Not built yet: automatic approval rules, and stopping a suspended owner's project from
  publishing and approving (an admin must transfer it).

## For whoever runs it

- Environment settings: `TF_ALLOWED_ORIGINS` (only if the UI is on another origin),
  `TF_SEPARATION_OF_DUTIES=true`, `TF_INSECURE_COOKIES=true` for plain-HTTP local runs only.
- Sign-in: `TF_LOGIN_PROVIDERS` (`entra`, `github`), `TF_SIGNUP_MODE` (`approval` default,
  `open`, `invite_only`), `TF_ADMIN_USERS` (ids such as `github:12345`, or emails; active at
  once), `TF_SIGNUP_MAX_PENDING`. Without an admin listed, nobody can approve the first user.
- GitHub: `TF_GITHUB_APP_ID`, a private key (`TF_GITHUB_APP_PRIVATE_KEY_FILE` or `_B64`),
  `TF_GITHUB_APP_CLIENT_ID` and `_CLIENT_SECRET`, `TF_GITHUB_REDIRECT_URI` (connect) and
  `TF_GITHUB_LOGIN_REDIRECT_URI` (sign-in); both URLs must be in the App's callback URLs.
  `TF_GITHUB_ALLOWED_OWNERS` limits which accounts may be connected.
- Generated module `source`: set in each catalog schema (`x-tf-module.source`), or placed
  under `TF_MODULE_SOURCE_BASE` for a bare name.
- `terraflow migrate` runs migrations and the catalog seed on their own; set
  `TF_MIGRATE_ON_START=false` to skip them at start.
- Behind an ingress that rewrites `Host`, forward the original in `X-Forwarded-Host`.
- Browser tests: `web/e2e/README.md`. Security review: `docs/10-security.md`.
