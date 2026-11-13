import { useCallback, useEffect, useState } from "react";
import { Loader2, Pencil, Plus, Shapes, Trash2 } from "lucide-react";
import type { RawResourceTemplate } from "../../hooks/useRawResources";
import { parseValues } from "../../lib/rawResource";
import { call } from "./api";
import { Card, Chip, EmptyState, HelpButton, inputCls, Notice, PanelHeader } from "./ui";
import { useProviders } from "../../hooks/useProviders";
import { providerOfType } from "../../lib/provider";
import { catalogPaths, type CatalogPaths } from "../../lib/catalogApi";

const GLOBAL = catalogPaths(null);

/* Raw provider resources prepared for developers (D-041). A raw resource is a plain
 * `resource "azurerm_x" "name" {}`, not a module: it has no schema. */

interface Draft {
  name: string;
  title: string;
  resource_type: string;
  description: string;
  target_folder: string;
  values: string;
  category: string;
  min_version: string;
  max_version: string;
}

const EMPTY: Draft = { name: "", title: "", resource_type: "", description: "", target_folder: "", values: "{}", category: "", min_version: "", max_version: "" };

/** Starting points, so a first-time admin sees what a complete entry looks like. */
const PRESETS: { label: string; draft: Draft }[] = [
  {
    label: "Resource group",
    draft: { ...EMPTY, name: "resource-group", title: "Resource group", resource_type: "azurerm_resource_group", description: "A plain resource group.", target_folder: "common", values: '{\n  "location": "southeastasia"\n}', category: "other" },
  },
  {
    label: "Storage account",
    draft: {
      ...EMPTY,
      name: "storage-account",
      title: "Storage account",
      resource_type: "azurerm_storage_account",
      description: "A storage account for one-off needs.",
      values: '{\n  "account_tier": "Standard",\n  "account_replication_type": "LRS"\n}',
      category: "storage",
    },
  },
  {
    label: "Public IP",
    draft: {
      ...EMPTY,
      name: "public-ip",
      title: "Public IP address",
      resource_type: "azurerm_public_ip",
      description: "A static public IP address.",
      values: '{\n  "allocation_method": "Static",\n  "sku": "Standard"\n}',
      category: "network",
    },
  },
];

function toDraft(t: RawResourceTemplate): Draft {
  return {
    name: t.name,
    title: t.title,
    resource_type: t.resource_type,
    description: t.description,
    target_folder: t.target_folder,
    values: JSON.stringify(t.default_values ?? {}, null, 2),
    category: t.category ?? "",
    min_version: t.min_version ?? "",
    max_version: t.max_version ?? "",
  };
}

interface Props {
  onDenied: () => void;
  onHelp?: () => void;
  /** Which templates: the global ones (default) or a project's own (D-060). */
  paths?: CatalogPaths;
  onChanged?: () => void;
}

