import { useState } from "react";
import {
  Boxes,
  Check,
  ChevronRight,
  CircleAlert,
  FileJson,
  FolderGit2,
  Plus,
  Search,
  ShieldCheck,
  Upload,
  Users,
} from "lucide-react";
import { CodeBlock } from "@/components/ui/code-block";
import { getAzureIcon } from "./nodes/domainStyles";

/* ------------------------------------------------------------------
 * MOCKUP: admin area. Static data only, nothing here is saved or sent.
 * It shows how module management (D-015/D-018), projects and access
 * could look, using the same design tokens as the rest of the app.
 * ------------------------------------------------------------------ */

type Status = "published" | "draft" | "deprecated";

interface Mod {
  name: string;
  title: string;
  description: string;
  domain: string;
  folder: string;
  file: string;
  status: Status;
  versions: { v: string; status: Status; date: string; note: string }[];
  usedBy: number;
  container?: boolean;
  parents?: string[];
  nest?: string;
  updated: string;
}

const MODULES: Mod[] = [
  { name: "resource-group", title: "Resource Group", description: "Azure Resource Group with optional management lock.", domain: "General", folder: "common", file: "rg.tf", status: "published", usedBy: 14, container: true, updated: "2 days ago",
    versions: [{ v: "v1", status: "published", date: "12 Oct 2026", note: "Initial release" }] },
  { name: "virtual-network", title: "Virtual Network", description: "Azure Virtual Network with subnets.", domain: "Networking", folder: "vnet", file: "vnet.tf", status: "published", usedBy: 9, container: true, parents: ["resource-group"], updated: "5 days ago",
    versions: [{ v: "v2", status: "draft", date: "6 Nov 2026", note: "Add flow logs input" }, { v: "v1", status: "published", date: "12 Oct 2026", note: "Initial release" }] },
  { name: "subnet", title: "Subnet", description: "A subnet inside a virtual network.", domain: "Networking", folder: "vnet", file: "(nested)", status: "published", usedBy: 22, container: true, parents: ["virtual-network"], nest: "subnets", updated: "1 week ago",
    versions: [{ v: "v1", status: "published", date: "1 Nov 2026", note: "Nested into VNet" }] },
  { name: "private-endpoint", title: "Private Endpoint", description: "Private connectivity to an Azure PaaS resource.", domain: "Networking", folder: "vnet", file: "pe.tf", status: "published", usedBy: 11, parents: ["subnet"], updated: "1 week ago",
    versions: [{ v: "v1", status: "published", date: "1 Nov 2026", note: "Initial release" }] },
  { name: "linux-virtual-machine", title: "Linux Virtual Machine", description: "Linux VM with SSH key authentication.", domain: "Compute", folder: "compute", file: "vm.tf", status: "published", usedBy: 6, parents: ["subnet"], updated: "3 days ago",
    versions: [{ v: "v1", status: "published", date: "3 Nov 2026", note: "Initial release" }] },
  { name: "key-vault", title: "Key Vault", description: "Azure Key Vault with optional private endpoint.", domain: "Security", folder: "keyvault", file: "kv.tf", status: "published", usedBy: 8, parents: ["resource-group"], updated: "2 weeks ago",
    versions: [{ v: "v1", status: "published", date: "12 Oct 2026", note: "Initial release" }] },
  { name: "storage-account", title: "Storage Account", description: "Azure Storage Account with optional blob private endpoint.", domain: "Data", folder: "storage", file: "storage.tf", status: "published", usedBy: 12, parents: ["resource-group"], updated: "2 weeks ago",
    versions: [{ v: "v1", status: "published", date: "12 Oct 2026", note: "Initial release" }] },
  { name: "network-security-group", title: "Network Security Group", description: "Azure NSG with security rules and subnet associations.", domain: "Networking", folder: "vnet", file: "nsg.tf", status: "published", usedBy: 7, parents: ["resource-group"], updated: "3 weeks ago",
    versions: [{ v: "v1", status: "published", date: "12 Oct 2026", note: "Initial release" }] },
  { name: "managed-identity", title: "Managed Identity", description: "User-assigned managed identity.", domain: "Security", folder: "common", file: "identity.tf", status: "draft", usedBy: 0, parents: ["resource-group"], updated: "today",
    versions: [{ v: "v1", status: "draft", date: "8 Nov 2026", note: "Awaiting schema review" }] },
  { name: "private-dns-zone", title: "Private DNS Zone", description: "Private DNS zone, optionally linked to VNets.", domain: "Networking", folder: "vnet", file: "dns.tf", status: "deprecated", usedBy: 3, parents: ["resource-group"], updated: "1 month ago",
    versions: [{ v: "v1", status: "deprecated", date: "20 Sep 2026", note: "Replaced by the DNS platform module" }] },
];

