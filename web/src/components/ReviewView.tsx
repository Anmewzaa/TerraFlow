import { useState } from "react";
import {
  AlertTriangle,
  Check,
  FileCode2,
  GitBranch,
  Loader2,
  ShieldAlert,
  Inbox,
  X,
} from "lucide-react";
import { CodeBlock } from "@/components/ui/code-block";
import { StatusBadge } from "./StatusBadge";
import { decide, useChangesetList, useReviewDetail } from "../hooks/useReview";
import type { User } from "../types/auth";

interface Props {
  projectId: string;
  user: User | null;
  /** Bumped by the parent after any decision so other views refresh. */
  onDecided: () => void;
  refreshKey: number;
}

function fmt(iso?: string | null) {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

export function ReviewView({ projectId, user, onDecided, refreshKey }: Props) {
  const { rows, loading, error } = useChangesetList(projectId, refreshKey);
  const [selected, setSelected] = useState<string | null>(null);
  const [localKey, setLocalKey] = useState(0);
  const [busy, setBusy] = useState<"approve" | "reject" | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const [result, setResult] = useState<{ branch?: string; repo?: string } | null>(null);

  const queue = rows.filter((r) => r.status !== "draft");
  const current = selected ?? queue.find((r) => r.status === "published")?.id ?? queue[0]?.id ?? null;
  const { detail, loading: detailLoading, error: detailError } = useReviewDetail(
    projectId,
    current,
    refreshKey + localKey,
  );

  const canReview = !!user?.roles.includes("devops");
  const cs = detail?.changeset;
  const pending = cs?.status === "published";
  const blocked = (detail?.conflicts.length ?? 0) > 0;

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

  return (
    <div className="flex min-h-0 flex-1 overflow-hidden bg-[var(--tf-color-canvas)]">
      {/* queue */}
      <aside className="flex w-[300px] shrink-0 flex-col border-r border-[var(--tf-color-border)] bg-[var(--tf-color-surface)]">
        <div className="border-b border-[var(--tf-color-border)] p-4">
          <h2 className="text-sm font-semibold">Review queue</h2>
          <p className="text-xs text-[var(--tf-color-text-muted)]">
            {queue.filter((r) => r.status === "published").length} waiting for review
          </p>
        </div>
        <div className="flex-1 space-y-2 overflow-y-auto p-3">
          {loading && <Loader2 className="mx-auto mt-6 size-5 animate-spin text-[var(--tf-color-text-muted)]" />}
          {error && <p className="text-sm text-[var(--tf-color-error)]">{error}</p>}
          {!loading && queue.length === 0 && (
            <div className="flex flex-col items-center gap-2 py-12 text-center text-sm text-[var(--tf-color-text-muted)]">
              <Inbox className="size-8" />
              Nothing to review yet. Publish a changeset from the canvas.
            </div>
          )}
          {queue.map((r) => (
            <button
              key={r.id}
              onClick={() => {
                setSelected(r.id);
                setResult(null);
                setActionError(null);
                setRejecting(false);
              }}
              className={`w-full cursor-pointer rounded-xl border p-3 text-left transition ${
                current === r.id
                  ? "border-[var(--tf-color-action)] bg-[var(--tf-color-surface-selected)]"
                  : "border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] hover:border-[var(--tf-color-border-strong)]"
              }`}
            >
              <div className="mb-1.5 flex items-center justify-between gap-2">
                <span className="truncate text-sm font-medium">{r.title || "Untitled changeset"}</span>
                <StatusBadge status={r.status} />
              </div>
              <div className="text-xs text-[var(--tf-color-text-muted)]">
                {r.env} · {fmt(r.updated_at)}
              </div>
            </button>
          ))}
        </div>
      </aside>

      {/* detail */}
      <section className="min-w-0 flex-1 overflow-y-auto p-6">
        {!current && !loading && (
          <p className="text-sm text-[var(--tf-color-text-muted)]">Select a changeset to review.</p>
        )}
        {detailLoading && !detail && <Loader2 className="size-5 animate-spin text-[var(--tf-color-text-muted)]" />}
        {detailError && (
          <div className="rounded-lg border border-[var(--tf-color-error)] bg-[var(--tf-color-error-surface)] px-4 py-3 text-sm text-[var(--tf-color-error)]">
            {detailError}
          </div>
        )}

        {detail && cs && (
          <div className="mx-auto max-w-4xl space-y-6">
            <header className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <div className="mb-1 flex items-center gap-2">
                  <h2 className="text-xl font-semibold">{cs.title || "Untitled changeset"}</h2>
                  <StatusBadge status={cs.status} />
                </div>
                <p className="text-sm text-[var(--tf-color-text-muted)]">
                  {cs.env} · requested by <span className="font-mono">{cs.user_oid}</span> · {fmt(cs.updated_at)}
                </p>
              </div>
              {pending && (
                <div className="flex gap-2">
                  <button
                    disabled={!canReview || !!busy || blocked}
                    onClick={() => setRejecting(true)}
                    className="tf-btn tf-btn--ghost"
                    style={{ opacity: !canReview || blocked ? 0.5 : 1 }}
                  >
                    <X className="size-4" /> Reject
                  </button>
                  <button
                    disabled={!canReview || !!busy || blocked}
                    onClick={() => run("approve")}
                    className="tf-btn"
                    style={{ opacity: !canReview || blocked ? 0.5 : 1 }}
                  >
                    {busy === "approve" ? <Loader2 className="size-4 animate-spin" /> : <Check className="size-4" />}
                    Approve &amp; create branch
                  </button>
                </div>
              )}
            </header>

            {pending && !canReview && (
              <Note tone="warn" icon={<ShieldAlert className="size-4" />}>
                Only the <strong>devops</strong> role can approve or reject. For local development run the
                server with <code>TF_DEV_USER_ROLE=devops</code>.
              </Note>
            )}
            {blocked && (
              <Note tone="bad" icon={<AlertTriangle className="size-4" />}>
                These generated files already exist and would be overwritten:{" "}
                {detail.conflicts.map((c) => (
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
            {(cs.status === "approved" || result) && (
              <Note tone="good" icon={<GitBranch className="size-4" />}>
                Branch <code>{result?.branch ?? cs.branch_name ?? detail.branch}</code> created
                {cs.commit_sha ? (
                  <>
                    {" "}at commit <code>{cs.commit_sha.slice(0, 8)}</code>
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
              <div className="rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] p-4">
                <label className="mb-1 block text-sm font-semibold" htmlFor="reject-reason">
                  Reason for rejection <span style={{ color: "var(--tf-color-error)" }}>*</span>
                </label>
                <textarea
                  id="reject-reason"
                  autoFocus
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  rows={3}
                  placeholder="Tell the developer what to change"
                  className="tf-input"
                  style={{ padding: "var(--tf-space-2) var(--tf-space-3)", resize: "vertical" }}
                />
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

            <Card title={`Canvas changes (${detail.canvas_diff.added_nodes.length} added)`}>
              {detail.canvas_diff.added_nodes.length === 0 ? (
                <p className="text-sm text-[var(--tf-color-text-muted)]">No modules in this changeset.</p>
              ) : (
                <ul className="m-0 list-none space-y-2 p-0">
                  {detail.canvas_diff.added_nodes.map((n) => (
                    <li
                      key={n.id}
                      className="flex items-start gap-3 rounded-lg border border-[var(--tf-color-border)] p-3"
                    >
                      <span className="mt-0.5 rounded bg-[var(--tf-color-success-surface)] px-1.5 text-xs font-bold text-[var(--tf-color-success)]">
                        +
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="text-sm font-medium">
                          {n.instance_name}{" "}
                          <span className="font-normal text-[var(--tf-color-text-muted)]">
                            {n.module_name}@{n.module_version} → {n.target_folder}/
                          </span>
                        </div>
                        <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                          {Object.entries(n.values)
                            .filter(([k]) => !k.startsWith("_"))
                            .map(([k, v]) => (
                              <div key={k} className="contents">
                                <dt className="text-[var(--tf-color-text-muted)]">{k}</dt>
                                <dd className="truncate font-mono">{typeof v === "string" ? v : JSON.stringify(v)}</dd>
                              </div>
                            ))}
                        </dl>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </Card>

            <Card
              title="Terraform changes (new files)"
              icon={<FileCode2 className="size-4" />}
            >
              {detail.hcl_diff.length === 0 ? (
                <p className="text-sm text-[var(--tf-color-text-muted)]">No files would be generated.</p>
              ) : (
                <>
                  <div className="mb-3 flex flex-wrap gap-2 text-xs">
                    {detail.hcl_diff.map((f) => (
                      <span key={f.path} className="tf-ident">
                        {f.status === "conflict" ? "!" : f.status === "modified" ? "~" : "+"} {f.path} ({f.lines_added} new lines)
                      </span>
                    ))}
                  </div>
                  <CodeBlock
                    language="hcl"
                    className="h-[460px]"
                    tabs={detail.hcl_diff.map((f) => ({
                      name: f.path,
                      code: f.content,
                      added: true,
                    }))}
                  />
                  <p className="mt-3 text-xs text-[var(--tf-color-text-muted)]">
                    Branch to be created: <code>{detail.branch}</code>
                  </p>
                </>
              )}
            </Card>
          </div>
        )}
      </section>
    </div>
  );
}

function Card({ title, icon, children }: { title: string; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-5 shadow-sm">
      <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold">
        {icon}
        {title}
      </h3>
      {children}
    </div>
  );
}

function Note({
  tone,
  icon,
  children,
}: {
  tone: "good" | "bad" | "warn";
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  const c = { good: "success", bad: "error", warn: "warning" }[tone];
  return (
    <div
      className="flex items-start gap-2 rounded-lg px-4 py-3 text-sm"
      style={{ background: `var(--tf-color-${c}-surface)`, color: `var(--tf-color-${c})` }}
    >
      <span className="mt-0.5 shrink-0">{icon}</span>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
