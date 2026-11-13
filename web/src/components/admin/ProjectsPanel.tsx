import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, FolderGit2, GitBranch, Github, Hourglass, Search, Users } from "lucide-react";
import { call } from "./api";
import { Card, Chip, EmptyState, HelpButton, inputCls, Notice, PanelHeader, Pills } from "./ui";

/* Every project at a glance, from the admin's side: who owns it, where its code is, whether its
 * GitHub connection works, and how much waits for review. Metadata only: opening a project's
 * content is a separate step and is audited (D-050). */

export interface ProjectOverview {
  id: string;
  name: string;
  source_type: string;
  iac_path: string;
  created_at: string;
  owner?: { oid: string; name: string; email: string };
  members: number;
  pending_changesets: number;
  repository?: {
    full_name: string;
    folder: string;
    default_branch: string;
    second_reviewer: boolean;
    account?: string;
    connection_status?: "active" | "needs_attention" | "disconnected";
    /** Whether GitHub still lets the owner reach the repository (D-061). */
    access_status?: "ok" | "needs_attention";
  };
}
interface UserRow {
  oid: string;
  email: string;
  name: string;
  status?: string;
}
interface Member {
  user_oid: string;
}

type Filter = "all" | "github" | "local" | "attention" | "review";

/** The owner lost access to the repository on GitHub; only the owner can repair it. */
export const ownerLostAccess = (p: ProjectOverview) => p.repository?.access_status === "needs_attention";

/** A project needs an admin's eye when its GitHub connection is not healthy or its owner lost access. */
export const needsAttention = (p: ProjectOverview) =>
  !!p.repository && (p.repository.connection_status !== "active" || ownerLostAccess(p));

export function filterProjects(list: ProjectOverview[], filter: Filter, q: string): ProjectOverview[] {
  const needle = q.trim().toLowerCase();
  return list.filter((p) => {
    if (filter === "github" && p.source_type !== "github") return false;
    if (filter === "local" && p.source_type === "github") return false;
    if (filter === "attention" && !needsAttention(p)) return false;
    if (filter === "review" && p.pending_changesets === 0) return false;
    if (!needle) return true;
    return `${p.name} ${p.repository?.full_name ?? ""} ${p.owner?.name ?? ""} ${p.owner?.email ?? ""}`.toLowerCase().includes(needle);
  });
}

const CONN: Record<string, { label: string; tone: "good" | "warn" | "neutral" }> = {
  active: { label: "Connected", tone: "good" },
  needs_attention: { label: "Needs attention", tone: "warn" },
  disconnected: { label: "Disconnected", tone: "neutral" },
};

