import { useCallback, useEffect, useState } from "react";
import type { RawResourceTemplate } from "../hooks/useRawResources";
import { parseValues } from "../lib/rawResource";

/* Admin tab: raw provider resources prepared for developers (D-041). A raw
 * resource is an empty `resource "azurerm_x" "name" {}`; it is not a module. */

const field =
  "mt-1 block w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]";

interface Draft {
  name: string;
  title: string;
  resource_type: string;
  description: string;
  target_folder: string;
  values: string;
}

const EMPTY: Draft = { name: "", title: "", resource_type: "", description: "", target_folder: "", values: "{}" };

function toDraft(t: RawResourceTemplate): Draft {
  return {
    name: t.name,
    title: t.title,
    resource_type: t.resource_type,
    description: t.description,
    target_folder: t.target_folder,
    values: JSON.stringify(t.default_values ?? {}, null, 2),
  };
}

async function send(method: string, path: string, body?: unknown): Promise<{ ok: boolean; status: number; error: string }> {
  const res = await fetch(`/api${path}`, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  let error = "";
  if (!res.ok) {
    try {
      error = (await res.json()).error ?? "";
    } catch {
      /* no body */
    }
    error ||= `Request failed (${res.status})`;
  }
  return { ok: res.ok, status: res.status, error };
}

export function RawResourcesAdmin({ onDenied }: { onDenied: () => void }) {
  const [rows, setRows] = useState<RawResourceTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<{ draft: Draft; isNew: boolean } | null>(null);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);

  const load = useCallback(async () => {
    const r = await fetch("/api/catalog/resources");
    setRows(r.ok ? await r.json() : []);
    setLoading(false);
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const save = async () => {
    if (!editing) return;
    const d = editing.draft;
    const parsed = parseValues(d.values);
    if ("error" in parsed) return setMsg({ tone: "error", text: `Default arguments: ${parsed.error}` });
    const body = {
      name: d.name,
      title: d.title,
      resource_type: d.resource_type.trim(),
      description: d.description,
      target_folder: d.target_folder.trim(),
      default_values: parsed.values,
    };
    const r = editing.isNew
      ? await send("POST", "/admin/resources", body)
      : await send("PUT", `/admin/resources/${encodeURIComponent(d.name)}`, body);
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setMsg({ tone: "ok", text: editing.isNew ? "Raw resource added." : "Raw resource saved." });
    setEditing(null);
    load();
  };

  const remove = async (t: RawResourceTemplate) => {
    if (!confirm(`Delete "${t.title}"? Resources developers already placed on a canvas keep working.`)) return;
    const r = await send("DELETE", `/admin/resources/${encodeURIComponent(t.name)}`);
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setMsg({ tone: "ok", text: "Raw resource deleted." });
    load();
  };

  const set = (k: keyof Draft, v: string) => setEditing((e) => (e ? { ...e, draft: { ...e.draft, [k]: v } } : e));

  return (
    <div>
      {msg && (
        <div
          role={msg.tone === "error" ? "alert" : "status"}
          className="mb-3 rounded-lg border px-3 py-2 text-sm"
          style={{
            borderColor: msg.tone === "error" ? "var(--tf-color-error)" : "var(--tf-color-success)",
            color: msg.tone === "error" ? "var(--tf-color-error)" : "var(--tf-color-success)",
          }}
        >
          {msg.text}
        </div>
      )}
      <div className="mb-3 flex items-start justify-between gap-4">
        <p className="m-0 max-w-xl text-sm text-[var(--tf-color-text-muted)]">
          A raw resource is a plain <code>resource</code> block, such as <code>azurerm_resource_group</code>, that
          developers can place on the canvas. It is not a module and has no schema; the default arguments are only a
          starting point.
        </p>
        <button className="tf-btn" onClick={() => { setMsg(null); setEditing({ draft: EMPTY, isNew: true }); }}>
          Add raw resource
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-[var(--tf-color-text-muted)]">Loading…</p>
      ) : (
        <table className="mb-6 w-full text-left text-sm">
          <thead className="text-xs uppercase text-[var(--tf-color-text-muted)]">
            <tr><th className="py-2">Title</th><th>Resource type</th><th>Default folder</th><th /></tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr><td colSpan={4} className="py-6 text-center text-[var(--tf-color-text-muted)]">None yet. Add the first one.</td></tr>
            )}
            {rows.map((t) => (
              <tr key={t.name} className="border-t border-[var(--tf-color-border)]">
                <td className="py-2"><strong>{t.title}</strong><div className="text-xs text-[var(--tf-color-text-muted)]">{t.description}</div></td>
                <td className="font-mono text-xs">{t.resource_type}</td>
                <td className="font-mono text-xs">{t.target_folder || "first folder"}</td>
                <td className="whitespace-nowrap text-right">
                  <button className="tf-btn tf-btn--ghost" onClick={() => { setMsg(null); setEditing({ draft: toDraft(t), isNew: false }); }}>Edit</button>{" "}
                  <button className="tf-btn tf-btn--ghost" style={{ color: "var(--tf-color-error)" }} onClick={() => remove(t)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {editing && (
        <div className="grid max-w-xl gap-3 rounded-xl border border-[var(--tf-color-border)] p-4">
          <h3 className="m-0 text-sm font-semibold">{editing.isNew ? "New raw resource" : `Edit ${editing.draft.name}`}</h3>
          <label className="text-sm">Name (identifier)
            <input className={`${field} font-mono`} value={editing.draft.name} disabled={!editing.isNew} placeholder="resource-group"
              onChange={(e) => set("name", e.target.value.toLowerCase())} />
          </label>
          <label className="text-sm">Title
            <input className={field} value={editing.draft.title} maxLength={80} placeholder="Resource group" onChange={(e) => set("title", e.target.value)} />
          </label>
          <label className="text-sm">Resource type
            <input className={`${field} font-mono`} value={editing.draft.resource_type} placeholder="azurerm_resource_group"
              onChange={(e) => set("resource_type", e.target.value)} />
          </label>
          <label className="text-sm">Description
            <input className={field} value={editing.draft.description} maxLength={500} onChange={(e) => set("description", e.target.value)} />
          </label>
          <label className="text-sm">Default folder <span className="text-[var(--tf-color-text-muted)]">(optional)</span>
            <input className={`${field} font-mono`} value={editing.draft.target_folder} placeholder="common" onChange={(e) => set("target_folder", e.target.value)} />
          </label>
          <label className="text-sm">Default arguments (JSON)
            <textarea className={`${field} font-mono text-xs`} rows={7} spellCheck={false} value={editing.draft.values} onChange={(e) => set("values", e.target.value)} />
          </label>
          <div className="flex gap-2">
            <button className="tf-btn" onClick={save}>Save</button>
            <button className="tf-btn tf-btn--ghost" onClick={() => setEditing(null)}>Cancel</button>
          </div>
        </div>
      )}
    </div>
  );
}
