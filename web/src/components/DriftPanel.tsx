import { useState } from "react";
import { ChevronDown, ChevronRight, GitCompareArrows } from "lucide-react";
import { KIND_LABEL, useDrift, type DriftItem } from "../hooks/useDrift";

interface Props {
  projectId: string;
  env: string;
  /** Owners and co-owners dismiss items and accept the current state. */
  canManage: boolean;
  /** Open a resource that is on the canvas as a repository resource. */
  onOpenNode: (nodeId: string) => void;
}

/**
 * What the repository holds that TerraFlow does not show or did not write (8.14): blocks the canvas
 * cannot show exactly, values that differ from the project variables, and files changed on the base
 * branch since the last approval. It never asks Azure.
 */
export function DriftPanel({ projectId, env, canManage, onOpenNode }: Props) {
  const { drift, error, dismiss, restore, acceptBaseline } = useDrift(projectId, env);
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState<{ key: string; text: string } | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const run = (fn: () => Promise<void>) => fn().then(() => setProblem(null), (e) => setProblem(e instanceof Error ? e.message : "Something went wrong"));

  if (error) return <p role="alert" className="m-3 text-xs text-[var(--tf-color-error)]">{error}</p>;
  if (!drift) return null;
  const count = drift.items.length;

  return (
    <section aria-label="Drift" className="border-b border-[var(--tf-color-border)]">
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-semibold">
        {open ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
        <GitCompareArrows className="size-4" />
        <span className="flex-1">Drift</span>
        <span className={count > 0 ? "tf-warn-badge" : "tf-draft-badge"}>{count > 0 ? `${count} to look at` : "No drift found"}</span>
      </button>
      {open && (
        <div className="px-3 pb-3 text-xs">
          <p className="m-0 text-[var(--tf-color-text-muted)]">
            The repository against what TerraFlow knows, in {env}. Nothing here asks Azure.
          </p>
          {!drift.has_baseline && (
            <p className="m-0 mt-2">
              No baseline yet: changes to files are measured from the last approval
              {canManage ? (
                <>
                  , or{" "}
                  <button type="button" className="underline" onClick={() => run(acceptBaseline)}>
                    accept the repository as it is now
                  </button>
                  .
                </>
              ) : (
                "."
              )}
            </p>
          )}
          {drift.has_baseline && canManage && count > 0 && (
            <p className="m-0 mt-2">
              <button type="button" className="underline" onClick={() => run(acceptBaseline)}>
                Accept the base branch as it is now
              </button>{" "}
              (file changes stop being listed).
            </p>
          )}
          {problem && <p role="alert" className="m-0 mt-2 text-[var(--tf-color-error)]">{problem}</p>}
          <ul className="m-0 mt-2 list-none space-y-2 p-0">
            {drift.items.map((it) => (
              <Item key={it.key} it={it} canManage={canManage} onOpenNode={onOpenNode} editing={note?.key === it.key}
                onDismissStart={() => setNote({ key: it.key, text: "" })}>
                {note?.key === it.key && (
                  <form className="mt-2 flex gap-2" onSubmit={(e) => { e.preventDefault(); run(() => dismiss(it.key, note.text)).then(() => setNote(null)); }}>
                    <input autoFocus aria-label="Why it is fine" maxLength={500} placeholder="Why it is fine (kept for the project)" value={note.text}
                      onChange={(e) => setNote({ key: it.key, text: e.target.value })}
                      className="h-8 min-w-0 flex-1 rounded-md border border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] px-2" />
                    <button type="submit" className="tf-btn">Dismiss</button>
                    <button type="button" className="tf-btn tf-btn--ghost" onClick={() => setNote(null)}>Cancel</button>
                  </form>
                )}
              </Item>
            ))}
            {count === 0 && <li className="text-[var(--tf-color-text-muted)]">Nothing to look at.</li>}
          </ul>
          {drift.dismissed.length > 0 && (
            <details className="mt-3">
              <summary className="cursor-pointer font-medium">Dismissed ({drift.dismissed.length})</summary>
              <ul className="m-0 mt-2 list-none space-y-2 p-0">
                {drift.dismissed.map((d) => (
                  <li key={d.key} className="rounded-md bg-[var(--tf-color-surface-subtle)] p-2">
                    <div>{d.message}</div>
                    <div className="mt-1 text-[var(--tf-color-text-muted)]">“{d.note || "no note"}”</div>
                    {canManage && (
                      <button type="button" className="mt-1 underline" onClick={() => run(() => restore(d.key))}>
                        Show again
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            </details>
          )}
        </div>
      )}
    </section>
  );
}

function Item({ it, canManage, onOpenNode, onDismissStart, editing, children }: {
  it: DriftItem; canManage: boolean; onOpenNode: (id: string) => void; onDismissStart: () => void; editing: boolean; children?: React.ReactNode;
}) {
  return (
    <li className="rounded-md border border-[var(--tf-color-border)] p-2">
      <div className="font-semibold">{KIND_LABEL[it.kind]}</div>
      <div className="mt-0.5">{it.message}</div>
      <div className="mt-1 flex gap-3">
        {it.importable && it.node_id && (
          <button type="button" className="underline" onClick={() => onOpenNode(it.node_id!)}>
            Open on the canvas
          </button>
        )}
        {canManage && !editing && (
          <button type="button" className="underline" onClick={onDismissStart}>
            Dismiss with a note
          </button>
        )}
      </div>
      {children}
    </li>
  );
}
