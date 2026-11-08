import { useState } from "react";
import { FolderGit2, Plus, Trash2, ArrowRight, Layers, Loader2, X } from "lucide-react";
import { DarkGradientBg } from "@/components/ui/elegant-dark-pattern";
import type { Project } from "../types/project";

interface Props {
  projects: Project[];
  loading: boolean;
  error: string | null;
  userName?: string;
  onOpen: (id: string) => void;
  onCreate: (name: string, iacPath?: string) => Promise<Project>;
  onDelete: (id: string) => Promise<void>;
}

function formatDate(iso?: string) {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? ""
    : d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function HomePage({ projects, loading, error, userName, onOpen, onCreate, onDelete }: Props) {
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmId, setConfirmId] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setFormError(null);
    try {
      const p = await onCreate(name.trim(), path.trim() || undefined);
      setCreating(false);
      setName("");
      setPath("");
      onOpen(p.id);
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Could not create project");
    } finally {
      setBusy(false);
    }
  };

  const doDelete = async (id: string) => {
    try {
      await onDelete(id);
    } catch (err) {
      alert(err instanceof Error ? err.message : "Could not delete project");
    } finally {
      setConfirmId(null);
    }
  };

  return (
    <DarkGradientBg className="min-h-screen text-[var(--tf-color-text)]">
      <div className="mx-auto max-w-6xl px-6 py-12">
        <header className="mb-10 flex flex-wrap items-end justify-between gap-4">
          <div>
            <div className="mb-3 flex items-center gap-2 text-[var(--tf-color-action)]">
              <Layers className="size-6" />
              <span className="text-sm font-semibold uppercase tracking-widest">TerraFlow</span>
            </div>
            <h1 className="text-4xl font-bold tracking-tight">
              {userName ? `Welcome back, ${userName}` : "Your projects"}
            </h1>
            <p className="mt-2 max-w-xl text-[var(--tf-color-text-secondary)]">
              Compose Azure infrastructure from approved modules, review it, and ship a Terraform branch.
            </p>
          </div>
          <button
            onClick={() => setCreating(true)}
            className="flex items-center gap-2 rounded-lg bg-[var(--tf-color-action)] px-4 py-2.5 text-sm font-semibold text-[var(--tf-color-on-action)] shadow-lg shadow-cyan-500/20 transition hover:bg-[var(--tf-color-action-hover)]"
          >
            <Plus className="size-4" /> New project
          </button>
        </header>

        {error && (
          <div className="mb-6 rounded-lg border border-[var(--tf-color-error)] bg-[var(--tf-color-error-surface)] px-4 py-3 text-sm text-[var(--tf-color-error)]">
            Could not load projects: {error}
          </div>
        )}

        {loading ? (
          <div className="flex items-center gap-2 text-[var(--tf-color-text-secondary)]">
            <Loader2 className="size-4 animate-spin" /> Loading projects…
          </div>
        ) : projects.length === 0 && !error ? (
          <div className="rounded-2xl border border-dashed border-[var(--tf-color-border-strong)] bg-[var(--tf-glass)] p-14 text-center">
            <FolderGit2 className="mx-auto mb-4 size-10 text-[var(--tf-color-text-muted)]" />
            <h2 className="text-xl font-semibold">No projects yet</h2>
            <p className="mx-auto mb-6 mt-1 max-w-sm text-sm text-[var(--tf-color-text-muted)]">
              Create your first project to start designing infrastructure on the canvas.
            </p>
            <button
              onClick={() => setCreating(true)}
              className="rounded-lg bg-[var(--tf-color-action)] px-4 py-2 text-sm font-semibold text-[var(--tf-color-on-action)] hover:bg-[var(--tf-color-action-hover)]"
            >
              Create a project
            </button>
          </div>
        ) : (
          <ul className="m-0 grid list-none gap-5 p-0 sm:grid-cols-2 lg:grid-cols-3">
            {projects.map((p) => (
              <li
                key={p.id}
                className="group relative flex flex-col rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-glass)] p-5 backdrop-blur transition hover:-translate-y-0.5 hover:border-[var(--tf-color-action)] hover:bg-[var(--tf-glass-hover)]"
              >
                <div className="mb-4 flex items-start justify-between">
                  <div className="flex size-10 items-center justify-center rounded-xl bg-[var(--tf-glass-strong)] text-[var(--tf-color-action)]">
                    <FolderGit2 className="size-5" />
                  </div>
                  <button
                    onClick={() => setConfirmId(p.id)}
                    aria-label={`Delete ${p.name}`}
                    className="rounded-md p-1.5 text-[var(--tf-color-text-muted)] opacity-0 transition hover:bg-[var(--tf-color-error-surface)] hover:text-[var(--tf-color-error)] focus:opacity-100 group-hover:opacity-100"
                  >
                    <Trash2 className="size-4" />
                  </button>
                </div>
                <h3 className="truncate text-lg font-semibold">{p.name}</h3>
                <p className="mt-1 truncate font-mono text-xs text-[var(--tf-color-text-muted)]" title={p.iac_path}>
                  {p.iac_path}
                </p>
                <div className="mt-4 flex items-center gap-2 text-xs text-[var(--tf-color-text-muted)]">
                  <span className="rounded-full bg-[var(--tf-glass-strong)] px-2 py-0.5">{p.source_type}</span>
                  {p.created_at && <span>Created {formatDate(p.created_at)}</span>}
                </div>
                <button
                  onClick={() => onOpen(p.id)}
                  className="mt-5 flex items-center justify-center gap-2 rounded-lg bg-[var(--tf-glass-strong)] px-3 py-2 text-sm font-medium transition hover:bg-[var(--tf-color-action)] hover:text-[var(--tf-color-on-action)]"
                >
                  Open canvas <ArrowRight className="size-4" />
                </button>

                {confirmId === p.id && (
                  <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 rounded-2xl bg-[var(--tf-color-surface)]/95 p-5 text-center">
                    <p className="text-sm">
                      Delete <strong>{p.name}</strong>? Drafts and changesets are removed; files on disk are kept.
                    </p>
                    <div className="flex gap-2">
                      <button
                        onClick={() => setConfirmId(null)}
                        className="rounded-md bg-[var(--tf-glass-strong)] px-3 py-1.5 text-sm hover:bg-[var(--tf-color-border)]"
                      >
                        Cancel
                      </button>
                      <button
                        onClick={() => doDelete(p.id)}
                        className="rounded-md bg-[var(--tf-color-error)] px-3 py-1.5 text-sm font-semibold text-[var(--tf-color-surface)] hover:opacity-90"
                      >
                        Delete
                      </button>
                    </div>
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      {creating && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--tf-color-overlay)] p-4 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          aria-label="New project"
        >
          <form
            onSubmit={submit}
            className="w-full max-w-md rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-6 shadow-2xl"
          >
            <div className="mb-5 flex items-center justify-between">
              <h2 className="text-xl font-semibold">New project</h2>
              <button
                type="button"
                onClick={() => setCreating(false)}
                aria-label="Close"
                className="text-[var(--tf-color-text-muted)] hover:text-[var(--tf-color-text)]"
              >
                <X className="size-5" />
              </button>
            </div>
            <label className="mb-1 block text-sm text-[var(--tf-color-text-secondary)]" htmlFor="pname">
              Project name
            </label>
            <input
              id="pname"
              autoFocus
              required
              maxLength={80}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Payments platform"
              className="mb-4 w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]"
            />
            <label className="mb-1 block text-sm text-[var(--tf-color-text-secondary)]" htmlFor="ppath">
              IaC folder <span className="text-[var(--tf-color-text-muted)]">(optional)</span>
            </label>
            <input
              id="ppath"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="Leave empty to create a new folder in the workspace"
              className="w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 font-mono text-xs outline-none focus:border-[var(--tf-color-action)]"
            />
            <p className="mt-1 text-xs text-[var(--tf-color-text-muted)]">An existing folder must be inside the server workspace.</p>
            {formError && <p className="mt-3 text-sm text-[var(--tf-color-error)]">{formError}</p>}
            <div className="mt-6 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setCreating(false)}
                className="rounded-lg px-4 py-2 text-sm text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-strong)]"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={busy || !name.trim()}
                className="flex items-center gap-2 rounded-lg bg-[var(--tf-color-action)] px-4 py-2 text-sm font-semibold text-[var(--tf-color-on-action)] hover:bg-[var(--tf-color-action-hover)] disabled:opacity-50"
              >
                {busy && <Loader2 className="size-4 animate-spin" />} Create
              </button>
            </div>
          </form>
        </div>
      )}
    </DarkGradientBg>
  );
}