export function ProjectsPanel({ onDenied, onChanged, onHelp }: { onDenied: () => void; onChanged?: () => void; onHelp: () => void }) {
  const [projects, setProjects] = useState<ProjectOverview[] | null>(null);
  const [users, setUsers] = useState<UserRow[]>([]);
  const [open, setOpen] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);
  const [filter, setFilter] = useState<Filter>("all");
  const [q, setQ] = useState("");

  const load = useCallback(async () => {
    const [p, u] = await Promise.all([call<ProjectOverview[]>("/admin/projects/overview"), call<UserRow[]>("/admin/users")]);
    if (p.status === 403 || u.status === 403) return onDenied();
    if (!p.ok) setMsg({ tone: "error", text: p.error });
    setProjects(p.data ?? []);
    setUsers((u.data ?? []).filter((x) => !x.status || x.status === "active"));
    onChanged?.();
  }, [onDenied, onChanged]);

  useEffect(() => {
    load();
  }, []);

  const list = projects ?? [];
  const shown = useMemo(() => filterProjects(list, filter, q), [list, filter, q]);
  const attention = list.filter(needsAttention).length;
  const waiting = list.reduce((n, p) => n + p.pending_changesets, 0);

  return (
    <div>
      <PanelHeader
        title="Projects"
        intro="Every project with its owner, repository and connection, and how much is waiting for review. You see this without opening any project's content."
        actions={<HelpButton onClick={onHelp} label="Docs" />}
      />
      {msg && <Notice tone={msg.tone} onClose={() => setMsg(null)}>{msg.text}</Notice>}

      {projects === null ? (
        <p className="text-sm text-[var(--tf-color-text-muted)]">Loading…</p>
      ) : list.length === 0 ? (
        <EmptyState icon={<FolderGit2 className="size-5" />} title="No projects yet">
          Projects appear here when someone creates one from a GitHub repository on the home page.
        </EmptyState>
      ) : (
        <>
          <div className="mb-4 grid gap-3 sm:grid-cols-3">
            <Card><p className="m-0 text-2xl font-bold">{list.length}</p><p className="m-0 text-xs text-[var(--tf-color-text-muted)]">projects</p></Card>
            <Card>
              <p className={`m-0 text-2xl font-bold ${attention ? "text-[var(--tf-color-warning)]" : ""}`}>{attention}</p>
              <p className="m-0 text-xs text-[var(--tf-color-text-muted)]">GitHub connections needing attention</p>
            </Card>
            <Card><p className="m-0 text-2xl font-bold">{waiting}</p><p className="m-0 text-xs text-[var(--tf-color-text-muted)]">changes waiting for review</p></Card>
          </div>

          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <Pills
              value={filter}
              onChange={setFilter}
              items={[
                { id: "all", label: "All", count: list.length },
                { id: "github", label: "GitHub", count: list.filter((p) => p.source_type === "github").length },
                { id: "local", label: "Local", count: list.filter((p) => p.source_type !== "github").length },
                { id: "attention", label: "Needs attention", count: attention },
                { id: "review", label: "Waiting for review", count: list.filter((p) => p.pending_changesets > 0).length },
              ]}
            />
            <div className="relative w-full max-w-xs">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
              <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search name, repository, owner" aria-label="Search projects" className={`${inputCls} mt-0 pl-9`} />
            </div>
          </div>

          <ul className="m-0 list-none space-y-3 p-0">
            {shown.length === 0 && <li className="py-6 text-center text-sm text-[var(--tf-color-text-muted)]">No project matches.</li>}
            {shown.map((p) => (
              <li key={p.id}>
                <Card>
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0">
                      <p className="m-0 flex items-center gap-2 font-semibold">
                        {p.source_type === "github" ? <Github className="size-4" /> : <FolderGit2 className="size-4" />}
                        {p.name}
                        {p.pending_changesets > 0 && (
                          <Chip tone="info">
                            <Hourglass className="mr-1 inline size-3" />
                            {p.pending_changesets} to review
                          </Chip>
                        )}
                      </p>
                      {p.repository ? (
                        <p className="m-0 mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--tf-color-text-secondary)]">
                          <span className="font-mono">{p.repository.full_name}</span>
                          <span className="flex items-center gap-1 font-mono"><GitBranch className="size-3" />{p.repository.default_branch}</span>
                          <span className="font-mono">/{p.repository.folder}</span>
                        </p>
                      ) : (
                        <p className="m-0 mt-1 font-mono text-xs text-[var(--tf-color-text-muted)]">{p.iac_path}</p>
                      )}
                    </div>
                    <button className="tf-btn tf-btn--ghost" onClick={() => setOpen(open === p.id ? null : p.id)}>
                      {open === p.id ? "Close" : "Manage"}
                    </button>
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs text-[var(--tf-color-text-secondary)]">
                    <span className="flex items-center gap-1"><Users className="size-3.5" />{p.owner ? `${p.owner.name || p.owner.email} (owner)` : "no owner"} · {p.members} {p.members === 1 ? "member" : "members"}</span>
                    {p.repository && (
                      <span className="flex items-center gap-1.5">
                        <Chip tone={CONN[p.repository.connection_status ?? "active"]?.tone ?? "neutral"}>{CONN[p.repository.connection_status ?? "active"]?.label ?? p.repository.connection_status}</Chip>
                        {p.repository.account && <span>via @{p.repository.account}</span>}
                        {p.repository.second_reviewer && <span>· second reviewer required</span>}
                      </span>
                    )}
                    {needsAttention(p) && (
                      <span className="flex items-center gap-1 text-[var(--tf-color-warning)]">
                        <AlertTriangle className="size-3.5" />{" "}
                        {ownerLostAccess(p)
                          ? "Blocked: GitHub says the owner can no longer access the repository. The owner must sign in with GitHub or connect it again, or transfer the project."
                          : "Publishing is blocked until the connection is checked (People & access, Connections)."}
                      </span>
                    )}
                  </div>
                  {open === p.id && (
                    <ProjectEditor
                      project={p}
                      users={users}
                      onDenied={onDenied}
                      onDone={(text) => { setMsg({ tone: "ok", text }); load(); }}
                      onError={(text) => setMsg({ tone: "error", text })}
                      onDeleted={() => { setOpen(null); load(); }}
                    />
                  )}
                </Card>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

function ProjectEditor(props: {
  project: ProjectOverview;
  users: UserRow[];
  onDenied: () => void;
  onDone: (text: string) => void;
  onError: (text: string) => void;
  onDeleted: () => void;
}) {
  const { project, users, onDenied, onDone, onError, onDeleted } = props;
  const [name, setName] = useState(project.name);
  const [path, setPath] = useState(project.iac_path);
  const [members, setMembers] = useState<Set<string> | null>(null);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    call<Member[]>(`/projects/${project.id}/members`).then((r) => {
      if (r.status === 403) return onDenied();
      setMembers(new Set((r.data ?? []).map((m) => m.user_oid)));
    });
  }, [project.id, onDenied]);

  const save = async () => {
    const body: Record<string, string> = {};
    if (name.trim() !== project.name) body.name = name.trim();
    if (path.trim() !== project.iac_path) body.iac_path = path.trim();
    if (Object.keys(body).length === 0) return onError("Nothing to change.");
    const r = await call(`/projects/${project.id}`, { method: "PATCH", json: body });
    if (r.ok) onDone("Project saved."); else onError(r.error);
  };

  const saveMembers = async () => {
    const r = await call(`/projects/${project.id}/members`, { method: "PUT", json: { user_oids: [...(members ?? [])] } });
    if (r.ok) onDone("Members saved."); else onError(r.error);
  };

  const remove = async () => {
    if (!confirm(`Delete "${project.name}" from TerraFlow? Repository files are kept.`)) return;
    let r = await call<{ pending_changesets?: number }>(`/projects/${project.id}`, { method: "DELETE" });
    if (r.status === 409) {
      const n = r.data?.pending_changesets ?? 0;
      if (!confirm(`${n} changeset(s) are waiting for review. Delete anyway?`)) return;
      r = await call(`/projects/${project.id}?force=true`, { method: "DELETE" });
    }
    if (r.ok) onDeleted(); else onError(r.error);
  };

  const toggle = (oid: string) =>
    setMembers((cur) => {
      const next = new Set(cur ?? []);
      if (next.has(oid)) next.delete(oid); else next.add(oid);
      return next;
    });

  const visible = users.filter((u) => `${u.name} ${u.email}`.toLowerCase().includes(filter.trim().toLowerCase()));

  return (
    <div className="mt-4 grid gap-6 border-t border-[var(--tf-color-border)] pt-4 md:grid-cols-2">
      <div className="grid content-start gap-3">
        <h3 className="m-0 text-sm font-semibold">Details</h3>
        <label className="text-sm font-medium">
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} className={inputCls} />
        </label>
        <label className="text-sm font-medium">
          Folder
          <input value={path} onChange={(e) => setPath(e.target.value)} className={`${inputCls} font-mono text-xs`} />
          {project.source_type === "github" && (
            <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">For a GitHub project the folder is fixed by its repository link.</span>
          )}
        </label>
        <div className="flex flex-wrap gap-2">
          <button className="tf-btn" onClick={save}>Save</button>
          <button className="tf-btn tf-btn--ghost" style={{ color: "var(--tf-color-error)" }} onClick={remove}>Delete project</button>
        </div>
      </div>
      <div>
        <h3 className="m-0 mb-2 text-sm font-semibold">Assign people</h3>
        {users.length === 0 ? (
          <p className="text-sm text-[var(--tf-color-text-muted)]">Nobody active has signed in yet, so nobody can be assigned.</p>
        ) : (
          <>
            {users.length > 6 && <input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Filter people" aria-label="Filter people" className={`${inputCls} mt-0 mb-2`} />}
            <ul className="m-0 mb-3 max-h-48 list-none space-y-1 overflow-y-auto rounded-lg border border-[var(--tf-color-border)] p-2">
              {visible.map((u) => (
                <li key={u.oid}>
                  <label className="flex cursor-pointer items-center gap-2 rounded px-1.5 py-1 text-sm hover:bg-[var(--tf-glass-hover)]">
                    <input type="checkbox" checked={members?.has(u.oid) ?? false} disabled={members === null} onChange={() => toggle(u.oid)} />
                    <span className="min-w-0 flex-1 truncate">{u.name || u.email}</span>
                    <span className="truncate text-xs text-[var(--tf-color-text-muted)]">{u.email}</span>
                  </label>
                </li>
              ))}
            </ul>
          </>
        )}
        <button className="tf-btn" disabled={members === null || members.size === 0} onClick={saveMembers}>Save people</button>
        <p className="m-0 mt-1.5 text-xs text-[var(--tf-color-text-muted)]">
          The project's owner and co-owners manage roles from the project's Team screen. This list only decides who can open the project. The owner is never removed.
        </p>
      </div>
    </div>
  );
}
