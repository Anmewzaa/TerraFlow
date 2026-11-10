import { useState } from "react";
import { Loader2, Trash2, UserPlus } from "lucide-react";
import { ROLE_LABEL, useTeam, type Role } from "../hooks/useGitHub";
import type { User } from "../types/auth";

const field =
  "rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]";

interface Props {
  projectId: string;
  user?: User | null;
  /** Called after the project was deleted by its owner. */
  onDeleted: () => void;
  /** Shown inside the project page: no scrolling or padding of its own. */
  embedded?: boolean;
}

/** The project's people: members and roles, invitations, the second-reviewer switch,
 *  ownership transfer and deleting the project. What a person may do follows their role. */
export function TeamView({ projectId, user, onDeleted, embedded }: Props) {
  const t = useTeam(projectId);
  const [who, setWho] = useState("");
  const [role, setRole] = useState<Role>("contributor");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ tone: "ok" | "error"; text: string } | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const isAdmin = !!user?.roles?.includes("devops");
  const myRole = t.team?.my_role ?? "";
  // An admin who is not a member manages as an owner (their access is audited).
  const manage = isAdmin || myRole === "owner" || myRole === "co_owner";
  const isOwner = isAdmin || myRole === "owner";
  const onGitHub = !!t.team?.repository;

  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    setBusy(true);
    setMessage(null);
    try {
      await fn();
      if (ok) setMessage({ tone: "ok", text: ok });
    } catch (e) {
      setMessage({ tone: "error", text: e instanceof Error ? e.message : "Something went wrong" });
    } finally {
      setBusy(false);
    }
  };

  if (t.error) {
    return <p role="alert" className="p-6 text-sm text-[var(--tf-color-error)]">{t.error}</p>;
  }
  if (!t.team) {
    return (
      <p className="flex items-center gap-2 p-6 text-sm text-[var(--tf-color-text-secondary)]">
        <Loader2 className="size-4 animate-spin" /> Loading the team…
      </p>
    );
  }

  return (
    <section aria-label="Team" className={embedded ? "w-full" : "mx-auto w-full max-w-3xl overflow-auto p-6"}>
      <h2 className="text-xl font-semibold">Team</h2>
      <p className="mt-1 text-sm text-[var(--tf-color-text-secondary)]">
        {onGitHub ? (
          <>
            Repository <code>{t.team.repository}</code>
            {t.team.folder ? <> · folder <code>{t.team.folder}</code></> : null}. People you add can open branches on
            it through TerraFlow.
          </>
        ) : (
          "This project is a local folder."
        )}{" "}
        Your role: <strong>{myRole ? ROLE_LABEL[myRole] : isAdmin ? "Admin (support access)" : "—"}</strong>.
      </p>

      {message && (
        <p role="status" className={`mt-4 rounded-lg px-3 py-2 text-sm ${message.tone === "ok" ? "bg-[var(--tf-color-surface-subtle)]" : "bg-[var(--tf-color-error-surface)] text-[var(--tf-color-error)]"}`}>
          {message.text}
        </p>
      )}

      <table className="mt-6 w-full text-left text-sm">
        <thead className="text-xs uppercase text-[var(--tf-color-text-muted)]">
          <tr>
            <th className="py-2 pr-3">Person</th>
            <th className="py-2 pr-3">Role</th>
            <th className="py-2" />
          </tr>
        </thead>
        <tbody>
          {t.team.members.map((m) => {
            const editable = manage && m.role !== "owner";
            return (
              <tr key={m.user_oid} className="border-t border-[var(--tf-color-border)]">
                <td className="py-2 pr-3">
                  <div className="font-medium">{m.name || m.email}</div>
                  <div className="text-xs text-[var(--tf-color-text-muted)]">{m.email}</div>
                </td>
                <td className="py-2 pr-3">
                  {editable ? (
                    <select
                      aria-label={`Role of ${m.name || m.email}`}
                      className={field}
                      value={m.role}
                      disabled={busy}
                      onChange={(e) => run(() => t.setRole(m.user_oid, e.target.value as Role))}
                    >
                      <option value="co_owner">{ROLE_LABEL.co_owner}</option>
                      <option value="contributor">{ROLE_LABEL.contributor}</option>
                    </select>
                  ) : (
                    ROLE_LABEL[m.role]
                  )}
                </td>
                <td className="py-2 text-right">
                  {editable && (
                    <button
                      className="rounded-md p-1.5 text-[var(--tf-color-text-muted)] hover:bg-[var(--tf-color-error-surface)] hover:text-[var(--tf-color-error)]"
                      aria-label={`Remove ${m.name || m.email}`}
                      disabled={busy}
                      onClick={() => run(() => t.remove(m.user_oid))}
                    >
                      <Trash2 className="size-4" />
                    </button>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>

      {manage && (
        <form
          className="mt-6 flex flex-wrap items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            run(async () => {
              await t.invite(who.trim(), role);
              setWho("");
            }, "Added.");
          }}
        >
          <div className="min-w-56 flex-1">
            <label htmlFor="invite-who" className="mb-1 block text-sm text-[var(--tf-color-text-secondary)]">
              Add a person by email
            </label>
            <input id="invite-who" className={`${field} w-full`} value={who} onChange={(e) => setWho(e.target.value)} placeholder="name@company.com" required />
          </div>
          <select aria-label="Role for the new person" className={field} value={role} onChange={(e) => setRole(e.target.value as Role)}>
            <option value="contributor">{ROLE_LABEL.contributor}</option>
            <option value="co_owner">{ROLE_LABEL.co_owner}</option>
          </select>
          <button className="tf-btn" disabled={busy || !who.trim()}>
            <UserPlus className="size-4" /> Add
          </button>
          <p className="w-full text-xs text-[var(--tf-color-text-muted)]">
            They must have signed in to TerraFlow once. Contributors build and publish; co-owners also approve and manage the team.
          </p>
        </form>
      )}

      {onGitHub && manage && (
        <label className="mt-8 flex items-start gap-3 text-sm">
          <input
            type="checkbox"
            className="mt-1"
            checked={t.team.second_reviewer}
            disabled={busy}
            onChange={(e) => run(() => t.setSecondReviewer(e.target.checked))}
          />
          <span>
            <strong>Require a second reviewer.</strong> Whoever published a change cannot approve it, even the owner.
          </span>
        </label>
      )}

      {isOwner && t.team.members.some((m) => m.role !== "owner") && (
        <div className="mt-8">
          <h3 className="text-sm font-semibold">Transfer ownership</h3>
          <form
            className="mt-2 flex flex-wrap items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const to = new FormData(e.currentTarget).get("to");
              if (typeof to === "string" && to) run(() => t.transfer(to), "Ownership transferred. You are now a co-owner.");
            }}
          >
            <select name="to" aria-label="New owner" className={field} defaultValue="">
              <option value="" disabled>Choose a member…</option>
              {t.team.members.filter((m) => m.role !== "owner").map((m) => (
                <option key={m.user_oid} value={m.user_oid}>{m.name || m.email}</option>
              ))}
            </select>
            <button className="tf-btn tf-btn--ghost" disabled={busy}>Transfer</button>
          </form>
        </div>
      )}

      {isOwner && onGitHub && (
        <div className="mt-8">
          <h3 className="text-sm font-semibold">Delete project</h3>
          <p className="mt-1 text-xs text-[var(--tf-color-text-muted)]">
            Removes the project, its drafts and changesets from TerraFlow. The repository and its branches are not touched.
          </p>
          {confirmDelete ? (
            <div className="mt-2 flex gap-2">
              <button className="tf-btn tf-btn--ghost" onClick={() => setConfirmDelete(false)}>Cancel</button>
              <button
                className="rounded-lg bg-[var(--tf-color-error)] px-3 py-2 text-sm font-semibold text-[var(--tf-color-surface)]"
                disabled={busy}
                onClick={() => run(async () => { await t.deleteProject(); onDeleted(); })}
              >
                Delete for good
              </button>
            </div>
          ) : (
            <button className="tf-btn tf-btn--ghost mt-2" onClick={() => setConfirmDelete(true)}>Delete this project…</button>
          )}
        </div>
      )}
    </section>
  );
}
