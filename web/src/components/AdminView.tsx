import { useCallback, useEffect, useState } from "react";
import type { Project } from "../types/project";
import { RawResourcesAdmin } from "./RawResourcesAdmin";
import { ConnectionsAdmin } from "./ConnectionsAdmin";

/* Admin area (devops role). Everything here talks to the real admin API
 * (D-037): modules, projects with their members, and users. */

interface ModuleRow {
  name: string;
  version: string;
  title: string;
  status: "published" | "deprecated";
  target_folder: string;
  imported_by: string;
  updated_at: string;
}
interface UserRow {
  oid: string;
  email: string;
  name: string;
  project_count: number;
  last_login_at?: string;
}
interface Member {
  user_oid: string;
  email: string;
  name: string;
}
interface Issue {
  path: string;
  code: string;
  message: string;
}
interface ImportCheck {
  report: { valid: boolean; errors?: Issue[]; warnings?: Issue[]; name?: string; version?: string };
  exists: "none" | "identical" | "different";
  compared_to?: string;
  changes?: { kind: string; path: string; detail: string; breaking: boolean }[];
  breaking_changes?: number;
}

interface Result<T> {
  ok: boolean;
  status: number;
  data: T | null;
  error: string;
}

async function call<T>(path: string, init?: RequestInit & { json?: unknown }): Promise<Result<T>> {
  const { json, ...rest } = init ?? {};
  const res = await fetch(`/api${path}`, {
    ...rest,
    headers: json !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: json !== undefined ? JSON.stringify(json) : undefined,
  });
  let data: unknown = null;
  try {
    data = await res.json();
  } catch {
    /* empty body */
  }
  const error = (data as { error?: string } | null)?.error ?? (res.ok ? "" : `Request failed (${res.status})`);
  return { ok: res.ok, status: res.status, data: data as T | null, error };
}

type Tab = "modules" | "resources" | "projects" | "connections" | "users";

export function AdminView({ onProjectsChanged }: { onProjectsChanged?: () => void }) {
  const [tab, setTab] = useState<Tab>("modules");
  const [denied, setDenied] = useState(false);

  if (denied) {
    return (
      <div className="p-8 text-sm text-[var(--tf-color-text-muted)]">
        The admin area needs the <strong>devops</strong> role.
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mx-auto max-w-5xl">
        <div role="tablist" className="mb-5 flex gap-1 border-b border-[var(--tf-color-border)]">
          {(["modules", "resources", "projects", "connections", "users"] as Tab[]).map((t) => (
            <button
              key={t}
              role="tab"
              aria-selected={tab === t}
              onClick={() => setTab(t)}
              className={`px-4 py-2 text-sm font-medium capitalize ${
                tab === t
                  ? "border-b-2 border-[var(--tf-color-action)] text-[var(--tf-color-text)]"
                  : "text-[var(--tf-color-text-muted)] hover:text-[var(--tf-color-text)]"
              }`}
            >
              {t === "resources" ? "raw resources" : t}
            </button>
          ))}
        </div>
        {tab === "modules" && <ModulesTab onDenied={() => setDenied(true)} />}
        {tab === "resources" && <RawResourcesAdmin onDenied={() => setDenied(true)} />}
        {tab === "projects" && <ProjectsTab onDenied={() => setDenied(true)} onChanged={onProjectsChanged} />}
        {tab === "connections" && <ConnectionsAdmin onDenied={() => setDenied(true)} />}
        {tab === "users" && <UsersTab onDenied={() => setDenied(true)} />}
      </div>
    </div>
  );
}

function Notice({ tone, children }: { tone: "error" | "ok"; children: React.ReactNode }) {
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      className="mb-3 rounded-lg border px-3 py-2 text-sm"
      style={{
        borderColor: tone === "error" ? "var(--tf-color-error)" : "var(--tf-color-success)",
        color: tone === "error" ? "var(--tf-color-error)" : "var(--tf-color-success)",
      }}
    >
      {children}
    </div>
  );
}

/* ---------------- Modules ---------------- */