export function RawResourcesPanel({ onDenied, onHelp, paths = GLOBAL, onChanged }: Props) {
  const project = paths.scope === "project";
  const [rows, setRows] = useState<RawResourceTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<{ draft: Draft; isNew: boolean } | null>(null);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const { providers, categories } = useProviders();

  const load = useCallback(async () => {
    const r = await call<RawResourceTemplate[]>(paths.resources);
    // A project manages only its own templates; the global ones are the admin's.
    setRows((r.data ?? []).filter((t) => !project || t.scope === "project"));
    setLoading(false);
  }, [paths.resources, project]);

  useEffect(() => {
    load();
  }, [load]);

  const start = (draft: Draft, isNew: boolean) => {
    setMsg(null);
    setEditing({ draft, isNew });
  };

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
      provider: providerOfType(d.resource_type.trim()),
      category: d.category,
      min_version: d.min_version.trim(),
      max_version: d.max_version.trim(),
    };
    setBusy(true);
    const r = editing.isNew
      ? await call(paths.resourcesManage, { method: "POST", json: body })
      : await call(`${paths.resourcesManage}/${encodeURIComponent(d.name)}`, { method: "PUT", json: body });
    setBusy(false);
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setMsg({ tone: "ok", text: editing.isNew ? "Raw resource added. It is in the palette now." : "Raw resource saved." });
    setEditing(null);
    load();
    onChanged?.();
  };

  const remove = async (t: RawResourceTemplate) => {
    if (!confirm(`Delete "${t.title}"? Resources developers already placed on a canvas keep working.`)) return;
    const r = await call(`${paths.resourcesManage}/${encodeURIComponent(t.name)}`, { method: "DELETE" });
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setMsg({ tone: "ok", text: "Raw resource deleted." });
    load();
    onChanged?.();
  };

  const set = (k: keyof Draft, v: string) => setEditing((e) => (e ? { ...e, draft: { ...e.draft, [k]: v } } : e));
  const valuesCheck = editing ? parseValues(editing.draft.values) : null;

  return (
    <div>
      <PanelHeader
        title={project ? "Project raw resources" : "Raw resources"}
        intro="Plain provider blocks, such as azurerm_resource_group, for the one-off things no module covers. Prefer a module when one exists: it has a schema, so the form can check the values."
        actions={
          <>
            {onHelp && <HelpButton onClick={onHelp} label="Docs" />}
            <button className="tf-btn" onClick={() => start(EMPTY, true)}>
              <Plus className="size-4" /> Add raw resource
            </button>
          </>
        }
      />
      {msg && <Notice tone={msg.tone} onClose={() => setMsg(null)}>{msg.text}</Notice>}

      {editing && (
        <Card className="mb-6 max-w-2xl">
          <h3 className="m-0 mb-3 text-base font-semibold">{editing.isNew ? "New raw resource" : `Edit ${editing.draft.name}`}</h3>
          {editing.isNew && (
            <div className="mb-4">
              <p className="m-0 mb-1.5 text-xs text-[var(--tf-color-text-muted)]">Start from a common one:</p>
              <div className="flex flex-wrap gap-1.5">
                {PRESETS.map((p) => (
                  <button key={p.label} type="button" className="rounded-full border border-[var(--tf-color-border)] px-3 py-1 text-xs hover:border-[var(--tf-color-action)]" onClick={() => start(p.draft, true)}>
                    {p.label}
                  </button>
                ))}
              </div>
            </div>
          )}
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="text-sm font-medium">
              Name
              <input className={`${inputCls} font-mono`} value={editing.draft.name} disabled={!editing.isNew} placeholder="resource-group" onChange={(e) => set("name", e.target.value.toLowerCase())} />
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">Lower case, no spaces. Cannot change later.</span>
            </label>
            <label className="text-sm font-medium">
              Title
              <input className={inputCls} value={editing.draft.title} maxLength={80} placeholder="Resource group" onChange={(e) => set("title", e.target.value)} />
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">What people see in the palette.</span>
            </label>
            <label className="text-sm font-medium sm:col-span-2">
              Resource type
              <input className={`${inputCls} font-mono`} value={editing.draft.resource_type} placeholder="azurerm_resource_group" onChange={(e) => set("resource_type", e.target.value)} />
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">The exact provider type. It is written as resource &quot;&lt;type&gt;&quot; &quot;&lt;name&gt;&quot; {"{}"}.</span>
            </label>
            <label className="text-sm font-medium">
              Category
              <select className={inputCls} value={editing.draft.category} onChange={(e) => set("category", e.target.value)}>
                <option value="">No category</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.label}
                  </option>
                ))}
              </select>
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">Where it appears in the palette.</span>
            </label>
            <div className="text-sm font-medium">
              Provider
              <p className="m-0 mt-1 rounded-lg border border-dashed border-[var(--tf-color-border-strong)] px-3 py-2 text-sm font-normal">
                {providerOfType(editing.draft.resource_type.trim()) || "from the resource type"}
                {providerOfType(editing.draft.resource_type.trim()) && !providers.some((p) => p.id === providerOfType(editing.draft.resource_type.trim())) && (
                  <span className="ml-2 text-xs text-[var(--tf-color-warning)]">not registered under Cloud providers yet</span>
                )}
              </p>
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">Taken from the resource type.</span>
            </div>
            <label className="text-sm font-medium">
              Available from provider version <span className="font-normal text-[var(--tf-color-text-muted)]">(optional)</span>
              <input className={`${inputCls} font-mono`} value={editing.draft.min_version} placeholder="3.0" onChange={(e) => set("min_version", e.target.value)} />
            </label>
            <label className="text-sm font-medium">
              Until provider version <span className="font-normal text-[var(--tf-color-text-muted)]">(optional)</span>
              <input className={`${inputCls} font-mono`} value={editing.draft.max_version} placeholder="4.9" onChange={(e) => set("max_version", e.target.value)} />
            </label>
            <label className="text-sm font-medium sm:col-span-2">
              Description <span className="font-normal text-[var(--tf-color-text-muted)]">(optional)</span>
              <input className={inputCls} value={editing.draft.description} maxLength={500} placeholder="When should a developer use this?" onChange={(e) => set("description", e.target.value)} />
            </label>
            <label className="text-sm font-medium sm:col-span-2">
              Default folder <span className="font-normal text-[var(--tf-color-text-muted)]">(optional)</span>
              <input className={`${inputCls} font-mono sm:max-w-xs`} value={editing.draft.target_folder} placeholder="common" onChange={(e) => set("target_folder", e.target.value)} />
            </label>
            <label className="text-sm font-medium sm:col-span-2">
              Default arguments (JSON)
              <textarea className={`${inputCls} font-mono text-xs`} rows={7} spellCheck={false} value={editing.draft.values} onChange={(e) => set("values", e.target.value)} />
              <span className={`mt-1 block text-xs font-normal ${valuesCheck && "error" in valuesCheck ? "text-[var(--tf-color-error)]" : "text-[var(--tf-color-text-muted)]"}`}>
                {valuesCheck && "error" in valuesCheck ? valuesCheck.error : "The block starts with these arguments; people can change them. Use {\"$expr\": \"var.location\"} for a reference."}
              </span>
            </label>
          </div>
          <div className="mt-4 flex gap-2">
            <button className="tf-btn" disabled={busy || !editing.draft.name || !editing.draft.title || !editing.draft.resource_type.trim()} onClick={save}>
              {busy && <Loader2 className="size-4 animate-spin" />} Save
            </button>
            <button className="tf-btn tf-btn--ghost" onClick={() => setEditing(null)}>
              Cancel
            </button>
          </div>
        </Card>
      )}

      {loading ? (
        <p className="text-sm text-[var(--tf-color-text-muted)]">Loading…</p>
      ) : rows.length === 0 && !editing ? (
        <EmptyState
          icon={<Shapes className="size-5" />}
          title="No raw resources yet"
          action={
            <button className="tf-btn" onClick={() => start(PRESETS[0].draft, true)}>
              <Plus className="size-4" /> Add the first one
            </button>
          }
        >
          Add one when developers need a provider block that has no module. Start from a common resource and change it.
        </EmptyState>
      ) : (
        <ul className="m-0 grid list-none gap-3 p-0 sm:grid-cols-2">
          {rows.map((t) => (
            <li key={t.name}>
              <Card className="flex h-full flex-col">
                <p className="m-0 font-semibold">{t.title}</p>
                <p className="m-0 font-mono text-xs text-[var(--tf-color-text-muted)]">{t.resource_type}</p>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {t.provider && <Chip tone="info">{providers.find((p) => p.id === t.provider)?.name ?? t.provider}</Chip>}
                  {t.category && <Chip>{categories.find((c) => c.id === t.category)?.label ?? t.category}</Chip>}
                  {(t.min_version || t.max_version) && <Chip>{`provider ${t.min_version || "any"} to ${t.max_version || "any"}`}</Chip>}
                </div>
                {t.description && <p className="m-0 mt-2 text-xs text-[var(--tf-color-text-secondary)]">{t.description}</p>}
                <p className="m-0 mt-2 text-xs text-[var(--tf-color-text-muted)]">
                  Default folder: <code>{t.target_folder || "first folder"}</code>
                </p>
                <div className="mt-auto flex gap-2 pt-3">
                  <button className="tf-btn tf-btn--ghost" onClick={() => start(toDraft(t), false)}>
                    <Pencil className="size-4" /> Edit
                  </button>
                  <button className="tf-btn tf-btn--ghost" style={{ color: "var(--tf-color-error)" }} onClick={() => remove(t)}>
                    <Trash2 className="size-4" /> Delete
                  </button>
                </div>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
