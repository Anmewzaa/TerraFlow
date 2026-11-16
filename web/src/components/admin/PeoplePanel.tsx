import { useCallback, useEffect, useMemo, useState } from "react";
import { Github, Loader2, RefreshCw, Search, ShieldCheck, Unplug, UserCheck, Users } from "lucide-react";
import { disconnectConnection, getConnections, recheckConnection, type ConnectionRow } from "../../hooks/useGitHub";
import { call } from "./api";
import { Card, Chip, EmptyState, HelpButton, inputCls, Notice, PanelHeader, Pills } from "./ui";
import { PanelLoading } from "@/components/ui/skeleton";

/* People & access: who may sign in (sign-ups, D-055) and which GitHub accounts TerraFlow
 * is connected to (D-049), on one screen, because the questions are the same: who can do
 * what, and through whose account. */

export type UserStatus = "pending" | "active" | "rejected" | "suspended";
export interface UserRow {
  oid: string;
  email: string;
  name: string;
  project_count: number;
  last_login_at?: string;
  provider?: string;
  login?: string;
  status?: UserStatus;
  status_reason?: string;
}

const STATUS_ORDER: Record<UserStatus, number> = { pending: 0, active: 1, suspended: 2, rejected: 3 };
const USER_STATUS: Record<UserStatus, { label: string; tone: "warn" | "good" | "bad" | "neutral" }> = {
  pending: { label: "Waiting", tone: "warn" },
  active: { label: "Active", tone: "good" },
  rejected: { label: "Rejected", tone: "bad" },
  suspended: { label: "Suspended", tone: "neutral" },
};

/** Sign-ups waiting for an admin first; then everyone else by name. */
export function sortUsers(users: UserRow[]): UserRow[] {
  return [...users].sort(
    (a, b) =>
      STATUS_ORDER[a.status ?? "active"] - STATUS_ORDER[b.status ?? "active"] ||
      (a.name || a.email).localeCompare(b.name || b.email),
  );
}

type Section = "users" | "connections";
type UserFilter = "all" | UserStatus;

const CONN_STATUS: Record<ConnectionRow["status"], { label: string; tone: "good" | "warn" | "neutral" }> = {
  active: { label: "Connected", tone: "good" },
  needs_attention: { label: "Needs attention", tone: "warn" },
  disconnected: { label: "Disconnected", tone: "neutral" },
};

