import { confirmDialog } from "../../lib/notify";
import { useState } from "react";
import { Cloud, Loader2, Pencil, Plus, Trash2 } from "lucide-react";
import { useProviders } from "../../hooks/useProviders";
import type { RegisteredProvider } from "../../lib/provider";
import { call } from "./api";
import { Card, EmptyState, HelpButton, inputCls, Notice, PanelHeader } from "./ui";
import { PanelLoading } from "@/components/ui/skeleton";

/* Cloud providers as data (D-058): a name, a Terraform source, the versions a team picks from, a logo
 * and the block written to providers.tf. TerraFlow never calls a cloud; this only describes one. */

const MAX_LOGO = 40 << 10;

interface Draft {
  id: string;
  name: string;
  source: string;
  logo: string;
  versions: string;
  block: string;
}
const EMPTY: Draft = { id: "", name: "", source: "", logo: "", versions: "", block: "" };

const toDraft = (p: RegisteredProvider): Draft => ({ ...p, versions: p.versions.join(", ") });
const versionList = (s: string) => s.split(",").map((v) => v.trim()).filter(Boolean);

/** A provider's logo, or its initial when it has none. */
export function ProviderLogo({ provider, size = 28 }: { provider: { name: string; logo?: string }; size?: number }) {
  return provider.logo ? (
    <img src={provider.logo} alt="" width={size} height={size} className="shrink-0 rounded-md object-contain" />
  ) : (
    <span style={{ width: size, height: size }} className="flex shrink-0 items-center justify-center rounded-md bg-[var(--tf-glass-strong)] text-xs font-bold">
      {provider.name.slice(0, 1).toUpperCase()}
    </span>
  );
}

