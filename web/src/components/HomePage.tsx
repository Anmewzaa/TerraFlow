import { notifyError, notifySuccess } from "../lib/notify";
import { ProjectPreview } from "./ProjectPreview";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, BookOpen, ArrowRight, FolderGit2, Github, History, Loader2, Plus, Search, Settings2, Trash2, X } from "lucide-react";
import { CanvasGlyphs } from "./CanvasGlyphs";
import type { Project } from "../types/project";
import type { User } from "../types/auth";
import { Logo } from "./Logo";
import { AccountMenu } from "./AccountMenu";
import { AdminView } from "./AdminView";
import { AuditView } from "./AuditView";
import { GitHubProjectDialog } from "./GitHubProjectDialog";
import { GitHubSettings } from "./GitHubSettings";
import { ReleaseNotes } from "./ReleaseNotes";
import { APP_VERSION } from "../lib/releases";
import { AUTHORIZE_URL, clearGitHubReturn, getGitHubStatus, getRepos, peekGitHubReturn } from "../hooks/useGitHub";

/** How many projects the home page lists before "View all projects". */
export const HOME_PROJECT_LIMIT = 6;

interface Props {
  projects: Project[];
  loading: boolean;
  error: string | null;
  user?: User | null;
  /** The local sign-in is on, so "Switch user" replaces "Logout". */
  devSignIn?: boolean;
  onProjectsChanged?: () => void;
  onOpen: (id: string) => void;
  onCreate: (name: string, iacPath?: string) => Promise<Project>;
  onDelete: (id: string) => Promise<void>;
}

type Page = "home" | "all" | "admin" | "audit" | "github" | "releases";

