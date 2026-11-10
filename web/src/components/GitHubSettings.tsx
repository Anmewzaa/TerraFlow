import { useCallback, useEffect, useState } from "react";
import { ExternalLink, Github, Loader2, RefreshCw, Unplug } from "lucide-react";
import {
  AUTHORIZE_URL,
  disconnectMine,
  getGitHubStatus,
  getMyConnections,
  type GitHubStatus,
  type MyConnection,
} from "../hooks/useGitHub";

const STATUS: Record<MyConnection["status"], { label: string; cls: string }> = {
  active: { label: "Connected", cls: "text-[var(--tf-color-success,#15803d)]" },
  needs_attention: { label: "Needs attention", cls: "text-[var(--tf-color-warning,#b45309)]" },
  disconnected: { label: "Disconnected", cls: "text-[var(--tf-color-text-muted)]" },
};

/** "valid for 7 more min" / "expired" from the time GitHub's confirmation runs out. */
export function validity(iso: string | undefined, now = Date.now()): string {
  const t = iso ? Date.parse(iso) : NaN;
  if (Number.isNaN(t)) return "no repositories confirmed yet";
  const m = Math.ceil((t - now) / 60000);
  return m > 0 ? `repository list valid for ${m} more min` : "repository list expired";
}

/**
 * Home > GitHub: the GitHub accounts this person connected. TerraFlow keeps no GitHub
 * token; "Refresh" asks GitHub again which repositories are reachable, and the App's
 * own repository choice is made on GitHub.
 */
export function GitHubSettings() {
  const [rows, setRows] = useState<MyConnection[] | null>(null);
  const [status, setStatus] = useState<GitHubStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [s, c] = await Promise.all([getGitHubStatus(), getMyConnections()]);
      setStatus(s);
      setRows(c);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Cannot load your GitHub connections");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const disconnect = async (id: string) => {
    setBusy(id);
    setError(null);
    try {
      await disconnectMine(id);
      setConfirm(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not disconnect");
    } finally {
      setBusy(null);
    }
  };

  return (
    <section className="p-6" aria-label="GitHub connections">
      <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
        <Github className="size-6" /> GitHub
      </h1>
      <p className="mt-1 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">
        TerraFlow reads your repositories and opens branches through the TerraFlow GitHub App. It never keeps your GitHub
        password or token; every time you start a project it asks GitHub again which repositories you can reach.
      </p>

      {error && (
        <p role="alert" className="mt-4 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
          {error}
        </p>
      )}

      {rows === null && !error && (
        <p className="mt-6 flex items-center gap-2 text-sm text-[var(--tf-color-text-secondary)]">
          <Loader2 className="size-4 animate-spin" /> Loading…
        </p>
      )}

      {rows && rows.length === 0 && (
        <div className="mt-6 text-sm text-[var(--tf-color-text-secondary)]">
          <p>No GitHub account is connected yet.</p>
          <a className="tf-btn mt-4 inline-flex" href={AUTHORIZE_URL}>
            <Github className="size-4" /> Connect GitHub
          </a>
        </div>
      )}

      {rows && rows.length > 0 && (
        <ul className="mt-6 grid list-none gap-3 p-0">
          {rows.map((c) => (
            <li key={c.id} className="rounded-xl border border-[var(--tf-color-border)] p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="font-semibold">
                    {c.account}{" "}
                    <span className="text-xs font-normal text-[var(--tf-color-text-muted)]">({c.account_type.toLowerCase()})</span>
                  </p>
                  <p className="text-xs text-[var(--tf-color-text-muted)]">
                    <span className={STATUS[c.status].cls}>{STATUS[c.status].label}</span> · {c.projects}{" "}
                    {c.projects === 1 ? "project" : "projects"} · {c.repositories} repositories · {validity(c.expires_at)}
                  </p>
                  {c.status === "needs_attention" && (
                    <p className="mt-1 text-xs text-[var(--tf-color-warning,#b45309)]">
                      GitHub no longer confirms this installation. Refresh it, or install the App again.
                    </p>
                  )}
                </div>
                <div className="flex flex-wrap gap-2">
                  <a className="tf-btn tf-btn--ghost" href={AUTHORIZE_URL}>
                    <RefreshCw className="size-4" /> Refresh
                  </a>
                  {status?.install_url && (
                    <a className="tf-btn tf-btn--ghost" href={status.install_url}>
                      <ExternalLink className="size-4" /> Choose repositories
                    </a>
                  )}
                  {confirm === c.id ? (
                    <>
                      <button className="tf-btn" disabled={busy === c.id} onClick={() => disconnect(c.id)}>
                        {busy === c.id && <Loader2 className="size-4 animate-spin" />} Confirm
                      </button>
                      <button className="tf-btn tf-btn--ghost" onClick={() => setConfirm(null)}>
                        Cancel
                      </button>
                    </>
                  ) : (
                    <button
                      className="tf-btn tf-btn--ghost"
                      disabled={c.projects > 0}
                      title={c.projects > 0 ? "Projects still use this connection" : "Disconnect this account"}
                      onClick={() => setConfirm(c.id)}
                    >
                      <Unplug className="size-4" /> Disconnect
                    </button>
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-6 text-xs text-[var(--tf-color-text-muted)]">
        To remove the App from your GitHub account completely, uninstall it in GitHub, Settings, Applications.
      </p>
    </section>
  );
}