export function PeoplePanel({ onDenied, onHelp, onWaiting }: { onDenied: () => void; onHelp: () => void; onWaiting: (n: number) => void }) {
  const [section, setSection] = useState<Section>("users");
  const [users, setUsers] = useState<UserRow[] | null>(null);
  const [me, setMe] = useState("");
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);

  const loadUsers = useCallback(async () => {
    const r = await call<UserRow[]>("/admin/users");
    if (r.status === 403) return onDenied();
    if (!r.ok) setMsg({ tone: "error", text: r.error });
    const list = r.data ?? [];
    setUsers(list);
    onWaiting(list.filter((u) => u.status === "pending").length);
  }, [onDenied, onWaiting]);

  useEffect(() => {
    loadUsers();
    call<{ oid: string }>("/me").then((r) => setMe(r.data?.oid ?? ""));
  }, [loadUsers]);

  const waiting = (users ?? []).filter((u) => u.status === "pending").length;

  return (
    <div>
      <PanelHeader
        title="People & access"
        intro="Who may sign in and what they are allowed to do, and which GitHub accounts TerraFlow works through."
        actions={<HelpButton onClick={onHelp} label="Docs" />}
      />
      {msg && <Notice tone={msg.tone} onClose={() => setMsg(null)}>{msg.text}</Notice>}
      <div className="mb-5 inline-flex rounded-lg border border-[var(--tf-color-border-strong)] p-0.5 text-sm" role="tablist">
        {(["users", "connections"] as Section[]).map((s) => (
          <button
            key={s}
            role="tab"
            aria-selected={section === s}
            onClick={() => setSection(s)}
            className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 font-medium ${section === s ? "bg-[var(--tf-glass-strong)]" : "text-[var(--tf-color-text-secondary)]"}`}
          >
            {s === "users" ? <Users className="size-4" /> : <Github className="size-4" />}
            {s === "users" ? "People" : "GitHub connections"}
            {s === "users" && waiting > 0 && <Chip tone="warn">{waiting}</Chip>}
          </button>
        ))}
      </div>
      {section === "users" ? (
        <PeopleList users={users} me={me} reload={loadUsers} setMsg={setMsg} />
      ) : (
        <Connections users={users ?? []} onDenied={onDenied} />
      )}
    </div>
  );
}

function PeopleList({ users, me, reload, setMsg }: { users: UserRow[] | null; me: string; reload: () => Promise<void>; setMsg: (m: { tone: "error" | "ok"; text: string } | null) => void }) {
  const [filter, setFilter] = useState<UserFilter>("all");
  const [q, setQ] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  const list = users ?? [];
  const count = (s: UserStatus) => list.filter((u) => (u.status ?? "active") === s).length;
  const shown = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return sortUsers(list).filter(
      (u) => (filter === "all" || (u.status ?? "active") === filter) && (!needle || `${u.name} ${u.email} ${u.login ?? ""}`.toLowerCase().includes(needle)),
    );
  }, [list, filter, q]);

  // Default to what needs doing: if people are waiting, show them first.
  useEffect(() => {
    if (users && users.some((u) => u.status === "pending")) setFilter((f) => (f === "all" ? "pending" : f));
    // only when the list first arrives
  }, [users === null]);

  const change = async (u: UserRow, status: UserStatus, why = "") => {
    setBusy(u.oid);
    setMsg(null);
    const r = await call(`/admin/users/${encodeURIComponent(u.oid)}/status`, { method: "POST", json: { status, reason: why } });
    setBusy(null);
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setRejecting(null);
    setReason("");
    setMsg({ tone: "ok", text: `${u.name || u.email}: ${USER_STATUS[status].label.toLowerCase()}.` });
    await reload();
  };

  if (users === null) return <PanelLoading rows={5} />;
  if (list.length === 0) return <EmptyState icon={<Users className="size-5" />} title="Nobody has signed in yet">People appear here after their first sign-in.</EmptyState>;

  const btn = "tf-btn tf-btn--ghost";
  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <Pills
          value={filter}
          onChange={setFilter}
          items={[
            { id: "all", label: "All", count: list.length },
            { id: "pending", label: "Waiting", count: count("pending") },
            { id: "active", label: "Active", count: count("active") },
            { id: "suspended", label: "Suspended", count: count("suspended") },
            { id: "rejected", label: "Rejected", count: count("rejected") },
          ]}
        />
        <div className="relative w-full max-w-xs">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search name, e-mail, login" aria-label="Search people" className={`${inputCls} mt-0 pl-9`} />
        </div>
      </div>
      <ul className="m-0 list-none space-y-2 p-0">
        {shown.length === 0 && <li className="py-6 text-center text-sm text-[var(--tf-color-text-muted)]">{filter === "pending" ? "Nobody is waiting for approval." : "No one matches."}</li>}
        {shown.map((u) => {
          const st = u.status ?? "active";
          const self = u.oid === me;
          return (
            <li key={u.oid}>
              <Card className="flex flex-wrap items-center justify-between gap-3 !py-3">
                <div className="min-w-0 flex-1">
                  <p className="m-0 flex flex-wrap items-center gap-2 text-sm font-semibold">
                    {u.name || u.email || u.oid}
                    <Chip tone={USER_STATUS[st].tone}>{USER_STATUS[st].label}</Chip>
                    {self && <Chip tone="info">you</Chip>}
                  </p>
                  <p className="m-0 mt-0.5 truncate text-xs text-[var(--tf-color-text-muted)]">
                    {[u.login ? `@${u.login}` : "", u.email, u.provider, `${u.project_count} ${u.project_count === 1 ? "project" : "projects"}`, u.last_login_at ? `last seen ${new Date(u.last_login_at).toLocaleDateString()}` : ""].filter(Boolean).join(" · ")}
                  </p>
                  {u.status_reason && <p className="m-0 mt-0.5 text-xs text-[var(--tf-color-text-secondary)]">Reason: {u.status_reason}</p>}
                </div>
                {!self && (
                  <div className="flex flex-wrap items-center justify-end gap-1.5">
                    {rejecting === u.oid ? (
                      <>
                        <input aria-label="Reason" autoFocus maxLength={200} placeholder="Reason (optional)" value={reason} onChange={(e) => setReason(e.target.value)} className={`${inputCls} mt-0 w-52 py-1.5 text-xs`} />
                        <button className="tf-btn" disabled={busy === u.oid} onClick={() => change(u, "rejected", reason)}>Reject</button>
                        <button className={btn} onClick={() => setRejecting(null)}>Cancel</button>
                      </>
                    ) : (
                      <>
                        {st === "pending" && (
                          <>
                            <button className="tf-btn" disabled={busy === u.oid} onClick={() => change(u, "active")}>
                              <UserCheck className="size-4" /> Approve
                            </button>
                            <button className={btn} onClick={() => setRejecting(u.oid)}>Reject</button>
                          </>
                        )}
                        {st === "active" && <button className={btn} disabled={busy === u.oid} onClick={() => change(u, "suspended")}>Suspend</button>}
                        {(st === "suspended" || st === "rejected") && <button className={btn} disabled={busy === u.oid} onClick={() => change(u, "active")}>Reactivate</button>}
                      </>
                    )}
                  </div>
                )}
              </Card>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function Connections({ users, onDenied }: { users: UserRow[]; onDenied: () => void }) {
  const [rows, setRows] = useState<ConnectionRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<string | null>(null);
  const nameOf = (oid: string) => users.find((u) => u.oid === oid)?.name || oid;

  const load = useCallback(async () => {
    try {
      setRows(await getConnections());
      setError(null);
    } catch (e) {
      const m = e instanceof Error ? e.message : "Cannot load connections";
      if (/admin role/i.test(m)) onDenied();
      else setError(m);
    }
  }, [onDenied]);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (id: string, fn: (id: string) => Promise<unknown>) => {
    setBusy(id);
    setError(null);
    try {
      await fn(id);
      setConfirm(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setBusy(null);
    }
  };

  if (!rows && !error) return <p className="flex items-center gap-2 text-sm text-[var(--tf-color-text-secondary)]"><Loader2 className="size-4 animate-spin" /> Loading connections…</p>;

  return (
    <div>
      <p className="mb-4 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">
        A connection is one installation of the TerraFlow GitHub App on a GitHub account or organisation. It holds no password or token.
      </p>
      {error && <Notice tone="error" onClose={() => setError(null)}>{error}</Notice>}
      {rows && rows.length === 0 ? (
        <EmptyState icon={<Github className="size-5" />} title="No GitHub connections yet">
          A connection appears when someone installs the TerraFlow GitHub App on their account and authorizes it from New project.
        </EmptyState>
      ) : (
        <ul className="m-0 list-none space-y-2 p-0">
          {rows?.map((c) => (
            <li key={c.id}>
              <Card className="flex flex-wrap items-center justify-between gap-3 !py-3">
                <div className="min-w-0">
                  <p className="m-0 flex flex-wrap items-center gap-2 text-sm font-semibold">
                    <Github className="size-4" /> {c.account_login}
                    <span className="text-xs font-normal text-[var(--tf-color-text-muted)]">{c.account_type.toLowerCase()}</span>
                    <Chip tone={CONN_STATUS[c.status].tone}>{CONN_STATUS[c.status].label}</Chip>
                  </p>
                  <p className="m-0 mt-0.5 flex items-center gap-1 text-xs text-[var(--tf-color-text-muted)]">
                    <ShieldCheck className="size-3" /> connected by {nameOf(c.created_by)} · {c.projects} {c.projects === 1 ? "project" : "projects"} use it
                  </p>
                </div>
                <div className="flex flex-wrap justify-end gap-2">
                  <button className="tf-btn tf-btn--ghost" disabled={busy === c.id} onClick={() => act(c.id, recheckConnection)}>
                    {busy === c.id ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />} Check
                  </button>
                  {c.status !== "disconnected" &&
                    (confirm === c.id ? (
                      <>
                        <button className="tf-btn" style={{ background: "var(--tf-color-error)", color: "var(--tf-color-surface)" }} disabled={busy === c.id} onClick={() => act(c.id, disconnectConnection)}>
                          Stop {c.projects} project{c.projects === 1 ? "" : "s"} and disconnect
                        </button>
                        <button className="tf-btn tf-btn--ghost" onClick={() => setConfirm(null)}>Cancel</button>
                      </>
                    ) : (
                      <button className="tf-btn tf-btn--ghost" onClick={() => setConfirm(c.id)}>
                        <Unplug className="size-4" /> Disconnect
                      </button>
                    ))}
                </div>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