const PROJECTS = [
  { name: "Project 1 (Dev)", group: "grp-platform-dev", envs: "dev · staging · prod", modules: 10, last: "today", members: 8 },
  { name: "Payments platform", group: "grp-payments", envs: "dev · prod", modules: 7, last: "yesterday", members: 5 },
  { name: "Data lake", group: "grp-data", envs: "dev · staging · prod", modules: 6, last: "3 days ago", members: 4 },
];

const GROUPS = [
  { name: "grp-devops", id: "5d8e…a1c2", role: "devops", members: 6, note: "Approve changesets, manage modules" },
  { name: "grp-platform-dev", id: "9b21…77ef", role: "developer", members: 14, note: "Compose and publish changesets" },
  { name: "grp-payments", id: "c0aa…3d18", role: "developer", members: 9, note: "Compose and publish changesets" },
  { name: "grp-auditors", id: "1fe9…b640", role: "read-only", members: 3, note: "View canvas, review and audit log" },
];

const STATUS_TONE: Record<Status, string> = { published: "good", draft: "info", deprecated: "warn" };
const STATUS_LABEL: Record<Status, string> = { published: "Published", draft: "Draft", deprecated: "Deprecated" };

function schemaFor(m: Mod): string {
  const canvas: Record<string, unknown> = { domain: m.domain.toLowerCase() };
  if (m.container) canvas.container = true;
  if (m.parents) canvas.parents = m.parents;
  if (m.nest) canvas.nest = { field: m.nest };
  return JSON.stringify(
    {
      title: m.title,
      description: m.description,
      "x-tf-module": { source: m.name, version: "v1", target_folder: m.folder, filename: m.file.startsWith("(") ? undefined : m.file },
      "x-tf-canvas": canvas,
      required: ["name", "resource_group_name", "location"],
      properties: {
        name: { type: "string", title: `${m.title} Name`, "x-tf-ui": { group: "basics", order: 1 } },
        resource_group_name: { type: "string", "x-tf-ui": { widget: "pick-existing", pick: { module_type: "resource-group", output: "name" } } },
        location: { type: "string", default: "southeastasia" },
      },
    },
    null,
    2,
  );
}