function ModulesTab({ onDenied }: { onDenied: () => void }) {
  const [rows, setRows] = useState<ModuleRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);
  const [name, setName] = useState("");
  const [schema, setSchema] = useState("");
  const [check, setCheck] = useState<ImportCheck | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const r = await call<ModuleRow[]>("/admin/catalog/modules");
    if (r.status === 403) return onDenied();
    if (!r.ok) setMsg({ tone: "error", text: r.error });
    setRows(r.data ?? []);
    setLoading(false);
  }, [onDenied]);

  useEffect(() => {
    load();
  }, [load]);

  const parse = (): unknown | null => {
    try {
      return JSON.parse(schema);
    } catch {
      setMsg({ tone: "error", text: "The schema is not valid JSON." });
      return null;
    }
  };

  const validate = async () => {
    const parsed = parse();
    if (parsed === null) return;
    setBusy(true);
    setMsg(null);
    const r = await call<ImportCheck>("/admin/catalog/validate", { method: "POST", json: { name, schema: parsed } });
    setBusy(false);
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setCheck(r.data);
  };

  const doImport = async () => {
    const parsed = parse();
    if (parsed === null) return;
    setBusy(true);
    setMsg(null);
    const r = await call<{ status: string; result?: ImportCheck }>("/admin/catalog/modules", {
      method: "POST",
      json: { name, schema: parsed },
    });
    setBusy(false);
    if (r.status === 403) return onDenied();
    if (r.status === 422 && r.data?.result) setCheck(r.data.result);
    if (!r.ok) return setMsg({ tone: "error", text: r.error || "Import failed" });
    setMsg({ tone: "ok", text: r.data?.status === "unchanged" ? "Already published, nothing changed." : "Module imported and published." });
    setCheck(r.data?.result ?? null);
    load();
  };

  const setStatus = async (m: ModuleRow, status: ModuleRow["status"]) => {
    const r = await call(`/admin/catalog/modules/${encodeURIComponent(m.name)}/${encodeURIComponent(m.version)}`, {
      method: "PATCH",
      json: { status },
    });
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    load();
  };

  const onFile = async (f: File | undefined) => {
    if (!f) return;
    setSchema(await f.text());
    setCheck(null);
    if (!name) setName(f.name.replace(/\.json$/i, "").toLowerCase());
  };

  return (
    <div>
      {msg && <Notice tone={msg.tone}>{msg.text}</Notice>}
      <h2 className="mb-2 text-base font-semibold">Module versions</h2>
      {loading ? (
        <p className="text-sm text-[var(--tf-color-text-muted)]">Loading…</p>
      ) : (
        <table className="mb-8 w-full text-left text-sm">
          <thead className="text-xs uppercase text-[var(--tf-color-text-muted)]">
            <tr><th className="py-2">Module</th><th>Version</th><th>Status</th><th>Folder</th><th>Imported by</th><th><span className="sr-only">Actions</span></th></tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr><td colSpan={6} className="py-6 text-center text-[var(--tf-color-text-muted)]">No modules yet. Import one below.</td></tr>
            )}
            {rows.map((m) => (
              <tr key={`${m.name}@${m.version}`} className="border-t border-[var(--tf-color-border)]">
                <td className="py-2"><strong>{m.title || m.name}</strong><div className="font-mono text-xs text-[var(--tf-color-text-muted)]">{m.name}</div></td>
                <td className="font-mono text-xs">{m.version}</td>
                <td><span className="tf-badge" data-tone={m.status === "published" ? "good" : "neutral"}>{m.status}</span></td>
                <td className="font-mono text-xs">{m.target_folder}</td>
                <td className="text-xs">{m.imported_by || "seed"}</td>
                <td className="text-right">
                  {m.status === "published" ? (
                    <button className="tf-btn tf-btn--ghost" onClick={() => setStatus(m, "deprecated")}>Deprecate</button>
                  ) : (
                    <button className="tf-btn tf-btn--ghost" onClick={() => setStatus(m, "published")}>Publish again</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h2 className="mb-2 text-base font-semibold">Import a module schema</h2>
      <div className="grid gap-3">
        <label className="text-sm">
          Catalog name
          <input
            value={name}
            onChange={(e) => { setName(e.target.value); setCheck(null); }}
            placeholder="e.g. storage-account"
            className="mt-1 block w-full max-w-sm rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm"
          />
        </label>
        <label className="text-sm">
          Schema file (.json)
          <input type="file" accept="application/json,.json" onChange={(e) => onFile(e.target.files?.[0])} className="mt-1 block text-sm" />
        </label>
        <label className="text-sm">
          …or paste the JSON
          <textarea
            value={schema}
            onChange={(e) => { setSchema(e.target.value); setCheck(null); }}
            rows={8}
            spellCheck={false}
            className="mt-1 block w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 font-mono text-xs"
          />
        </label>
        <div className="flex gap-2">
          <button className="tf-btn tf-btn--ghost" disabled={busy || !name || !schema} onClick={validate}>Check</button>
          <button className="tf-btn" disabled={busy || !name || !schema} onClick={doImport}>Import and publish</button>
        </div>
        {check && <CheckReport check={check} />}
      </div>
    </div>
  );
}

function CheckReport({ check }: { check: ImportCheck }) {
  const { report } = check;
  return (
    <div className="rounded-lg border border-[var(--tf-color-border)] p-3 text-sm">
      <p className="mb-2">
        {report.valid ? "The schema is valid" : "The schema has errors"}
        {report.name && report.version ? ` — ${report.name}@${report.version}` : ""}.{" "}
        {check.exists === "identical" && "This exact version is already published."}
        {check.exists === "different" && "This version exists with different content, so it cannot be replaced."}
        {check.exists === "none" && "This is a new version."}
      </p>
      {(report.errors ?? []).map((e, i) => (
        <p key={`e${i}`} className="text-[var(--tf-color-error)]"><code>{e.path || "/"}</code> {e.message}</p>
      ))}
      {(report.warnings ?? []).map((e, i) => (
        <p key={`w${i}`} className="text-[var(--tf-color-text-muted)]"><code>{e.path || "/"}</code> {e.message}</p>
      ))}
      {(check.changes ?? []).length > 0 && (
        <div className="mt-2">
          <p className="font-medium">
            Changes since {check.compared_to}: {check.breaking_changes ?? 0} breaking
          </p>
          <ul className="m-0 list-disc pl-5">
            {check.changes!.map((c, i) => (
              <li key={i} className={c.breaking ? "text-[var(--tf-color-error)]" : undefined}>
                {c.kind} <code>{c.path}</code> {c.detail}{c.breaking ? " (breaking)" : ""}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

/* ---------------- Projects ---------------- */

function ProjectsTab({ onDenied, onChanged }: { onDenied: () => void; onChanged?: () => void }) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [users, setUsers] = useState<UserRow[]>([]);
  const [open, setOpen] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);

  const load = useCallback(async () => {
    const [p, u] = await Promise.all([call<Project[]>("/projects"), call<UserRow[]>("/admin/users")]);
    if (u.status === 403) return onDenied();
    setProjects(p.data ?? []);
    setUsers(u.data ?? []);
    onChanged?.();
  }, [onDenied, onChanged]);

  useEffect(() => {
    load();
  }, []);

  return (
    <div>
      {msg && <Notice tone={msg.tone}>{msg.text}</Notice>}
      {projects.length === 0 && <p className="text-sm text-[var(--tf-color-text-muted)]">No projects.</p>}
      <ul className="m-0 list-none space-y-3 p-0">
        {projects.map((p) => (
          <li key={p.id} className="rounded-xl border border-[var(--tf-color-border)] p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <strong>{p.name}</strong>
                <div className="font-mono text-xs text-[var(--tf-color-text-muted)]">{p.iac_path}</div>
              </div>
              <button className="tf-btn tf-btn--ghost" onClick={() => setOpen(open === p.id ? null : p.id)}>
                {open === p.id ? "Close" : "Manage"}
              </button>
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
          </li>
        ))}
      </ul>
    </div>
  );
}

function ProjectEditor(props: {
  project: Project;
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
    if (!confirm(`Delete "${project.name}" from the app? Repository files are kept.`)) return;
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

  return (
    <div className="mt-4 grid gap-5 border-t border-[var(--tf-color-border)] pt-4 md:grid-cols-2">
      <div className="grid content-start gap-2">
        <h3 className="text-sm font-semibold">Details</h3>
        <label className="text-sm">Name
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={80}
            className="mt-1 block w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm" />
        </label>
        <label className="text-sm">IaC folder
          <input value={path} onChange={(e) => setPath(e.target.value)}
            className="mt-1 block w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 font-mono text-xs" />
        </label>
        <div className="flex gap-2">
          <button className="tf-btn" onClick={save}>Save</button>
          <button className="tf-btn tf-btn--ghost" style={{ color: "var(--tf-color-error)" }} onClick={remove}>Delete project</button>
        </div>
      </div>
      <div>
        <h3 className="mb-2 text-sm font-semibold">Members</h3>
        {users.length === 0 ? (
          <p className="text-sm text-[var(--tf-color-text-muted)]">Nobody has signed in yet, so nobody can be assigned.</p>
        ) : (
          <ul className="m-0 mb-2 max-h-48 list-none space-y-1 overflow-y-auto p-0">
            {users.map((u) => (
              <li key={u.oid}>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={members?.has(u.oid) ?? false} disabled={members === null} onChange={() => toggle(u.oid)} />
                  {u.name || u.email} <span className="text-xs text-[var(--tf-color-text-muted)]">{u.email}</span>
                </label>
              </li>
            ))}
          </ul>
        )}
        <button className="tf-btn" disabled={members === null || members.size === 0} onClick={saveMembers}>Save members</button>
        {members !== null && members.size === 0 && (
          <p className="mt-1 text-xs text-[var(--tf-color-text-muted)]">Pick at least one member (admins see every project anyway).</p>
        )}
      </div>
    </div>
  );
}

/* ---------------- Users ---------------- */

function UsersTab({ onDenied }: { onDenied: () => void }) {
  const [users, setUsers] = useState<UserRow[] | null>(null);
  useEffect(() => {
    call<UserRow[]>("/admin/users").then((r) => {
      if (r.status === 403) return onDenied();
      setUsers(r.data ?? []);
    });
  }, [onDenied]);

  if (users === null) return <p className="text-sm text-[var(--tf-color-text-muted)]">Loading…</p>;
  return (
    <table className="w-full text-left text-sm">
      <thead className="text-xs uppercase text-[var(--tf-color-text-muted)]">
        <tr><th className="py-2">Name</th><th>Email</th><th>Projects</th><th>Last sign-in</th></tr>
      </thead>
      <tbody>
        {users.length === 0 && (
          <tr><td colSpan={4} className="py-6 text-center text-[var(--tf-color-text-muted)]">Nobody has signed in yet.</td></tr>
        )}
        {users.map((u) => (
          <tr key={u.oid} className="border-t border-[var(--tf-color-border)]">
            <td className="py-2">{u.name || "—"}</td>
            <td>{u.email}</td>
            <td>{u.project_count}</td>
            <td className="text-xs">{u.last_login_at ? new Date(u.last_login_at).toLocaleString() : "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