function formatDate(iso?: string) {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? ""
    : d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

const glass =
  "rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-glass)] shadow-[var(--tf-shadow-rest)] backdrop-blur transition hover:-translate-y-0.5 hover:shadow-[var(--tf-shadow-lift)] hover:border-[var(--tf-color-action)] hover:bg-[var(--tf-glass-hover)]";

export function HomePage({ projects, loading, error, user, devSignIn, onProjectsChanged, onOpen, onCreate, onDelete }: Props) {
  const isAdmin = !!user?.roles?.includes("devops");
  const [page, setPage] = useState<Page>("home");
  const [search, setSearch] = useState("");
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmId, setConfirmId] = useState<string | null>(null);
  // Coming back from GitHub lands on #/projects/new?connected=1 (or ?error=code), which
  // opens the GitHub dialog again. Read once, before App rewrites the hash.
  // How this server creates projects (TF_PROJECT_SOURCE): one way only, never a choice.
  // A server that does not answer is treated as local (no GitHub, no database).
  const [projectSource, setProjectSource] = useState<"github" | "local" | null>(null);
  useEffect(() => {
    let off = false;
    getGitHubStatus()
      .then((s) => !off && setProjectSource(s.project_source === "github" ? "github" : "local"))
      .catch(() => !off && setProjectSource("local"));
    return () => {
      off = true;
    };
  }, []);
  // New project: nobody connected yet (or the repository list expired) goes straight to
  // GitHub to connect; otherwise the dialog opens with the repositories.
  const [starting, setStarting] = useState(false);
  const startNewProject = async () => {
    if (projectSource === "local") return setCreating(true);
    setStarting(true);
    try {
      if ((await getRepos()).length === 0) {
        window.location.href = AUTHORIZE_URL;
        return;
      }
    } catch {
      /* the dialog shows the error */
    }
    setStarting(false);
    setFromGitHub(new URLSearchParams());
  };
  const [fromGitHub, setFromGitHub] = useState<URLSearchParams | null>(() => peekGitHubReturn());
  useEffect(() => clearGitHubReturn, []);

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
      notifySuccess("Project deleted.");
    } catch (err) {
      notifyError(err instanceof Error ? err.message : "Could not delete project");
    } finally {
      setConfirmId(null);
    }
  };

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return q ? projects.filter((p) => `${p.name} ${p.iac_path}`.toLowerCase().includes(q)) : projects;
  }, [projects, search]);

  const card = (p: Project) => (
    <li key={p.id} className={`group relative flex flex-col p-5 ${glass}`}>
      <div className="mb-4 flex items-start justify-between">
        <div className="flex size-10 items-center justify-center rounded-xl bg-[var(--tf-glass-strong)] text-[var(--tf-color-action)]">
          <FolderGit2 className="size-5" />
        </div>
        {isAdmin && (
          <button
            onClick={() => setConfirmId(p.id)}
            aria-label={`Delete ${p.name}`}
            className="rounded-md p-1.5 text-[var(--tf-color-text-muted)] opacity-0 transition hover:bg-[var(--tf-color-error-surface)] hover:text-[var(--tf-color-error)] focus:opacity-100 group-hover:opacity-100"
          >
            <Trash2 className="size-4" />
          </button>
        )}
      </div>
      <h3 className="truncate text-lg font-semibold">{p.name}</h3>
      <p className="mt-1 truncate font-mono text-xs text-[var(--tf-color-text-muted)]" title={p.iac_path}>
        {p.iac_path}
      </p>
      <div className="mt-4"><ProjectPreview projectId={p.id} /></div>
      <div className="flex items-center gap-2 text-xs text-[var(--tf-color-text-muted)]">
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
  );

  const back = (label: string, to: Page) => (
    <button
      onClick={() => { setPage(to); setSearch(""); }}
      className="mb-5 flex items-center gap-1.5 text-sm text-[var(--tf-color-text-secondary)] hover:text-[var(--tf-color-text)]"
    >
      <ArrowLeft className="size-4" /> {label}
    </button>
  );

  const shown = projects.slice(0, HOME_PROJECT_LIMIT);
  const navItems: { label: string; icon: typeof FolderGit2; pages: Page[] }[] = [
    { label: "Projects", icon: FolderGit2, pages: ["home", "all"] },
    ...(projectSource === "github" ? [{ label: "GitHub", icon: Github, pages: ["github" as Page] }] : []),
    ...(isAdmin
      ? [
          { label: "Admin", icon: Settings2, pages: ["admin" as Page] },
          { label: "Audit log", icon: History, pages: ["audit" as Page] },
        ]
      : []),
    { label: "Release notes", icon: BookOpen, pages: ["releases" as Page] },
  ];

  return (
    <div className="h-screen bg-[var(--tf-color-canvas)] text-[var(--tf-color-text)]">
      <CanvasGlyphs enabled soft>
        <div className="relative z-10 flex h-full flex-col md:flex-row">
          <aside className="flex shrink-0 flex-col gap-3 border-b border-[var(--tf-color-border)] bg-[var(--tf-glass)] p-3 backdrop-blur md:w-60 md:border-b-0 md:border-r md:p-5">
            <div className="flex items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
              <button type="button" onClick={() => setPage("home")} aria-label="TerraFlow, back to projects" className="flex items-center gap-2 text-[var(--tf-color-action-active)]">
                <Logo size={30} />
                <span className="text-lg font-bold tracking-tight text-[var(--tf-color-text)]">TerraFlow</span>
              </button>
              <button
                type="button"
                onClick={() => setPage("releases")}
                title="What is new in this version"
                aria-label={`Version ${APP_VERSION}: open the release notes`}
                className="text-xs font-medium text-[var(--tf-color-action)] hover:underline"
              >
                {APP_VERSION}
              </button>
              </div>
            </div>
            <nav aria-label="Main" className="flex gap-1 overflow-x-auto md:mt-4 md:flex-col md:overflow-visible">
              {navItems.map((n) => {
                const active = n.pages.includes(page);
                return (
                  <button
                    key={n.label}
                    type="button"
                    aria-current={active ? "page" : undefined}
                    onClick={() => { setPage(n.pages[0]!); setSearch(""); }}
                    className={`flex items-center gap-2.5 whitespace-nowrap rounded-lg px-3 py-2 text-left text-sm font-medium transition ${active ? "bg-[var(--tf-color-info-surface)] text-[var(--tf-color-action)]" : "text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-strong)] hover:text-[var(--tf-color-text)]"}`}
                  >
                    <n.icon className="size-4 shrink-0" aria-hidden /> {n.label}
                  </button>
                );
              })}
            </nav>
            <div className="hidden flex-1 md:block" />
            {user && (
              <div className="border-t border-[var(--tf-color-border)] pt-3">
                <AccountMenu user={user} dev={!!devSignIn} />
              </div>
            )}
          </aside>

          <main className="min-w-0 flex-1 overflow-y-auto px-5 pb-16 pt-8 md:px-10">
          <div className="mx-auto max-w-5xl">
        {page === "admin" && isAdmin && (
          <>
            {back("Back to projects", "home")}
            <h1 className="sr-only">Administration</h1>
            <div className="flex min-h-[60vh] flex-col overflow-hidden rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)]">
              <AdminView onProjectsChanged={onProjectsChanged} />
            </div>
          </>
        )}

        {page === "audit" && isAdmin && (
          <>
            {back("Back to projects", "home")}
            <h1 className="sr-only">Audit log</h1>
            <div className="flex min-h-[60vh] flex-col overflow-hidden rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)]">
              <AuditView />
            </div>
          </>
        )}

        {page === "releases" && (
          <>
            {back("Back to projects", "home")}
            <div className="mx-auto max-w-3xl">
              <ReleaseNotes />
            </div>
          </>
        )}

        {page === "github" && projectSource === "github" && (
          <>
            {back("Back to projects", "home")}
            <div className="flex min-h-[40vh] flex-col overflow-hidden rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)]">
              <GitHubSettings />
            </div>
          </>
        )}

        {page === "all" && (
          <>
            {back("Back", "home")}
            <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
              <div>
                <h1 className="text-3xl font-bold tracking-tight">All projects</h1>
                <p className="mt-1 text-sm text-[var(--tf-color-text-secondary)]">
                  {filtered.length === projects.length ? `${projects.length} projects` : `${filtered.length} of ${projects.length} projects`}
                </p>
              </div>
              <div className="relative w-full max-w-xs">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
                <input
                  autoFocus
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Search projects…"
                  aria-label="Search projects"
                  className="h-10 w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] pl-9 pr-3 text-sm outline-none focus:border-[var(--tf-color-action)]"
                />
              </div>
            </div>
            {filtered.length === 0 ? (
              <p className="rounded-2xl border border-dashed border-[var(--tf-color-border-strong)] p-10 text-center text-sm text-[var(--tf-color-text-muted)]">
                No project matches “{search}”.
              </p>
            ) : (
              <ul className="m-0 grid list-none gap-5 p-0 sm:grid-cols-2 lg:grid-cols-3">{filtered.map(card)}</ul>
            )}
          </>
        )}

        {page === "home" && (
          <>
            <header className="mb-10 flex flex-wrap items-end justify-between gap-4">
              <div>
                <h1 className="text-4xl font-bold tracking-tight">
                  {user?.name ? `Welcome back, ${user.name}` : "Your projects"}
                </h1>
                <p className="mt-2 max-w-xl text-[var(--tf-color-text-secondary)]">
                  Compose Azure infrastructure from approved modules, review it, and ship a Terraform branch.
                </p>
              </div>
              {projectSource !== null && (projectSource === "github" || isAdmin) && (
                <div className="flex flex-wrap gap-2">
                  <button
                    disabled={starting}
                    onClick={startNewProject}
                    className="flex items-center gap-2 rounded-lg bg-[var(--tf-color-action)] px-4 py-2.5 text-sm font-semibold text-[var(--tf-color-on-action)] shadow-lg shadow-cyan-500/20 transition hover:bg-[var(--tf-color-action-hover)]"
                  >
                    {starting ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />} New project
                  </button>
                </div>
              )}
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
                  Create a project from one of your GitHub repositories, or ask a project owner to add you.
                </p>
                <button
                  onClick={() => setFromGitHub(new URLSearchParams())}
                  className="rounded-lg bg-[var(--tf-color-action)] px-4 py-2 text-sm font-semibold text-[var(--tf-color-on-action)] hover:bg-[var(--tf-color-action-hover)]"
                >
                  New from GitHub
                </button>
              </div>
            ) : (
              <>
                <ul className="m-0 grid list-none gap-5 p-0 sm:grid-cols-2 lg:grid-cols-3">{shown.map(card)}</ul>
                {projects.length > HOME_PROJECT_LIMIT && (
                  <div className="mt-6 flex justify-center">
                    <button
                      onClick={() => setPage("all")}
                      className="flex items-center gap-2 rounded-lg border border-[var(--tf-color-border-strong)] px-4 py-2 text-sm font-medium hover:border-[var(--tf-color-action)] hover:bg-[var(--tf-glass-hover)]"
                    >
                      View all projects ({projects.length}) <ArrowRight className="size-4" />
                    </button>
                  </div>
                )}
              </>
            )}
          </>
        )}
          </div>
          </main>
        </div>
      </CanvasGlyphs>

      {fromGitHub && (
        <GitHubProjectDialog
          returned={fromGitHub}
          onClose={() => setFromGitHub(null)}
          onCreated={(p) => {
            setFromGitHub(null);
            onProjectsChanged?.();
            onOpen(p.id);
          }}
        />
      )}

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
    </div>
  );
}
