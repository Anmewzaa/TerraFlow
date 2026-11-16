import { useMemo, useState, type ReactNode } from "react";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  FileCode2,
  GitBranch,
  Inbox,
  Info,
  Loader2,
  Pencil,
  Plus,
  Search,
  Send,
  ShieldAlert,
  X,
} from "lucide-react";
import { diffWithRepo } from "@/lib/diff";
import { DiffFiles } from "./DiffFiles";
import { PanelLoading } from "@/components/ui/skeleton";
import { StatusBadge } from "./StatusBadge";
import { canApproveRole, useTeam } from "../hooks/useGitHub";
import { decide, useChangesetList, useReviewDetail, type ChangesetRow } from "../hooks/useReview";
import type { User } from "../types/auth";

function fmt(iso?: string | null) {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

type Filter = "all" | "saved" | "published" | "approved" | "rejected";
const FILTERS: { id: Filter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "saved", label: "Saved" },
  { id: "published", label: "In review" },
  { id: "approved", label: "Approved" },
  { id: "rejected", label: "Rejected" },
];

const glass = "bg-[color-mix(in_srgb,var(--tf-color-surface)_90%,transparent)] backdrop-blur-sm";

/**
 * Changesets and their review in one page. The list is every version saved from the canvas with where it stands;
 * picking one shows what it changes and, for the people who may decide, the buttons that approve it (which
 * creates the branch) or reject it. The app still stops at the branch.
 */
