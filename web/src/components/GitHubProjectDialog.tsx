import { useEffect, useMemo, useState } from "react";
import { Github, Loader2, Lock, X } from "lucide-react";
import {
  AUTHORIZE_URL,
  createFromRepo,
  getFolders,
  getGitHubStatus,
  getRepos,
  type GitHubStatus,
  type RepoChoice,
} from "../hooks/useGitHub";
import type { Project } from "../types/project";

/** What the server puts in `?error=` when it sends the browser back from GitHub. */
const RETURN_ERRORS: Record<string, string> = {
  state: "The sign-in with GitHub could not be verified. Please try again.",
  denied: "GitHub access was not granted.",
  code: "GitHub did not accept the authorization. Please try again.",
  github: "GitHub could not be reached. Please try again.",
  server: "TerraFlow could not save the connection. Please try again.",
  not_allowed: "This GitHub account is not allowed to connect to this TerraFlow.",
  no_installation:
    "The TerraFlow GitHub App is not installed on any account you can reach. Install it on your account or organisation first, then come back.",
};

const field =
  "w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]";

interface Props {
  /** The query string the server returned with (`connected=1` or `error=code`). */
  returned?: URLSearchParams;
  onClose: () => void;
  onCreated: (project: Project) => void;
}

/** New project from a GitHub repository: connect, pick a repository, pick the folder. */
export function GitHubProjectDialog({ returned, onClose, onCreated }: Props) {
  const [status, setStatus] = useState<GitHubStatus | null>(null);
  const [repos, setRepos] = useState<RepoChoice[] | null>(null);
  const [error, setError] = useState<string | null>(() => {
    const code = returned?.get("error");
    return code ? RETURN_ERRORS[code] ?? "Something went wrong with GitHub." : null;
  });
  const [repoKey, setRepoKey] = useState("");
  const [folders, setFolders] = useState<string[] | null>(null);
  const [folder, setFolder] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  const repo = useMemo(() => repos?.find((r) => `${r.connection_id}/${r.repo_id}` === repoKey), [repos, repoKey]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const s = await getGitHubStatus();
        if (cancelled) return;
        setStatus(s);
        if (s.configured) setRepos(await getRepos());
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Cannot reach the server");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // Picking a repository asks GitHub (through the server) which folders hold Terraform.
  useEffect(() => {
    setFolders(null);
    setFolder("");
    if (!repo) return;
    let cancelled = false;
    getFolders(repo.connection_id, repo.repo_id)
      .then((r) => {
        if (cancelled) return;
        setFolders(r.folders);
        // Suggest a name from the repository once; the user can change it.
        setName((n) => n || repo.full_name.split("/")[1]);
        if (r.folders.length === 1) setFolder(r.folders[0]);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Cannot read the repository");
      });
    return () => {
      cancelled = true;
    };
  }, [repo]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!repo) return;
    setBusy(true);
    setError(null);
    try {
      onCreated(await createFromRepo({ name: name.trim(), connection_id: repo.connection_id, repo_id: repo.repo_id, folder }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the project");
    } finally {
      setBusy(false);
    }
  };

  const authorize = () => {
    window.location.href = AUTHORIZE_URL;
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--tf-color-overlay)] p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label="New project from GitHub"
    >
      <form
        onSubmit={submit}
        className="w-full max-w-lg rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-6 shadow-2xl"
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="flex items-center gap-2 text-xl font-semibold">
            <Github className="size-5" /> New project from GitHub
          </h2>
          <button type="button" onClick={onClose} aria-label="Close" className="text-[var(--tf-color-text-muted)] hover:text-[var(--tf-color-text)]">
            <X className="size-5" />
          </button>
        </div>

        {error && (
          <p role="alert" className="mb-4 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
            {error}
          </p>
        )}

        {!status && !error && (
          <p className="flex items-center gap-2 text-sm text-[var(--tf-color-text-secondary)]">
            <Loader2 className="size-4 animate-spin" /> Checking GitHub…
          </p>
        )}

        {status && !status.configured && (
          <p className="text-sm text-[var(--tf-color-text-secondary)]">
            GitHub is not set up on this TerraFlow server. An administrator has to configure the GitHub App
            (<code>TF_GITHUB_APP_*</code>) first.
          </p>
        )}

        {status?.configured && repos && repos.length === 0 && (
          <div className="text-sm text-[var(--tf-color-text-secondary)]">
            <p>
              TerraFlow asks GitHub, each time, which repositories you can reach. It does not keep your GitHub
              token. Connect to see them.
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              <button type="button" className="tf-btn" onClick={authorize}>
                <Github className="size-4" /> Connect GitHub
              </button>
              {status.install_url && (
                <a className="tf-btn tf-btn--ghost" href={status.install_url} target="_blank" rel="noreferrer">
                  Install the App on an account
                </a>
              )}
            </div>
          </div>
        )}

        {status?.configured && repos && repos.length > 0 && (
          <>
            <label htmlFor="gh-repo" className="mb-1 block text-sm text-[var(--tf-color-text-secondary)]">
              Repository
            </label>
            <select id="gh-repo" className={`${field} mb-1`} value={repoKey} onChange={(e) => setRepoKey(e.target.value)} required>
              <option value="">Choose a repository…</option>
              {repos.map((r) => (
                <option key={`${r.connection_id}/${r.repo_id}`} value={`${r.connection_id}/${r.repo_id}`}>
                  {r.full_name}
                  {r.private ? " (private)" : ""}
                </option>
              ))}
            </select>
            <p className="mb-4 flex items-center gap-1 text-xs text-[var(--tf-color-text-muted)]">
              <Lock className="size-3" /> Only repositories you can access are listed. The list expires after a few minutes;
              <button type="button" className="underline" onClick={authorize}>
                refresh it
              </button>
              .
            </p>

            {repo && (
              <>
                <label htmlFor="gh-folder" className="mb-1 block text-sm text-[var(--tf-color-text-secondary)]">
                  Terraform folder
                </label>
                {folders === null ? (
                  <p className="mb-4 flex items-center gap-2 text-sm text-[var(--tf-color-text-secondary)]">
                    <Loader2 className="size-4 animate-spin" /> Reading {repo.full_name}…
                  </p>
                ) : folders.length === 0 ? (
                  <p className="mb-4 text-sm text-[var(--tf-color-error)]">No folder in this repository holds Terraform (.tf) files.</p>
                ) : (
                  <select id="gh-folder" className={`${field} mb-4 font-mono text-xs`} value={folder} onChange={(e) => setFolder(e.target.value)}>
                    {folders.length > 1 && <option value="">Choose a folder…</option>}
                    {folders.map((f) => (
                      <option key={f || "(root)"} value={f}>
                        {f || "(repository root)"}
                      </option>
                    ))}
                  </select>
                )}

                <label htmlFor="gh-name" className="mb-1 block text-sm text-[var(--tf-color-text-secondary)]">
                  Project name
                </label>
                <input id="gh-name" required maxLength={80} className={`${field} mb-2`} value={name} onChange={(e) => setName(e.target.value)} />
                <p className="text-xs text-[var(--tf-color-text-muted)]">
                  You become the owner. People you invite can open branches on this repository through TerraFlow.
                </p>
              </>
            )}
          </>
        )}

        <div className="mt-6 flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-strong)]">
            Cancel
          </button>
          <button
            type="submit"
            disabled={busy || !repo || folders === null || folders.length === 0 || (folders.length > 1 && !folder) || !name.trim()}
            className="tf-btn disabled:opacity-50"
          >
            {busy && <Loader2 className="size-4 animate-spin" />} Create project
          </button>
        </div>
      </form>
    </div>
  );
}
