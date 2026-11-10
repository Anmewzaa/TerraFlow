import { useEffect, useMemo, useState, type ReactNode } from "react";
import { AlertTriangle, Github, Loader2, Lock, X } from "lucide-react";
import {
  ApiError,
  AUTHORIZE_URL,
  createFromRepo,
  getFolders,
  getGitHubStatus,
  getRepos,
  initRepo,
  NEW_FOLDER_RE,
  type GitHubStatus,
  type RepoChoice,
} from "../hooks/useGitHub";
import type { Project } from "../types/project";
import { RepoPicker, repoKeyOf } from "./RepoPicker";
import { putGeneration } from "../hooks/useGeneration";
import { useProviders } from "../hooks/useProviders";
import { DEFAULT_SINGLE_FILE, DEFAULT_SINGLE_FOLDER, STARTER_PRESETS, VAR_NAME, type GenerationSettings } from "../lib/generation";

/** The optional "Advanced" choices made while creating a project (docs/11-project-settings.md). */
interface Advanced {
  single: boolean;
  folder: string;
  file: string;
  /** Cloud provider id and version constraint ("" = none chosen). */
  provider: string;
  providerVersion: string;
  backend: boolean;
  vars: { name: string; value: string }[];
}
const NO_ADVANCED: Advanced = { single: false, folder: "", file: "", provider: "", providerVersion: "", backend: false, vars: [] };

/** The settings document an Advanced form describes; null when nothing was chosen. */
export function settingsFromAdvanced(a: Advanced): GenerationSettings | null {
  const variables = a.vars
    .filter((v) => v.name.trim() && v.value.trim())
    .map((v) => ({ name: v.name.trim(), values: { dev: v.value.trim() } }));
  const starters = a.backend ? STARTER_PRESETS.map((p) => p.file) : [];
  const providers = a.provider ? [{ id: a.provider, ...(a.providerVersion.trim() ? { version: a.providerVersion.trim() } : {}) }] : [];
  if (!a.single && variables.length === 0 && starters.length === 0 && providers.length === 0) return null;
  return {
    layout: a.single ? { mode: "single", folder: a.folder.trim() || DEFAULT_SINGLE_FOLDER, file: a.file.trim() || DEFAULT_SINGLE_FILE } : {},
    variables,
    starters,
    providers,
  };
}

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
  "w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none transition focus:border-[var(--tf-color-action)] focus:ring-2 focus:ring-[var(--tf-color-action)]/25";

/** One numbered step of the dialog: what to decide, and why. */
function Step({ n, title, hint, children }: { n: number; title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-canvas)]/40 p-4">
      <div className="mb-3 flex items-start gap-3">
        <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-[var(--tf-color-action)] text-xs font-bold text-[var(--tf-color-on-action)]">
          {n}
        </span>
        <div>
          <h3 className="text-sm font-semibold leading-6">{title}</h3>
          {hint && <p className="text-xs text-[var(--tf-color-text-muted)]">{hint}</p>}
        </div>
      </div>
      {children}
    </section>
  );
}

interface Props {
  /** The query string the server returned with (`connected=1` or `error=code`). */
  returned?: URLSearchParams;
  onClose: () => void;
  onCreated: (project: Project) => void;
}