export function ChangesetsView({
  projectId,
  user,
  refreshKey,
  onDecided,
  onPublish,
  onRevise,
}: {
  projectId: string;
  user: User | null;
  refreshKey: number;
  /** Bumped by the parent after any decision so other views refresh. */
  onDecided: () => void;
  /** Opens the publish dialog for a saved version. */
  onPublish: (row: ChangesetRow) => void;
  /** Opens the changeset as a new draft on the canvas. */
  onRevise: (csId: string, env: string) => Promise<void>;
}) {
  const { rows, loading, error } = useChangesetList(projectId, refreshKey);
  const [filter, setFilter] = useState<Filter>("all");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [localKey, setLocalKey] = useState(0);
  const [busy, setBusy] = useState<"approve" | "reject" | "revise" | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const [result, setResult] = useState<{ branch?: string; repo?: string } | null>(null);

  const all = rows.filter((r) => r.status !== "draft");
  const counts = useMemo(() => {
    const c: Record<Filter, number> = { all: all.length, saved: 0, published: 0, approved: 0, rejected: 0 };
    for (const r of all) if (r.status in c) c[r.status as Filter] += 1;
    return c;
  }, [all]);
  const q = query.trim().toLowerCase();
  const list = all.filter((r) => (filter === "all" || r.status === filter) && (!q || `${r.title} ${r.env}`.toLowerCase().includes(q)));
  // What the person most likely came for: the change waiting for a decision, else the newest.
  const current = selected && all.some((r) => r.id === selected) ? selected : (all.find((r) => r.status === "published") ?? all[0])?.id ?? null;
  const row = all.find((r) => r.id === current);
  const { detail, loading: detailLoading, error: detailError } = useReviewDetail(projectId, row && row.status !== "saved" ? current : null, refreshKey + localKey);

  // The global devops role, or (D-050) the project's owner and co-owners.
  const { team } = useTeam(projectId);
  const canReview = !!user?.roles.includes("devops") || canApproveRole(team?.my_role);
  // A saved version is not in review yet, so the review endpoint has nothing for it: the row is what is known.
  const cs = detail?.changeset ?? row;
  const pending = cs?.status === "published";
  const blocked = (detail?.conflicts.length ?? 0) > 0;
  const savedNodes = row?.status === "saved" ? (row.canvas?.nodes ?? []) : [];
  const changeFiles = useMemo(
    () =>
      diffWithRepo(
        [],
        [],
        (detail?.hcl_diff ?? []).map((f) => {
          const at = f.path.lastIndexOf("/");
          return { folder: at < 0 ? "" : f.path.slice(0, at), filename: at < 0 ? f.path : f.path.slice(at + 1), content: f.content };
        }),
      ),
    [detail],
  );
  const resourcesShown: ResourceInfo[] = detail ? detail.canvas_diff.added_nodes : savedNodes;

  const pick = (id: string) => {
    setSelected(id);
    setResult(null);
    setActionError(null);
    setRejecting(false);
  };

  const run = async (action: "approve" | "reject") => {
    if (!cs) return;
    setBusy(action);
    setActionError(null);
    try {
      const res = await decide(projectId, cs.id, action, reason.trim());
      setResult(action === "approve" ? res : null);
      setRejecting(false);
      setReason("");
      setLocalKey((k) => k + 1);
      onDecided();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy(null);
    }
  };

  const revise = async () => {
    if (!row) return;
    setBusy("revise");
    setActionError(null);
    try {
      await onRevise(row.id, row.env);
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "Could not open the changeset");
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden md:flex-row">
      {/* the list */}
      <aside className={`flex max-h-[42%] w-full shrink-0 flex-col border-b border-[var(--tf-color-border)] md:max-h-none md:w-[330px] md:border-b-0 md:border-r ${glass}`} aria-label="Changesets">
        <div className="space-y-3 border-b border-[var(--tf-color-border)] p-4">
          <div>
            <h2 className="m-0 text-base font-semibold">Changesets</h2>
            <p className="m-0 mt-0.5 text-xs text-[var(--tf-color-text-muted)]">
              {counts.published > 0 ? `${counts.published} waiting for review` : "Versions you saved, and where each stands"}
            </p>
          </div>
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" aria-hidden />
            <input aria-label="Search changesets" placeholder="Search" className="tf-input" style={{ paddingLeft: 32, minHeight: 34 }} value={query} onChange={(e) => setQuery(e.target.value)} />
          </div>
          <div className="flex flex-wrap gap-1" role="group" aria-label="Filter by status">
            {FILTERS.map((f) => (
              <button
                key={f.id}
                type="button"
                aria-pressed={filter === f.id}
                onClick={() => setFilter(f.id)}
                className="rounded-full border px-2 py-0.5 text-xs transition"
                style={{
                  borderColor: filter === f.id ? "var(--tf-color-action)" : "var(--tf-color-border)",
                  background: filter === f.id ? "color-mix(in srgb, var(--tf-color-action) 12%, transparent)" : "transparent",
                  color: filter === f.id ? "var(--tf-color-text)" : "var(--tf-color-text-secondary)",
                  fontWeight: filter === f.id ? 600 : 400,
                }}
              >
                {f.label} <span className="tabular-nums opacity-70">{counts[f.id]}</span>
              </button>
            ))}
          </div>
        </div>
        <div className="flex-1 space-y-2 overflow-y-auto p-3">
          {loading && all.length === 0 && <PanelLoading rows={4} label="Loading changesets" />}
          {error && <p className="text-sm text-[var(--tf-color-error)]">{error}</p>}
          {!loading && !error && all.length === 0 && (
            <div className="flex flex-col items-center gap-2 px-4 py-12 text-center text-sm text-[var(--tf-color-text-muted)]">
              <Inbox className="size-8" aria-hidden />
              No changesets yet. Drag modules onto the canvas and save a version.
            </div>
          )}
          {all.length > 0 && list.length === 0 && <p className="px-2 py-6 text-center text-sm text-[var(--tf-color-text-muted)]">Nothing matches.</p>}
          {list.map((r) => (
            <button
              key={r.id}
              type="button"
              onClick={() => pick(r.id)}
              aria-current={current === r.id ? "true" : undefined}
              className={`tf-enter w-full cursor-pointer rounded-xl border p-3 text-left transition hover:-translate-y-px hover:shadow-md ${
                current === r.id ? "border-[var(--tf-color-action)] bg-[var(--tf-color-surface-selected)]" : "border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] hover:border-[var(--tf-color-border-strong)]"
              }`}
            >
              <div className="mb-1.5 flex items-center justify-between gap-2">
                <span className="truncate text-sm font-medium">{r.title || "Untitled changeset"}</span>
                <StatusBadge status={r.status} />
              </div>
              <div className="flex items-center justify-between gap-2 text-xs text-[var(--tf-color-text-muted)]">
                <span className="rounded bg-[var(--tf-color-surface-subtle)] px-1.5 py-px font-mono">{r.env}</span>
                <span>{fmt(r.updated_at)}</span>
              </div>
              {r.status === "approved" && r.branch_name && (
                <div className="mt-2 flex items-center gap-1.5 truncate text-xs text-[var(--tf-color-success)]">
                  <GitBranch className="size-3.5 shrink-0" aria-hidden />
                  <code className="truncate">{r.branch_name}</code>
                </div>
              )}
            </button>
          ))}
        </div>
      </aside>

      {/* the selected changeset */}
      <section className="min-w-0 flex-1 overflow-y-auto p-4 md:p-6" aria-label="Changeset details">
        {!current && !loading && all.length === 0 && (
          <div className={`mx-auto mt-10 max-w-xl rounded-2xl border border-[var(--tf-color-border)] p-8 text-center shadow-sm ${glass}`}>
            <span className="mx-auto grid size-12 place-items-center rounded-xl bg-[var(--tf-color-info-surface)] text-[var(--tf-color-info)]">
              <GitBranch className="size-6" aria-hidden />
            </span>
            <h2 className="m-0 mt-4 text-lg font-semibold">From canvas to branch</h2>
            <p className="mx-auto mb-5 mt-1 max-w-md text-sm text-[var(--tf-color-text-secondary)]">
              A change goes through three steps here. TerraFlow stops at the branch; the pull request is yours.
            </p>
            <ol className="m-0 grid list-none gap-3 p-0 text-left text-sm">
              {["Drag modules onto the canvas and save a version.", "Send it for review. An owner or co-owner reads the diff.", "Approve it and TerraFlow opens a branch with the Terraform."].map((t, i) => (
                <li key={t} className="flex items-center gap-3 rounded-xl border border-[var(--tf-color-border)] px-4 py-3">
                  <span className="grid size-6 shrink-0 place-items-center rounded-full bg-[var(--tf-color-action)] text-xs font-semibold text-[var(--tf-color-on-action)]">{i + 1}</span>
                  {t}
                </li>
              ))}
            </ol>
          </div>
        )}
        {!current && !loading && all.length > 0 && <p className="text-sm text-[var(--tf-color-text-muted)]">Select a changeset.</p>}
        {detailLoading && !detail && current && <PanelLoading rows={3} label="Loading changeset" className="mx-auto max-w-4xl" />}
        {detailError && (
          <div role="alert" className="mx-auto mb-4 max-w-4xl rounded-lg border border-[var(--tf-color-error)] bg-[var(--tf-color-error-surface)] px-4 py-3 text-sm text-[var(--tf-color-error)]">
            {detailError}
          </div>
        )}

        {cs && row && (
          <div className="mx-auto max-w-4xl space-y-5">
            <header className={`rounded-2xl border border-[var(--tf-color-border)] p-5 shadow-sm ${glass}`}>
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div className="min-w-0">
                  <div className="mb-1 flex flex-wrap items-center gap-2">
                    <h2 className="m-0 text-xl font-semibold">{cs.title || "Untitled changeset"}</h2>
                    <StatusBadge status={cs.status} />
                  </div>
                  <p className="m-0 text-sm text-[var(--tf-color-text-muted)]">
                    {cs.env} · requested by <span className="font-mono">{cs.user_oid}</span> · {fmt(cs.updated_at)}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {row.status === "saved" && (
                    <button className="tf-btn" onClick={() => onPublish(row)} title="Send this version for review">
                      <Send className="size-4" aria-hidden /> Send for review
                    </button>
                  )}
                  {row.status !== "saved" && (
                    <button className="tf-btn tf-btn--ghost" disabled={busy === "revise"} onClick={revise} title="Open as a new draft to add, remove or change resources">
                      {busy === "revise" ? <Loader2 className="size-4 animate-spin" /> : <Pencil className="size-4" aria-hidden />} Edit
                    </button>
                  )}
                  {pending && (
                    <>
                      <button disabled={!canReview || !!busy || blocked} onClick={() => setRejecting(true)} className="tf-btn tf-btn--ghost" style={{ opacity: !canReview || blocked ? 0.5 : 1 }}>
                        <X className="size-4" aria-hidden /> Reject
                      </button>
                      <button disabled={!canReview || !!busy || blocked} onClick={() => run("approve")} className="tf-btn" style={{ opacity: !canReview || blocked ? 0.5 : 1 }}>
                        {busy === "approve" ? <Loader2 className="size-4 animate-spin" /> : <Check className="size-4" aria-hidden />}
                        Approve &amp; create branch
                      </button>
                    </>
                  )}
                </div>
              </div>
              <Steps status={cs.status} />
            </header>

            <NextStep status={cs.status} env={cs.env} count={detail ? detail.canvas_diff.added_nodes.length : savedNodes.length} files={detail?.hcl_diff.length ?? 0} branch={result?.branch ?? cs.branch_name ?? detail?.branch} canReview={canReview} />

            {pending && !canReview && (
              <Note tone="warn" icon={<ShieldAlert className="size-4" />}>
                Only the project's <strong>owner and co-owners</strong> (or the <strong>devops</strong> role) can approve or reject. Ask one of them to review this change.
              </Note>
            )}
            {detail && blocked && (
              <Note tone="bad" icon={<AlertTriangle className="size-4" />}>
                These generated files already exist and would be overwritten:{" "}
                {detail?.conflicts.map((c) => (
                  <code key={c} className="mr-1">
                    {c}
                  </code>
                ))}
              </Note>
            )}
            {actionError && (
              <Note tone="bad" icon={<AlertTriangle className="size-4" />}>
                {actionError}
              </Note>
            )}
            {detail && (cs.status === "approved" || result) && (
              <Note tone="good" icon={<GitBranch className="size-4" />}>
                Branch <code>{result?.branch ?? cs.branch_name ?? detail.branch}</code> created
                {cs.commit_sha ? (
                  <>
                    {" "}
                    at commit <code>{cs.commit_sha.slice(0, 8)}</code>
                  </>
                ) : null}
                . Open the pull request in your Git host; TerraFlow stops at the branch.
              </Note>
            )}
            {cs.status === "rejected" && (
              <Note tone="bad" icon={<X className="size-4" />}>
                Rejected{cs.reviewed_at ? ` on ${fmt(cs.reviewed_at)}` : ""}: {cs.reject_reason}
              </Note>
            )}

            {rejecting && (
              <div className={`rounded-xl border border-[var(--tf-color-border)] p-4 ${glass}`}>
                <label className="mb-1 block text-sm font-semibold" htmlFor="reject-reason">
                  Reason for rejection <span style={{ color: "var(--tf-color-error)" }}>*</span>
                </label>
                <textarea id="reject-reason" autoFocus value={reason} onChange={(e) => setReason(e.target.value)} rows={3} placeholder="Tell the developer what to change" className="tf-input" style={{ padding: "var(--tf-space-2) var(--tf-space-3)", resize: "vertical" }} />
                <div className="mt-3 flex justify-end gap-2">
                  <button className="tf-btn tf-btn--ghost" onClick={() => setRejecting(false)}>
                    Cancel
                  </button>
                  <button className="tf-btn" disabled={!reason.trim() || !!busy} onClick={() => run("reject")}>
                    {busy === "reject" && <Loader2 className="size-4 animate-spin" />} Confirm rejection
                  </button>
                </div>
              </div>
            )}

            <Card title={`What this adds: ${resourcesShown.length} ${resourcesShown.length === 1 ? "resource" : "resources"}`} icon={<Plus className="size-4 text-[var(--tf-color-success)]" aria-hidden />}>
              <p className="mb-3 mt-0 text-xs text-[var(--tf-color-text-muted)]">
                Each line becomes a module call in your repository. Nothing already there is changed or removed.
              </p>
              {resourcesShown.length === 0 ? (
                <p className="m-0 text-sm text-[var(--tf-color-text-muted)]">Nothing in this version.</p>
              ) : (
                <ul className="m-0 list-none space-y-2 p-0">
                  {resourcesShown.map((n) => (
                    <ResourceLine key={n.id} n={n} />
                  ))}
                </ul>
              )}
            </Card>

            {detail && (
              <Card title={`Files TerraFlow will write: ${detail.hcl_diff.length}`} icon={<FileCode2 className="size-4" />}>
                <p className="mb-3 mt-0 text-xs text-[var(--tf-color-text-muted)]">This is the Terraform that goes on the new branch. It is the same view as the HCL preview.</p>
                <div className="-mx-5 -mb-5 flex h-[520px] flex-col overflow-hidden rounded-b-2xl border-t border-[var(--tf-color-border)]">
                  <DiffFiles shown={changeFiles} emptyText="No files would be generated." />
                </div>
                <p className="mb-0 mt-5 text-xs text-[var(--tf-color-text-muted)]">
                  Branch to be created: <code>{detail.branch}</code>
                </p>
              </Card>
            )}
          </div>
        )}
      </section>
    </div>
  );
}

