import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { errorOf, useTeam } from "../hooks/useGitHub";

interface Branches {
  branches: string[];
  truncated: boolean;
  default_branch: string;
  base_branch: string;
}

const field =
  "rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]";

/**
 * The branch this project reads from and starts its review branches from (8.8). It is only read:
 * Approve still makes a new branch and never commits to this one. Owner and co-owner only.
 */
export function BaseBranchPanel({ projectId, canManage }: { projectId: string; canManage: boolean }) {
  const t = useTeam(projectId);
  const [list, setList] = useState<Branches | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pick, setPick] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const onGitHub = !!t.team?.repository;

  useEffect(() => {
    if (!canManage || !onGitHub) return;
    let cancelled = false;
    fetch(`/api/projects/${encodeURIComponent(projectId)}/branches`)
      .then(async (r) => {
        if (!r.ok) throw new Error(await errorOf(r));
        return (await r.json()) as Branches;
      })
      .then((b) => {
        if (cancelled) return;
        setList(b);
        setPick(b.base_branch || b.default_branch);
        setError(null);
      })
      .catch((e) => !cancelled && setError(e instanceof Error ? e.message : "Cannot list the branches"));
    return () => {
      cancelled = true;
    };
  }, [projectId, canManage, onGitHub]);

  if (!onGitHub) return null;

  const save = async (branch: string) => {
    setBusy(true);
    setSaved(false);
    try {
      const r = await fetch(`/api/projects/${encodeURIComponent(projectId)}/base-branch`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ branch }),
      });
      if (!r.ok) throw new Error(await errorOf(r));
      setError(null);
      setSaved(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-label="Branch" id="branch" className="w-full">
      <h2 className="text-xl font-semibold">Branch</h2>
      <p className="mt-1 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">
        TerraFlow reads the project from this branch and starts every review branch from it. It never commits to it:
        Approve still opens a new <code>terraflow/…</code> branch for you to review in GitHub. Changes planned on an
        earlier branch keep the commit they were made on.
      </p>
      {t.team?.base_branch_missing && (
        <p role="alert" className="mt-3 rounded-md border border-[var(--tf-color-warning)] px-3 py-2 text-sm text-[var(--tf-color-warning)]">
          The branch <code>{t.team.base_branch}</code> is no longer in the repository, so this project cannot be read or branched until a
          branch is chosen again. Nothing falls back to the default on its own.
        </p>
      )}
      {!!t.team?.pending_on_old_base && (
        <p className="mt-3 text-sm">
          {t.team.pending_on_old_base} open {t.team.pending_on_old_base === 1 ? "change was" : "changes were"} made before the base branch was changed.
          They are planned again on the current branch when they are reviewed.
        </p>
      )}
      {!canManage ? (
        <p className="mt-3 text-sm">
          Working branch: <code>{t.team?.base_branch || t.team?.default_branch}</code>
          {t.team?.base_branch ? "" : " (the repository default)"}. The owner and co-owners can change it.
        </p>
      ) : (
        <div className="mt-4 flex flex-wrap items-center gap-2">
          {!list && !error && <Loader2 className="size-4 animate-spin" aria-label="Loading the branches" />}
          {list && (
            <>
              <label className="sr-only" htmlFor="base-branch">
                Base branch
              </label>
              <select id="base-branch" className={field} value={pick} onChange={(e) => { setPick(e.target.value); setSaved(false); }}>
                {[...new Set([list.default_branch, ...list.branches, pick])].map((b) => (
                  <option key={b} value={b}>
                    {b}
                    {b === list.default_branch ? " (repository default)" : ""}
                  </option>
                ))}
              </select>
              <button
                disabled={busy || pick === (list.base_branch || list.default_branch)}
                onClick={() => save(pick)}
                className="rounded-lg bg-[var(--tf-color-action)] px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
              >
                Save
              </button>
              {list.truncated && <span className="text-xs text-[var(--tf-color-text-muted)]">Only the first {list.branches.length} branches are listed.</span>}
            </>
          )}
        </div>
      )}
      {saved && <p role="status" className="mt-2 text-sm">Saved. The next read of the repository uses this branch.</p>}
      {error && <p role="alert" className="mt-2 text-sm text-[var(--tf-color-error)]">{error}</p>}
    </section>
  );
}