/** New project from a GitHub repository: connect, pick a repository, pick the folder, name it. */
export function GitHubProjectDialog({ returned, onClose, onCreated }: Props) {
  const [status, setStatus] = useState<GitHubStatus | null>(null);
  const [repos, setRepos] = useState<RepoChoice[] | null>(null);
  const [error, setError] = useState<string | null>(() => {
    const code = returned?.get("error");
    return code ? RETURN_ERRORS[code] ?? "Something went wrong with GitHub." : null;
  });
  const [repoKey, setRepoKey] = useState("");
  const [folders, setFolders] = useState<string[] | null>(null);
  const [folderError, setFolderError] = useState<{ message: string; code?: string } | null>(null);
  const [reload, setReload] = useState(0);
  const [folder, setFolder] = useState("");
  // Starting in a folder that holds no Terraform yet (an empty folder or a new project).
  const [useNew, setUseNew] = useState(false);
  const [newFolder, setNewFolder] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmInit, setConfirmInit] = useState(false);
  const [adv, setAdv] = useState<Advanced>(NO_ADVANCED);
  const { providers: registry } = useProviders();
  // Back from GitHub with nothing to list: the App still has to be given repositories.
  const needsInstall = returned?.get("connected") != null || returned?.get("error") === "no_installation";

  const repo = useMemo(() => repos?.find((r) => repoKeyOf(r) === repoKey), [repos, repoKey]);

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
    setFolderError(null);
    setFolder("");
    setUseNew(false);
    setNewFolder("");
    setConfirmInit(false);
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
        if (cancelled) return;
        setFolderError({ message: e instanceof Error ? e.message : "Cannot read the repository", code: e instanceof ApiError ? e.code : undefined });
      });
    return () => {
      cancelled = true;
    };
  }, [repo, reload]);

  const startingNew = folders !== null && !folderError && (folders.length === 0 || useNew);
  const newPath = newFolder.trim().replace(/^\/+|\/+$/g, "");
  const chosenFolder = startingNew ? newPath : folder;
  const folderReady = startingNew ? NEW_FOLDER_RE.test(newPath) : !!folders && folders.length > 0 && (folders.length === 1 || !!folder);
  const badVar = adv.vars.some((v) => v.name.trim() !== "" && !VAR_NAME.test(v.name.trim()));
  const canSubmit = !busy && !!repo && folders !== null && !folderError && folderReady && !!name.trim() && !badVar;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!repo) return;
    setBusy(true);
    setError(null);
    try {
      const project = await createFromRepo({
        name: name.trim(),
        connection_id: repo.connection_id,
        repo_id: repo.repo_id,
        folder: chosenFolder,
        new_folder: startingNew,
      });
      const chosen = settingsFromAdvanced(adv);
      if (chosen) {
        try {
          await putGeneration(project.id, chosen);
        } catch (err) {
          // The project exists; the settings can be set again from its Settings page.
          window.alert(`The project was created, but its settings could not be saved: ${err instanceof Error ? err.message : "unknown error"}\n\nOpen Settings in the project to set them.`);
        }
      }
      onCreated(project);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the project");
    } finally {
      setBusy(false);
    }
  };

  const initialize = async () => {
    if (!repo) return;
    setBusy(true);
    setError(null);
    try {
      await initRepo(repo.connection_id, repo.repo_id);
      setConfirmInit(false);
      setReload((n) => n + 1); // read the repository again: it now has a commit
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not initialize the repository");
    } finally {
      setBusy(false);
    }
  };

  const authorize = () => {
    window.location.href = AUTHORIZE_URL;
  };

  const hasRepos = !!status?.configured && !!repos && repos.length > 0;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--tf-color-overlay)] p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label="New project from GitHub"
    >
      <form
        onSubmit={submit}
        className="flex max-h-[92vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] shadow-2xl"
      >
        <header className="flex items-start justify-between gap-4 border-b border-[var(--tf-color-border)] px-6 py-4">
          <div>
            <h2 className="flex items-center gap-2 text-xl font-semibold">
              <Github className="size-5" /> New project from GitHub
            </h2>
            <p className="mt-0.5 text-sm text-[var(--tf-color-text-secondary)]">
              Pick the repository and folder TerraFlow should read, and where it opens branches.
            </p>
          </div>
          <button type="button" onClick={onClose} aria-label="Close" className="rounded-md p-1 text-[var(--tf-color-text-muted)] hover:bg-[var(--tf-glass-strong)] hover:text-[var(--tf-color-text)]">
            <X className="size-5" />
          </button>
        </header>

        <div className="flex-1 space-y-4 overflow-y-auto px-6 py-5">
          {error && (
            <p role="alert" className="rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
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
              GitHub is not set up on this TerraFlow server. An administrator has to configure the GitHub App (<code>TF_GITHUB_APP_*</code>) first.
            </p>
          )}

          {status?.configured && repos && repos.length === 0 && (
            <div className="rounded-xl border border-dashed border-[var(--tf-color-border-strong)] p-6 text-center text-sm text-[var(--tf-color-text-secondary)]">
              {needsInstall ? (
                <>
                  <p>GitHub is connected, but the TerraFlow App has not been given any repository yet. Choose the repositories it may use, save, and come back.</p>
                  {status.install_url && (
                    <a className="tf-btn mt-4 inline-flex" href={status.install_url} rel="noreferrer">
                      <Github className="size-4" /> Choose repositories on GitHub
                    </a>
                  )}
                </>
              ) : (
                <>
                  <p>Connect your GitHub account to pick a repository. TerraFlow does not keep your GitHub token.</p>
                  <button type="button" className="tf-btn mt-4" onClick={authorize}>
                    <Github className="size-4" /> Connect GitHub
                  </button>
                </>
              )}
            </div>
          )}

          {hasRepos && repos && (
            <>
              <Step n={1} title="Repository" hint="Only repositories you can access are listed.">
                <RepoPicker repos={repos} value={repoKey} onChange={setRepoKey} />
                <p className="mt-2 flex flex-wrap items-center gap-x-1 text-xs text-[var(--tf-color-text-muted)]">
                  <Lock className="size-3" /> The list is valid for a few minutes;
                  <button type="button" className="underline" onClick={authorize}>
                    refresh it
                  </button>
                  .
                  {status?.install_url && (
                    <>
                      {" "}
                      Missing one?{" "}
                      <a className="underline" href={status.install_url} rel="noreferrer">
                        Choose repositories on GitHub
                      </a>
                      .
                    </>
                  )}
                </p>
              </Step>

              {repo && (
                <Step n={2} title="Terraform folder" hint="The folder TerraFlow reads and writes branches for.">
                  {folderError?.code === "empty_repo" ? (
                    <div className="rounded-lg border border-[var(--tf-color-warning,#b45309)]/40 bg-[var(--tf-color-warning,#b45309)]/10 p-4 text-sm">
                      <p className="flex items-center gap-2 font-medium">
                        <AlertTriangle className="size-4" /> This repository is empty
                      </p>
                      <p className="mt-1 text-[var(--tf-color-text-secondary)]">
                        It has no commits yet, so there is no branch to read. TerraFlow can add a <code>README.md</code> as the first commit, which
                        creates the default branch. This is the only time TerraFlow writes outside a review branch, and it only works on a
                        repository with no commits.
                      </p>
                      {confirmInit ? (
                        <div className="mt-3 flex flex-wrap items-center gap-2">
                          <span className="text-xs">
                            Add README.md to <strong>{repo.full_name}</strong>?
                          </span>
                          <button type="button" className="tf-btn" disabled={busy} onClick={initialize}>
                            {busy && <Loader2 className="size-4 animate-spin" />} Yes, initialize
                          </button>
                          <button type="button" className="tf-btn tf-btn--ghost" onClick={() => setConfirmInit(false)}>
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <button type="button" className="tf-btn mt-3" onClick={() => setConfirmInit(true)}>
                          Initialize repository
                        </button>
                      )}
                    </div>
                  ) : folderError ? (
                    <p role="alert" className="text-sm text-[var(--tf-color-error)]">
                      {folderError.message}
                    </p>
                  ) : folders === null ? (
                    <p className="flex items-center gap-2 text-sm text-[var(--tf-color-text-secondary)]">
                      <Loader2 className="size-4 animate-spin" /> Reading {repo.full_name}…
                    </p>
                  ) : startingNew ? (
                    <div>
                      {folders.length === 0 && (
                        <p className="mb-2 text-sm text-[var(--tf-color-text-secondary)]">
                          This repository has no Terraform yet. Name the folder TerraFlow should use; the first publish creates it on a branch.
                        </p>
                      )}
                      <input
                        id="gh-newfolder"
                        aria-label="New folder"
                        autoFocus
                        placeholder="e.g. terraform/dev"
                        className={`${field} font-mono text-xs`}
                        value={newFolder}
                        onChange={(e) => setNewFolder(e.target.value)}
                      />
                      {newPath && !NEW_FOLDER_RE.test(newPath) && (
                        <p className="mt-1 text-xs text-[var(--tf-color-error)]">Use letters, digits, dot, dash or underscore; up to four levels separated by “/”.</p>
                      )}
                      {folders.length > 0 && (
                        <button type="button" className="mt-2 text-xs underline" onClick={() => setUseNew(false)}>
                          Use a folder that already has Terraform
                        </button>
                      )}
                    </div>
                  ) : (
                    <div>
                      <select id="gh-folder" aria-label="Terraform folder" className={`${field} font-mono text-xs`} value={folder} onChange={(e) => setFolder(e.target.value)}>
                        {folders.length > 1 && <option value="">Choose a folder…</option>}
                        {folders.map((f) => (
                          <option key={f || "(root)"} value={f}>
                            {f || "(repository root)"}
                          </option>
                        ))}
                      </select>
                      <button type="button" className="mt-2 text-xs underline" onClick={() => setUseNew(true)}>
                        Start a new folder instead
                      </button>
                    </div>
                  )}
                </Step>
              )}

              {repo && folders !== null && !folderError && folderReady && (
                <Step n={3} title="Project name" hint="You become the owner. People you invite can open branches on this repository through TerraFlow.">
                  <input id="gh-name" aria-label="Project name" required maxLength={80} className={field} value={name} onChange={(e) => setName(e.target.value)} />
                </Step>
              )}

              {repo && folders !== null && !folderError && folderReady && (
                <details className="rounded-xl border border-[var(--tf-color-border)] px-4 py-3">
                  <summary className="cursor-pointer text-sm font-semibold">Advanced settings (optional)</summary>
                  <p className="m-0 mt-2 text-xs text-[var(--tf-color-text-muted)]">
                    Defaults for how the Terraform is written. You can change all of this later in the project's Settings.
                  </p>

                  <p className="m-0 mt-3 text-xs font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">Where new resources go</p>
                  <div className="mt-1.5 grid gap-2 sm:grid-cols-2">
                    {[
                      [false, "Each kind in its own folder", "common/rg.tf, network/vnet.tf"],
                      [true, "Everything in one file", "one folder, one file"],
                    ].map(([single, title, hint]) => (
                      <label key={String(single)} className={`flex cursor-pointer gap-2 rounded-lg border p-2.5 text-sm ${adv.single === single ? "border-[var(--tf-color-action)] bg-[var(--tf-glass-strong)]" : "border-[var(--tf-color-border)]"}`}>
                        <input type="radio" name="adv-layout" checked={adv.single === single} onChange={() => setAdv({ ...adv, single: single as boolean })} className="mt-0.5" />
                        <span>
                          <span className="block font-medium">{title as string}</span>
                          <span className="text-xs text-[var(--tf-color-text-muted)]">{hint as string}</span>
                        </span>
                      </label>
                    ))}
                  </div>
                  {adv.single && (
                    <div className="mt-2 grid max-w-sm grid-cols-2 gap-2">
                      <input aria-label="Folder" className={`${field} font-mono text-xs`} placeholder={DEFAULT_SINGLE_FOLDER} value={adv.folder} onChange={(e) => setAdv({ ...adv, folder: e.target.value })} />
                      <input aria-label="File" className={`${field} font-mono text-xs`} placeholder={DEFAULT_SINGLE_FILE} value={adv.file} onChange={(e) => setAdv({ ...adv, file: e.target.value })} />
                    </div>
                  )}

                  {registry.length > 0 && (
                    <>
                      <p className="m-0 mt-4 text-xs font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">Cloud provider</p>
                      <p className="m-0 mt-1 text-xs text-[var(--tf-color-text-muted)]">The raw resources offered are the ones that fit this provider and version. A new folder gets a providers.tf.</p>
                      <div className="mt-1.5 grid max-w-md grid-cols-2 gap-2">
                        <select
                          aria-label="Cloud provider"
                          className={field}
                          value={adv.provider}
                          onChange={(e) => setAdv({ ...adv, provider: e.target.value, providerVersion: registry.find((p) => p.id === e.target.value)?.versions[0] ?? "" })}
                        >
                          <option value="">No provider chosen</option>
                          {registry.map((p) => (
                            <option key={p.id} value={p.id}>
                              {p.name}
                            </option>
                          ))}
                        </select>
                        <input
                          aria-label="Provider version"
                          list="adv-provider-versions"
                          disabled={!adv.provider}
                          className={`${field} font-mono text-xs`}
                          placeholder="~> 4.0"
                          value={adv.providerVersion}
                          onChange={(e) => setAdv({ ...adv, providerVersion: e.target.value })}
                        />
                        <datalist id="adv-provider-versions">
                          {(registry.find((p) => p.id === adv.provider)?.versions ?? []).map((v) => (
                            <option key={v} value={v} />
                          ))}
                        </datalist>
                      </div>
                    </>
                  )}

                  <p className="m-0 mt-4 text-xs font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">Variables</p>
                  <p className="m-0 mt-1 text-xs text-[var(--tf-color-text-muted)]">Type {"{org}"} in a field and the code uses var.org. Value shown is for dev; add other environments later.</p>
                  {adv.vars.map((v, i) => (
                    <div key={i} className="mt-1.5 flex gap-2">
                      <input aria-label="Variable name" className={`${field} w-36 font-mono text-xs ${v.name && !VAR_NAME.test(v.name) ? "border-[var(--tf-color-error)]" : ""}`} placeholder="org" value={v.name} onChange={(e) => setAdv({ ...adv, vars: adv.vars.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)) })} />
                      <input aria-label="Value for dev" className={`${field} flex-1 font-mono text-xs`} placeholder="aic" value={v.value} onChange={(e) => setAdv({ ...adv, vars: adv.vars.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)) })} />
                      <button type="button" aria-label="Remove variable" className="px-2 text-[var(--tf-color-text-muted)]" onClick={() => setAdv({ ...adv, vars: adv.vars.filter((_, j) => j !== i) })}>
                        ×
                      </button>
                    </div>
                  ))}
                  <button type="button" className="mt-2 text-xs underline" onClick={() => setAdv({ ...adv, vars: [...adv.vars, { name: "", value: "" }] })}>
                    Add a variable
                  </button>

                  <p className="m-0 mt-4 text-xs font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">Starter files</p>
                  <label className="mt-1.5 flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={adv.backend} onChange={(e) => setAdv({ ...adv, backend: e.target.checked })} /> backend.tf (azurerm state; edit the placeholders in Settings)
                  </label>
                </details>
              )}
            </>
          )}
        </div>

        <footer className="flex justify-end gap-2 border-t border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] px-6 py-4">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-strong)]">
            Cancel
          </button>
          <button type="submit" disabled={!canSubmit} className="tf-btn disabled:opacity-50">
            {busy && <Loader2 className="size-4 animate-spin" />} Create project
          </button>
        </footer>
      </form>
    </div>
  );
}