/** Where a changeset is on its way, each step with what it means. */
function Steps({ status }: { status: string }) {
  const rejected = status === "rejected";
  const at = status === "saved" ? 0 : status === "published" ? 1 : 2;
  const steps = [
    { label: "Saved", hint: "Kept in TerraFlow only" },
    { label: "In review", hint: "An owner reads it" },
    { label: rejected ? "Rejected" : "Approved", hint: rejected ? "Sent back with a reason" : "Branch is created" },
  ];
  return (
    <ol className="m-0 mt-5 grid list-none grid-cols-3 gap-2 p-0" aria-label="Progress">
      {steps.map((st, i) => {
        const done = i < at || (i === at && at === 2);
        const now = i === at && at < 2;
        const bad = rejected && i === 2;
        const tone = bad ? "var(--tf-color-error)" : done ? "var(--tf-color-success)" : now ? "var(--tf-color-action)" : "var(--tf-color-text-muted)";
        return (
          <li key={st.label} aria-current={i === at ? "step" : undefined} className="rounded-xl border px-3 py-2" style={{ borderColor: i === at ? tone : "var(--tf-color-border)", background: i === at ? `color-mix(in srgb, ${tone} 8%, transparent)` : "transparent" }}>
            <div className="flex items-center gap-2">
              <span className="grid size-5 shrink-0 place-items-center rounded-full border text-[10px] font-semibold" style={{ borderColor: tone, color: done || bad ? "var(--tf-color-canvas)" : tone, background: done || bad ? tone : "transparent" }}>
                {bad ? <X className="size-3" aria-hidden /> : done ? <CheckCircle2 className="size-3" aria-hidden /> : i + 1}
              </span>
              <span className="text-xs font-semibold" style={{ color: i <= at ? "var(--tf-color-text)" : "var(--tf-color-text-muted)" }}>{st.label}</span>
            </div>
            <p className="m-0 mt-1 hidden text-[11px] text-[var(--tf-color-text-muted)] sm:block">{st.hint}</p>
          </li>
        );
      })}
    </ol>
  );
}