export function ProvidersPanel({ onDenied, onHelp }: { onDenied: () => void; onHelp: () => void }) {
  const { providers, loading, reload } = useProviders();
  const [editing, setEditing] = useState<{ draft: Draft; isNew: boolean } | null>(null);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  const set = (k: keyof Draft, v: string) => setEditing((e) => (e ? { ...e, draft: { ...e.draft, [k]: v } } : e));

  const onLogo = (f: File | undefined) => {
    if (!f) return;
    if (!/^image\/(svg\+xml|png)$/.test(f.type)) return setMsg({ tone: "error", text: "The logo must be an SVG or PNG file." });
    if (f.size > MAX_LOGO) return setMsg({ tone: "error", text: "The logo is too large (at most 40 KB)." });
    const r = new FileReader();
    r.onload = () => set("logo", String(r.result));
    r.readAsDataURL(f);
  };

  const save = async () => {
    if (!editing) return;
    const d = editing.draft;
    setBusy(true);
    setMsg(null);
    const body = { id: d.id, name: d.name, source: d.source.trim(), logo: d.logo, versions: versionList(d.versions), block: d.block };
    const r = editing.isNew ? await call("/admin/providers", { method: "POST", json: body }) : await call(`/admin/providers/${encodeURIComponent(d.id)}`, { method: "PUT", json: body });
    setBusy(false);
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setMsg({ tone: "ok", text: editing.isNew ? "Provider added." : "Provider saved." });
    setEditing(null);
    reload();
  };

  const remove = async (p: RegisteredProvider) => {
    if (!(await confirmDialog(`Delete the provider "${p.name}"? Projects that chose it keep their setting but get no providers.tf.`))) return;
    const r = await call(`/admin/providers/${encodeURIComponent(p.id)}`, { method: "DELETE" });
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    setMsg({ tone: "ok", text: "Provider deleted." });
    reload();
  };

  return (
    <div>
      <PanelHeader
        title="Cloud providers"
        intro="The providers your teams can build on, with the versions they may pick. Raw resources belong to a provider, and a project only sees the resources that fit the provider and version it chose."
        actions={
          <>
            <HelpButton onClick={onHelp} label="Docs" />
            <button className="tf-btn" onClick={() => { setMsg(null); setEditing({ draft: EMPTY, isNew: true }); }}>
              <Plus className="size-4" /> Add provider
            </button>
          </>
        }
      />
      {msg && <Notice tone={msg.tone} onClose={() => setMsg(null)}>{msg.text}</Notice>}

      {editing && (
        <Card className="mb-6 max-w-2xl">
          <h3 className="m-0 mb-3 text-base font-semibold">{editing.isNew ? "New provider" : `Edit ${editing.draft.name}`}</h3>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="text-sm font-medium">
              Id
              <input className={`${inputCls} font-mono`} value={editing.draft.id} disabled={!editing.isNew} placeholder="aws" onChange={(e) => set("id", e.target.value.toLowerCase())} />
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">The prefix of its resource types (aws_s3_bucket). Cannot change later.</span>
            </label>
            <label className="text-sm font-medium">
              Name
              <input className={inputCls} value={editing.draft.name} maxLength={60} placeholder="Amazon Web Services" onChange={(e) => set("name", e.target.value)} />
            </label>
            <label className="text-sm font-medium sm:col-span-2">
              Terraform source
              <input className={`${inputCls} font-mono`} value={editing.draft.source} placeholder="hashicorp/aws" onChange={(e) => set("source", e.target.value)} />
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">Written as <code>source</code> in required_providers.</span>
            </label>
            <label className="text-sm font-medium sm:col-span-2">
              Versions people can pick
              <input className={`${inputCls} font-mono`} value={editing.draft.versions} placeholder="~> 5.0, ~> 4.0" onChange={(e) => set("versions", e.target.value)} />
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">Terraform constraints separated by commas, newest first. A project may also type its own.</span>
            </label>
            <div className="sm:col-span-2">
              <p className="m-0 text-sm font-medium">Logo</p>
              <div className="mt-1 flex items-center gap-3">
                <ProviderLogo provider={{ name: editing.draft.name || "?", logo: editing.draft.logo }} size={36} />
                <input type="file" accept="image/svg+xml,image/png" aria-label="Logo file" className="text-sm" onChange={(e) => onLogo(e.target.files?.[0])} />
                {editing.draft.logo && (
                  <button type="button" className="text-xs underline" onClick={() => set("logo", "")}>
                    Remove
                  </button>
                )}
              </div>
              <span className="mt-1 block text-xs text-[var(--tf-color-text-muted)]">SVG or PNG, at most 40 KB. Shown as an image only; nothing is loaded from another site.</span>
            </div>
            <label className="text-sm font-medium sm:col-span-2">
              Provider block <span className="font-normal text-[var(--tf-color-text-muted)]">(optional)</span>
              <textarea className={`${inputCls} font-mono text-xs`} rows={4} spellCheck={false} value={editing.draft.block} placeholder={'provider "aws" {\n  region = "us-east-1"\n}'} onChange={(e) => set("block", e.target.value)} />
              <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">Written to providers.tf of a new folder, after required_providers. Azure needs <code>features {"{}"}</code>.</span>
            </label>
          </div>
          <div className="mt-4 flex gap-2">
            <button className="tf-btn" disabled={busy || !editing.draft.id || !editing.draft.name || !editing.draft.source} onClick={save}>
              {busy && <Loader2 className="size-4 animate-spin" />} Save
            </button>
            <button className="tf-btn tf-btn--ghost" onClick={() => setEditing(null)}>
              Cancel
            </button>
          </div>
        </Card>
      )}

      {loading ? (
        <PanelLoading rows={4} />
      ) : providers.length === 0 && !editing ? (
        <EmptyState icon={<Cloud className="size-5" />} title="No providers yet" action={<button className="tf-btn" onClick={() => setEditing({ draft: EMPTY, isNew: true })}><Plus className="size-4" /> Add the first one</button>}>
          Register the clouds your teams use. Each one needs only a name, its Terraform source and the versions to offer.
        </EmptyState>
      ) : (
        <ul className="m-0 grid list-none gap-3 p-0 sm:grid-cols-2">
          {providers.map((p) => (
            <li key={p.id}>
              <Card className="flex h-full flex-col">
                <div className="flex items-center gap-3">
                  <ProviderLogo provider={p} size={36} />
                  <div className="min-w-0">
                    <p className="m-0 truncate font-semibold">{p.name}</p>
                    <p className="m-0 font-mono text-xs text-[var(--tf-color-text-muted)]">
                      {p.id} · {p.source}
                    </p>
                  </div>
                </div>
                <p className="m-0 mt-3 text-xs text-[var(--tf-color-text-secondary)]">Versions: {p.versions.length ? p.versions.join(", ") : "none listed"}</p>
                <div className="mt-auto flex gap-2 pt-3">
                  <button className="tf-btn tf-btn--ghost" onClick={() => { setMsg(null); setEditing({ draft: toDraft(p), isNew: false }); }}>
                    <Pencil className="size-4" /> Edit
                  </button>
                  <button className="tf-btn tf-btn--ghost" style={{ color: "var(--tf-color-error)" }} onClick={() => remove(p)}>
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