export function AdminView() {
  const [tab, setTab] = useState<"modules" | "projects" | "access">("modules");
  const [sel, setSel] = useState<string>(MODULES[0].name);
  const [detailTab, setDetailTab] = useState<"overview" | "schema" | "canvas" | "versions">("overview");
  const [q, setQ] = useState("");
  const [statusFilter, setStatusFilter] = useState<Status | "all">("all");
  const [toast, setToast] = useState<string | null>(null);

  const say = (m: string) => {
    setToast(m);
    setTimeout(() => setToast(null), 2600);
  };
  const mod = MODULES.find((m) => m.name === sel) ?? MODULES[0];
  const list = MODULES.filter(
    (m) =>
      (statusFilter === "all" || m.status === statusFilter) &&
      (q === "" || m.title.toLowerCase().includes(q.toLowerCase()) || m.name.includes(q.toLowerCase())),
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden bg-[var(--tf-color-canvas)]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] px-6 py-4">
        <div>
          <div className="flex items-center gap-2">
            <ShieldCheck className="size-5 text-[var(--tf-color-action)]" />
            <h2 className="text-lg font-semibold">Admin</h2>
            <span className="tf-badge" data-tone="warn">Mockup · not connected</span>
          </div>
          <p className="mt-0.5 text-sm text-[var(--tf-color-text-muted)]">Manage the module catalog, projects and who can do what.</p>
        </div>
        <div className="flex gap-1 rounded-lg bg-[var(--tf-color-surface-subtle)] p-1" role="tablist">
          {([["modules", "Modules", Boxes], ["projects", "Projects", FolderGit2], ["access", "Roles & access", Users]] as const).map(([k, label, Icon]) => (
            <button
              key={k}
              role="tab"
              aria-selected={tab === k}
              onClick={() => setTab(k)}
              className={`flex cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-sm font-medium ${
                tab === k ? "bg-[var(--tf-color-surface-raised)] text-[var(--tf-color-text)] shadow-sm" : "bg-transparent text-[var(--tf-color-text-muted)]"
              }`}
            >
              <Icon className="size-4" /> {label}
            </button>
          ))}
        </div>
      </div>

      {tab === "modules" && (
        <div className="flex min-h-0 flex-1">
          {/* list */}
          <section className="flex min-w-0 flex-1 flex-col">
            <div className="flex flex-wrap items-center gap-3 px-6 py-4">
              <div className="relative min-w-[220px] flex-1">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
                <input className="tf-input" style={{ paddingLeft: 34 }} placeholder="Search modules…" value={q} onChange={(e) => setQ(e.target.value)} />
              </div>
              <div className="flex gap-1">
                {(["all", "published", "draft", "deprecated"] as const).map((s) => (
                  <button
                    key={s}
                    onClick={() => setStatusFilter(s)}
                    className={`cursor-pointer rounded-full border px-3 py-1 text-xs font-medium ${
                      statusFilter === s
                        ? "border-[var(--tf-color-action)] bg-[var(--tf-color-surface-selected)] text-[var(--tf-color-action)]"
                        : "border-[var(--tf-color-border)] bg-transparent text-[var(--tf-color-text-secondary)]"
                    }`}
                  >
                    {s === "all" ? "All" : STATUS_LABEL[s]}
                  </button>
                ))}
              </div>
              <button className="tf-btn tf-btn--ghost" onClick={() => say("Mockup: upload a schema.json or paste variables.tf")}>
                <Upload className="size-4" /> Import
              </button>
              <button className="tf-btn" onClick={() => say("Mockup: opens the register-module wizard")}>
                <Plus className="size-4" /> Register module
              </button>
            </div>
            <div className="min-h-0 flex-1 overflow-auto px-6 pb-6">
              <div className="overflow-hidden rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] shadow-sm">
                <table className="w-full border-collapse text-left text-sm">
                  <thead className="bg-[var(--tf-color-surface-subtle)] text-xs uppercase tracking-wide text-[var(--tf-color-text-muted)]">
                    <tr>
                      {["Module", "Latest", "Status", "Writes to", "Used in", "Updated", ""].map((h) => (
                        <th key={h} className="px-4 py-2.5 font-semibold">{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {list.map((m) => {
                      const icon = getAzureIcon(m.name);
                      return (
                        <tr
                          key={m.name}
                          onClick={() => setSel(m.name)}
                          className={`cursor-pointer border-t border-[var(--tf-color-border)] ${
                            sel === m.name ? "bg-[var(--tf-color-surface-selected)]" : "hover:bg-[var(--tf-color-surface-subtle)]"
                          }`}
                        >
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-3">
                              <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-[var(--tf-color-surface-subtle)]">
                                {icon ? <img src={icon} alt="" className="size-5" /> : <Boxes className="size-4" />}
                              </span>
                              <div className="min-w-0">
                                <div className="font-medium">{m.title}</div>
                                <div className="truncate font-mono text-xs text-[var(--tf-color-text-muted)]">{m.name}</div>
                              </div>
                            </div>
                          </td>
                          <td className="px-4 py-3 font-mono text-xs">{m.versions[m.versions.length > 1 && m.versions[0].status === "draft" ? 1 : 0].v}</td>
                          <td className="px-4 py-3"><span className="tf-badge" data-tone={STATUS_TONE[m.status]}>{STATUS_LABEL[m.status]}</span></td>
                          <td className="px-4 py-3 font-mono text-xs">{m.folder}/{m.file}</td>
                          <td className="px-4 py-3">{m.usedBy} <span className="text-[var(--tf-color-text-muted)]">changesets</span></td>
                          <td className="px-4 py-3 text-[var(--tf-color-text-muted)]">{m.updated}</td>
                          <td className="px-3 py-3 text-[var(--tf-color-text-muted)]"><ChevronRight className="size-4" /></td>
                        </tr>
                      );
                    })}
                    {list.length === 0 && (
                      <tr><td colSpan={7} className="px-4 py-10 text-center text-[var(--tf-color-text-muted)]">No modules match.</td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </section>

          {/* detail */}
          <aside className="flex w-[460px] max-w-[45vw] shrink-0 flex-col border-l border-[var(--tf-color-border)] bg-[var(--tf-color-surface)]">
            <div className="border-b border-[var(--tf-color-border)] p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold">{mod.title}</h3>
                  <p className="font-mono text-xs text-[var(--tf-color-text-muted)]">{mod.name}</p>
                </div>
                <span className="tf-badge" data-tone={STATUS_TONE[mod.status]}>{STATUS_LABEL[mod.status]}</span>
              </div>
              <div className="mt-4 flex gap-1 rounded-lg bg-[var(--tf-color-surface-subtle)] p-1" role="tablist">
                {([["overview", "Overview"], ["schema", "Schema"], ["canvas", "Canvas rules"], ["versions", "Versions"]] as const).map(([k, label]) => (
                  <button
                    key={k}
                    role="tab"
                    aria-selected={detailTab === k}
                    onClick={() => setDetailTab(k)}
                    className={`flex-1 cursor-pointer rounded-md border-0 px-2 py-1.5 text-xs font-medium ${
                      detailTab === k ? "bg-[var(--tf-color-surface-raised)] text-[var(--tf-color-text)] shadow-sm" : "bg-transparent text-[var(--tf-color-text-muted)]"
                    }`}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>

            <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-5">
              {detailTab === "overview" && (
                <>
                  <Field label="Display name"><input className="tf-input" defaultValue={mod.title} key={mod.name + "t"} /></Field>
                  <Field label="Description"><textarea className="tf-input" style={{ padding: "8px 12px" }} rows={2} defaultValue={mod.description} key={mod.name + "d"} /></Field>
                  <div className="grid grid-cols-2 gap-3">
                    <Field label="Folder"><input className="tf-input" defaultValue={mod.folder} key={mod.name + "f"} /></Field>
                    <Field label="Default file"><input className="tf-input" defaultValue={mod.file.startsWith("(") ? "" : mod.file} placeholder="main.tf" key={mod.name + "n"} disabled={mod.file.startsWith("(")} /></Field>
                  </div>
                  {mod.file.startsWith("(") && <p className="text-xs text-[var(--tf-color-text-muted)]">Nested modules are folded into their parent, so they have no file of their own.</p>}
                  <Field label="Domain colour">
                    <select className="tf-input" defaultValue={mod.domain} key={mod.name + "c"}>
                      {["General", "Networking", "Compute", "Data", "Security"].map((d) => <option key={d}>{d}</option>)}
                    </select>
                  </Field>
                  <div className="rounded-lg border border-[var(--tf-color-border)] p-3 text-sm">
                    <div className="font-medium">Usage</div>
                    <p className="mt-1 text-[var(--tf-color-text-muted)]">Used in {mod.usedBy} changesets across {Math.min(3, Math.ceil(mod.usedBy / 4))} projects.</p>
                  </div>
                </>
              )}

              {detailTab === "schema" && (
                <>
                  <div className="flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-sm font-medium"><FileJson className="size-4" /> schema.json (v1)</span>
                    <button className="tf-btn tf-btn--ghost" style={{ minHeight: 30 }} onClick={() => say("Mockup: validates the schema and renders a live form preview")}>
                      <Check className="size-4" /> Validate
                    </button>
                  </div>
                  <CodeBlock language="json" filename="schema.json" code={schemaFor(mod)} className="h-[380px]" />
                  <div className="rounded-lg border border-[var(--tf-color-border)] bg-[var(--tf-color-success-surface)] p-3 text-sm text-[var(--tf-color-success)]">
                    <Check className="mr-1 inline size-4" /> Valid JSON Schema. 3 inputs, 1 pick-existing binding.
                  </div>
                </>
              )}

              {detailTab === "canvas" && (
                <>
                  <p className="text-sm text-[var(--tf-color-text-muted)]">How this module behaves on the drag-and-drop canvas (the schema's <code>x-tf-canvas</code> block).</p>
                  <Toggle label="Can contain other resources" hint="Shown as a coloured block with a colour picker." on={!!mod.container} />
                  <Toggle label="Must be placed in a parent" hint="Shows a red border until it is dropped inside one." on={mod.name === "subnet" || mod.name === "private-endpoint" || mod.name === "linux-virtual-machine"} />
                  <Toggle label="Fold into the parent module" hint={`Becomes an entry of the parent's "${mod.nest ?? "…"}" input instead of its own module call.`} on={!!mod.nest} />
                  <Field label="Allowed parents">
                    <div className="flex flex-wrap gap-2">
                      {(mod.parents ?? []).map((p) => <span key={p} className="tf-ident">{p}</span>)}
                      {(mod.parents ?? []).length === 0 && <span className="text-sm text-[var(--tf-color-text-muted)]">Anywhere on the canvas</span>}
                      <button className="tf-btn tf-btn--ghost" style={{ minHeight: 26, fontSize: 12 }} onClick={() => say("Mockup: pick another module")}><Plus className="size-3.5" /> Add</button>
                    </div>
                  </Field>
                </>
              )}

              {detailTab === "versions" && (
                <>
                  <ul className="m-0 list-none space-y-2 p-0">
                    {mod.versions.map((v) => (
                      <li key={v.v} className="rounded-lg border border-[var(--tf-color-border)] p-3">
                        <div className="flex items-center justify-between">
                          <span className="font-mono text-sm font-semibold">{v.v}</span>
                          <span className="tf-badge" data-tone={STATUS_TONE[v.status]}>{STATUS_LABEL[v.status]}</span>
                        </div>
                        <p className="mt-1 text-sm text-[var(--tf-color-text-secondary)]">{v.note}</p>
                        <div className="mt-2 flex items-center justify-between text-xs text-[var(--tf-color-text-muted)]">
                          <span>{v.date}</span>
                          <span className="flex gap-2">
                            {v.status === "draft" && <button className="tf-btn" style={{ minHeight: 26, fontSize: 12 }} onClick={() => say("Mockup: publishes this version")}>Publish</button>}
                            {v.status === "published" && <button className="tf-btn tf-btn--ghost" style={{ minHeight: 26, fontSize: 12 }} onClick={() => say("Mockup: marks the version deprecated")}>Deprecate</button>}
                          </span>
                        </div>
                      </li>
                    ))}
                  </ul>
                  <button className="tf-btn tf-btn--ghost" onClick={() => say("Mockup: starts a new version from the latest schema")}><Plus className="size-4" /> New version</button>
                  <p className="flex items-start gap-1.5 text-xs text-[var(--tf-color-text-muted)]"><CircleAlert className="mt-0.5 size-3.5 shrink-0" /> Existing changesets keep the version they were built with; deprecating hides it from the catalog only.</p>
                </>
              )}
            </div>

            <div className="flex items-center justify-between gap-2 border-t border-[var(--tf-color-border)] p-4">
              <button className="tf-btn tf-btn--ghost" onClick={() => say("Mockup: discards unsaved edits")}>Discard</button>
              <button className="tf-btn" onClick={() => say("Mockup: saves as a new draft version")}>Save changes</button>
            </div>
          </aside>
        </div>
      )}

      {tab === "projects" && (
        <div className="min-h-0 flex-1 overflow-auto p-6">
          <div className="mx-auto max-w-5xl">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-base font-semibold">Projects</h3>
              <button className="tf-btn" onClick={() => say("Mockup: opens the new-project form")}><Plus className="size-4" /> New project</button>
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              {PROJECTS.map((p) => (
                <div key={p.name} className="rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-5 shadow-sm">
                  <div className="flex items-center gap-3">
                    <span className="flex size-10 items-center justify-center rounded-xl bg-[var(--tf-color-surface-selected)] text-[var(--tf-color-action)]"><FolderGit2 className="size-5" /></span>
                    <div>
                      <div className="font-semibold">{p.name}</div>
                      <div className="text-xs text-[var(--tf-color-text-muted)]">{p.envs}</div>
                    </div>
                  </div>
                  <dl className="mt-4 grid grid-cols-3 gap-3 text-sm">
                    <Stat label="Modules" value={String(p.modules)} />
                    <Stat label="Members" value={String(p.members)} />
                    <Stat label="Last change" value={p.last} />
                  </dl>
                  <div className="mt-4 flex items-center justify-between border-t border-[var(--tf-color-border)] pt-3 text-xs">
                    <span className="text-[var(--tf-color-text-muted)]">Entra group</span>
                    <code>{p.group}</code>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {tab === "access" && (
        <div className="min-h-0 flex-1 overflow-auto p-6">
          <div className="mx-auto max-w-5xl">
            <div className="mb-4 flex items-center justify-between">
              <div>
                <h3 className="text-base font-semibold">Entra groups → roles</h3>
                <p className="text-sm text-[var(--tf-color-text-muted)]">Roles come from group claims at sign-in; nothing here stores passwords.</p>
              </div>
              <button className="tf-btn" onClick={() => say("Mockup: map another Entra group")}><Plus className="size-4" /> Map group</button>
            </div>
            <div className="overflow-hidden rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] shadow-sm">
              <table className="w-full border-collapse text-left text-sm">
                <thead className="bg-[var(--tf-color-surface-subtle)] text-xs uppercase tracking-wide text-[var(--tf-color-text-muted)]">
                  <tr>{["Group", "Group ID", "Role", "Members", "Can"].map((h) => <th key={h} className="px-4 py-2.5 font-semibold">{h}</th>)}</tr>
                </thead>
                <tbody>
                  {GROUPS.map((g) => (
                    <tr key={g.name} className="border-t border-[var(--tf-color-border)]">
                      <td className="px-4 py-3 font-medium">{g.name}</td>
                      <td className="px-4 py-3 font-mono text-xs text-[var(--tf-color-text-muted)]">{g.id}</td>
                      <td className="px-4 py-3"><span className="tf-badge" data-tone={g.role === "devops" ? "good" : g.role === "developer" ? "info" : "neutral"}>{g.role}</span></td>
                      <td className="px-4 py-3">{g.members}</td>
                      <td className="px-4 py-3 text-[var(--tf-color-text-secondary)]">{g.note}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="mt-4 rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-5 shadow-sm">
              <h4 className="text-sm font-semibold">Review rules</h4>
              <div className="mt-3 space-y-3">
                <Toggle label="Separation of duties" hint="The requester cannot approve their own changeset." on />
                <Toggle label="Require a reason to reject" hint="Shown to the developer on the Changesets page." on />
              </div>
            </div>
          </div>
        </div>
      )}

      {toast && (
        <div role="status" className="tf-toast" style={{ position: "fixed", bottom: 24, left: "50%", transform: "translateX(-50%)", zIndex: 60 }}>
          {toast}
        </div>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="mb-1 block text-sm font-semibold">{label}</label>
      {children}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-[var(--tf-color-text-muted)]">{label}</dt>
      <dd className="m-0 font-medium">{value}</dd>
    </div>
  );
}

function Toggle({ label, hint, on }: { label: string; hint: string; on: boolean }) {
  const [v, setV] = useState(on);
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <div className="text-sm font-medium">{label}</div>
        <div className="text-xs text-[var(--tf-color-text-muted)]">{hint}</div>
      </div>
      <button
        role="switch"
        aria-checked={v}
        aria-label={label}
        onClick={() => setV(!v)}
        className="relative h-6 w-11 shrink-0 cursor-pointer rounded-full border-0 transition-colors"
        style={{ background: v ? "var(--tf-color-action)" : "var(--tf-color-border-strong)" }}
      >
        <span
          className="absolute top-0.5 size-5 rounded-full bg-white shadow transition-all"
          style={{ left: v ? 22 : 2 }}
        />
      </button>
    </div>
  );
}