/** The one thing to know about this change right now, in plain words. */
function NextStep({ status, env, count, files, branch, canReview }: { status: string; env: string; count: number; files: number; branch?: string | null; canReview: boolean }) {
  const n = `${count} ${count === 1 ? "resource" : "resources"}`;
  const text: Record<string, { title: string; body: ReactNode }> = {
    saved: {
      title: "This version is saved, not sent yet",
      body: <>It adds {n} to <strong>{env}</strong>. Nothing is in your repository. Press <strong>Send for review</strong> so an owner or co-owner can check it.</>,
    },
    published: {
      title: canReview ? "It is waiting for your decision" : "It is waiting for an owner or co-owner",
      body: <>Reading it is safe. <strong>Approve</strong> makes a new branch{branch ? <> (<code>{branch}</code>)</> : null} with {files > 0 ? `${files} generated ${files === 1 ? "file" : "files"}` : "the generated files"}. <strong>Reject</strong> sends it back with your reason. Neither changes your main branch.</>,
    },
    approved: {
      title: "Approved: the branch was created",
      body: <>The Terraform for these {n} is on the branch{branch ? <> <code>{branch}</code></> : null}. Open the pull request in your Git host. TerraFlow does not plan or apply.</>,
    },
    rejected: {
      title: "Rejected: it was sent back",
      body: <>Press <strong>Edit</strong> to open it on the canvas, change what the reviewer asked for, save and send it again.</>,
    },
  };
  const t = text[status];
  if (!t) return null;
  return (
    <div className={`flex gap-3 rounded-2xl border border-[var(--tf-color-info)]/30 bg-[var(--tf-color-info-surface)] p-4 text-sm`}>
      <Info className="mt-0.5 size-4 shrink-0 text-[var(--tf-color-info)]" aria-hidden />
      <div className="min-w-0">
        <p className="m-0 font-semibold text-[var(--tf-color-info)]">{t.title}</p>
        <p className="m-0 mt-0.5 text-[var(--tf-color-text-secondary)]">{t.body}</p>
      </div>
    </div>
  );
}

