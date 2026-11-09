import { useCallback, useEffect, useState } from "react";
import { Loader2, RefreshCw, Unplug } from "lucide-react";
import { disconnectConnection, getConnections, recheckConnection, type ConnectionRow } from "../hooks/useGitHub";

/* Admin tab: every GitHub connection (one per installation of the App) and how many
 * projects use it. Disconnecting stops those projects from reading or branching until
 * the account installs the App again; "Check" asks GitHub whether the installation
 * still exists. A connection holds no token or key. */

const STATUS: Record<ConnectionRow["status"], { label: string; cls: string }> = {
  active: { label: "Active", cls: "text-[var(--tf-color-success,#15803d)]" },
  needs_attention: { label: "Needs attention", cls: "text-[var(--tf-color-warning,#b45309)]" },
  disconnected: { label: "Disconnected", cls: "text-[var(--tf-color-text-muted)]" },
};

export function ConnectionsAdmin({ onDenied }: { onDenied: () => void }) {
  const [rows, setRows] = useState<ConnectionRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setRows(await getConnections());
      setError(null);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Cannot load connections";
      if (/admin role/i.test(msg)) onDenied();
      else setError(msg);
    }
  }, [onDenied]);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (id: string, fn: (id: string) => Promise<unknown>) => {
    setBusy(id);
    setError(null);
    try {
      await fn(id);
      setConfirm(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setBusy(null);
    }
  };

  if (!rows && !error) {
    return (
      <p className="flex items-center gap-2 text-sm text-[var(--tf-color-text-secondary)]">
        <Loader2 className="size-4 animate-spin" /> Loading connections…
      </p>
    );
  }

  return (
    <div>
      <h2 className="mb-2 text-base font-semibold">GitHub connections</h2>
      <p className="mb-4 text-sm text-[var(--tf-color-text-secondary)]">
        A connection appears when someone installs the TerraFlow GitHub App on their account or organisation and authorizes.
      </p>
      {error && (
        <p role="alert" className="mb-4 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
          {error}
        </p>
      )}
      {rows && rows.length === 0 ? (
        <p className="text-sm text-[var(--tf-color-text-muted)]">No connections yet.</p>
      ) : (
        <table className="w-full text-left text-sm">
          <thead className="text-xs uppercase text-[var(--tf-color-text-muted)]">
            <tr>
              <th className="py-2 pr-3">Account</th>
              <th className="py-2 pr-3">Status</th>
              <th className="py-2 pr-3">Projects</th>
              <th className="py-2 pr-3">Connected by</th>
              <th className="py-2" />
            </tr>
          </thead>
          <tbody>
            {rows?.map((c) => (
              <tr key={c.id} className="border-t border-[var(--tf-color-border)]">
                <td className="py-2 pr-3">
                  <span className="font-medium">{c.account_login}</span>{" "}
                  <span className="text-xs text-[var(--tf-color-text-muted)]">{c.account_type}</span>
                </td>
                <td className={`py-2 pr-3 ${STATUS[c.status].cls}`}>{STATUS[c.status].label}</td>
                <td className="py-2 pr-3">{c.projects}</td>
                <td className="py-2 pr-3 font-mono text-xs">{c.created_by}</td>
                <td className="py-2 text-right">
                  <div className="flex justify-end gap-2">
                    <button className="tf-btn tf-btn--ghost" disabled={busy === c.id} onClick={() => act(c.id, recheckConnection)}>
                      {busy === c.id ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />} Check
                    </button>
                    {c.status !== "disconnected" &&
                      (confirm === c.id ? (
                        <button
                          className="rounded-lg bg-[var(--tf-color-error)] px-3 text-sm font-semibold text-[var(--tf-color-surface)]"
                          disabled={busy === c.id}
                          onClick={() => act(c.id, disconnectConnection)}
                        >
                          Disconnect {c.projects} project{c.projects === 1 ? "" : "s"}
                        </button>
                      ) : (
                        <button className="tf-btn tf-btn--ghost" onClick={() => setConfirm(c.id)}>
                          <Unplug className="size-4" /> Disconnect
                        </button>
                      ))}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