interface ResourceInfo {
  id: string;
  instance_name: string;
  kind?: string;
  resource_type?: string;
  module_name: string;
  module_version?: string;
  target_folder: string;
  values: Record<string, unknown>;
}

/** One resource: what it is, where it goes, and its first settings; the rest behind a button. */
function ResourceLine({ n }: { n: ResourceInfo }) {
  const [all, setAll] = useState(false);
  const entries = Object.entries(n.values ?? {}).filter(([k]) => !k.startsWith("_"));
  const shown = all ? entries : entries.slice(0, 3);
  const what = n.kind === "resource" ? `${n.resource_type} (raw resource)` : `${n.module_name}${n.module_version ? ` ${n.module_version}` : ""}`;
  return (
    <li className="rounded-xl border border-[var(--tf-color-border)] p-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="grid size-5 place-items-center rounded bg-[var(--tf-color-success-surface)] text-[var(--tf-color-success)]" aria-label="added"><Plus className="size-3.5" aria-hidden /></span>
        <span className="text-sm font-semibold">{n.instance_name}</span>
        <span className="text-xs text-[var(--tf-color-text-muted)]">{what}</span>
        <span className="ml-auto rounded bg-[var(--tf-color-surface-subtle)] px-1.5 py-px font-mono text-[11px] text-[var(--tf-color-text-secondary)]">into {n.target_folder}/</span>
      </div>
      {entries.length > 0 && (
        <dl className="m-0 mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 pl-8 text-xs">
          {shown.map(([k, v]) => (
            <div key={k} className="contents">
              <dt className="text-[var(--tf-color-text-muted)]">{k}</dt>
              <dd className="m-0 truncate font-mono">{typeof v === "string" ? v : JSON.stringify(v)}</dd>
            </div>
          ))}
        </dl>
      )}
      {entries.length > 3 && (
        <button type="button" onClick={() => setAll(!all)} className="ml-8 mt-1.5 text-xs text-[var(--tf-color-action)] hover:underline">
          {all ? "Show fewer settings" : `Show all ${entries.length} settings`}
        </button>
      )}
    </li>
  );
}

function Card({ title, icon, children }: { title: string; icon?: ReactNode; children: ReactNode }) {
  return (
    <div className={`rounded-2xl border border-[var(--tf-color-border)] p-5 shadow-sm ${glass}`}>
      <h3 className="mb-3 mt-0 flex items-center gap-2 text-sm font-semibold">
        {icon}
        {title}
      </h3>
      {children}
    </div>
  );
}

function Note({ tone, icon, children }: { tone: "good" | "bad" | "warn"; icon: ReactNode; children: ReactNode }) {
  const c = { good: "success", bad: "error", warn: "warning" }[tone];
  return (
    <div role={tone === "bad" ? "alert" : "status"} className="flex items-start gap-2 rounded-lg px-4 py-3 text-sm" style={{ background: `var(--tf-color-${c}-surface)`, color: `var(--tf-color-${c})` }}>
      <span className="mt-0.5 shrink-0">{icon}</span>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
