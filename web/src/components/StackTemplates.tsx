import { useEffect, useMemo, useState } from "react";
import { Layers, Loader2, Trash2, X } from "lucide-react";
import {
  StackError,
  parameterChoices,
  type ApplyBody,
  type StackDoc,
  type StackParam,
  type StackSummary,
} from "../hooks/useStackTemplates";
import type { CanvasNode, CanvasState } from "../types/changeset";
import type { ProjectGraph } from "../types/graph";

const input =
  "rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]";

function Dialog({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--tf-color-overlay)] p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label={title}>
      <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-6 shadow-2xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="m-0 text-lg font-semibold">{title}</h2>
          <button type="button" onClick={onClose} aria-label="Close" className="text-[var(--tf-color-text-muted)]">
            <X className="size-5" />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

function Problems({ error }: { error: Error | null }) {
  if (!error) return null;
  const list = error instanceof StackError ? error.problems : [];
  return (
    <div role="alert" className="mt-3 rounded-lg border border-[var(--tf-color-error)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
      {error.message}
      {list.length > 0 && (
        <ul className="m-0 mt-1 list-disc pl-5">
          {list.map((p, i) => (
            <li key={i}>{p}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** Make a template from the picked resources: a name, and the arguments that become parameters. */
export function CreateStackDialog({ canvas, picked, onCreate, onClose }: {
  canvas: CanvasState;
  picked: string[];
  onCreate: (name: string, selected: string[], params: StackParam[]) => Promise<void>;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [params, setParams] = useState<StackParam[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const nodes = useMemo(() => canvas.nodes.filter((n) => picked.includes(n.id)), [canvas, picked]);
  const choices = useMemo(() => parameterChoices(nodes), [nodes]);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onCreate(name.trim(), picked, params.filter((p) => p.name && p.node && p.field));
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err : new Error("Could not make the template"));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog title="Create stack template" onClose={onClose}>
      <form onSubmit={submit}>
        <p className="mt-0 text-sm text-[var(--tf-color-text-secondary)]">
          {nodes.length} resource{nodes.length === 1 ? "" : "s"}: {nodes.map((n) => n.instance_name).join(", ")}. What they pointed at outside
          this group becomes a slot to fill when the template is applied.
        </p>
        <label className="mb-1 block text-sm" htmlFor="stack-name">Template name</label>
        <input id="stack-name" autoFocus required maxLength={80} className={`${input} w-full`} value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Private endpoint for a key vault" />
        <p className="mb-1 mt-4 text-sm font-medium">Parameters (optional)</p>
        <p className="m-0 mb-2 text-xs text-[var(--tf-color-text-muted)]">
          An argument that changes between uses becomes a project variable (<code>var.name</code>) with a value per environment. A secret is never a parameter.
        </p>
        {params.map((p, i) => (
          <div key={i} className="mb-2 flex flex-wrap items-center gap-2">
            <select aria-label="Argument" className={`${input} min-w-0 flex-1`} value={`${p.node}\u0000${p.field}`}
              onChange={(e) => { const [node, field] = e.target.value.split("\u0000"); setParams(params.map((x, j) => (j === i ? { ...x, node, field } : x))); }}>
              <option value="\u0000">Choose an argument…</option>
              {choices.map((c) => (
                <option key={`${c.node}-${c.field}`} value={`${c.node}\u0000${c.field}`}>{c.node} · {c.field} = {c.value}</option>
              ))}
            </select>
            <input aria-label="Variable name" className={`${input} w-36 font-mono text-xs`} placeholder="variable_name" value={p.name}
              onChange={(e) => setParams(params.map((x, j) => (j === i ? { ...x, name: e.target.value.trim() } : x)))} />
            <button type="button" className="text-xs underline" onClick={() => setParams(params.filter((_, j) => j !== i))}>Remove</button>
          </div>
        ))}
        <button type="button" className="rounded-md border border-[var(--tf-color-border-strong)] px-3 py-1.5 text-sm" onClick={() => setParams([...params, { name: "", node: "", field: "" }])}>
          Add a parameter
        </button>
        <Problems error={error} />
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className="tf-btn tf-btn--ghost" onClick={onClose}>Cancel</button>
          <button type="submit" className="tf-btn" disabled={busy || !name.trim()}>
            {busy && <Loader2 className="size-4 animate-spin" />} Create template
          </button>
        </div>
      </form>
    </Dialog>
  );
}

/** Apply a template: environments, a prefix or suffix, the slots, and a value per environment for each parameter. */
export function ApplyStackDialog({ summary, load, envs, currentEnv, canvasNodes, graph, onApply, onClose }: {
  summary: StackSummary;
  load: (id: string) => Promise<StackDoc>;
  envs: string[];
  currentEnv: string;
  canvasNodes: CanvasNode[];
  graph: ProjectGraph | null;
  onApply: (body: Omit<ApplyBody, "env" | "offset_x" | "offset_y">, envs: string[]) => Promise<void>;
  onClose: () => void;
}) {
  const [doc, setDoc] = useState<StackDoc | null>(null);
  const [chosen, setChosen] = useState<string[]>([currentEnv]);
  const [prefix, setPrefix] = useState("");
  const [suffix, setSuffix] = useState("");
  const [slots, setSlots] = useState<Record<string, string>>({});
  const [values, setValues] = useState<Record<string, Record<string, string>>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  useEffect(() => {
    let off = false;
    load(summary.id).then((d) => !off && setDoc(d)).catch((e) => !off && setError(e instanceof Error ? e : new Error("Cannot read the template")));
    return () => {
      off = true;
    };
  }, [load, summary.id]);

  if (!doc) {
    return (
      <Dialog title={`Apply ${summary.name}`} onClose={onClose}>
        {error ? <Problems error={error} /> : <Loader2 className="size-5 animate-spin" aria-label="Loading" />}
      </Dialog>
    );
  }
  const hasParentSlot = doc.slots.some((s) => s.kind === "parent");
  // What a slot can be filled with. A parent must be a resource of the draft (a repository resource cannot hold a new one);
  // a reference may also be a module call in the repository.
  const options = (s: StackDoc["slots"][number]) => {
    const fromDraft = canvasNodes.filter((n) => !n.existing && (!s.accepts || n.module_name === s.accepts)).map((n) => ({ value: s.kind === "parent" ? n.id : n.instance_name, label: `${n.instance_name} (draft)` }));
    if (s.kind === "parent") return fromDraft;
    const fromRepo = (graph?.folders ?? []).flatMap((f) => f.nodes.filter((n) => n.kind === "module" && n.label && (!s.accepts || n.type === s.accepts)).map((n) => ({ value: n.label!, label: `${n.label} (${f.name}/)` })));
    return [...fromDraft, ...fromRepo];
  };
  const toggleEnv = (e: string) => setChosen(chosen.includes(e) ? chosen.filter((x) => x !== e) : [...chosen, e]);
  const submit = async (ev: React.FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onApply({ prefix: prefix.trim(), suffix: suffix.trim(), slots, values, draft_labels: [] }, chosen);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err : new Error("Could not apply the template"));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog title={`Apply ${summary.name}`} onClose={onClose}>
      <form onSubmit={submit}>
        <p className="mt-0 text-sm text-[var(--tf-color-text-secondary)]">
          {doc.nodes.length} resource{doc.nodes.length === 1 ? "" : "s"} are added to the draft of each environment you choose. Nothing is written to the repository until the change is reviewed and approved.
        </p>
        <fieldset className="m-0 border-0 p-0">
          <legend className="mb-1 text-sm font-medium">Environments</legend>
          <div className="flex flex-wrap gap-3">
            {envs.map((e) => (
              <label key={e} className="flex items-center gap-1.5 text-sm">
                <input type="checkbox" checked={chosen.includes(e)} disabled={hasParentSlot && e !== currentEnv} onChange={() => toggleEnv(e)} /> {e}
              </label>
            ))}
          </div>
          {hasParentSlot && <p className="m-0 mt-1 text-xs text-[var(--tf-color-text-muted)]">This template sits inside a resource of the draft, so it can only be applied to the environment on the canvas ({currentEnv}).</p>}
        </fieldset>
        <div className="mt-4 grid grid-cols-2 gap-3">
          <label className="text-sm">Name prefix
            <input className={`${input} mt-1 w-full font-mono text-xs`} value={prefix} placeholder="app-" onChange={(e) => setPrefix(e.target.value)} />
          </label>
          <label className="text-sm">Name suffix
            <input className={`${input} mt-1 w-full font-mono text-xs`} value={suffix} placeholder="-01" onChange={(e) => setSuffix(e.target.value)} />
          </label>
        </div>
        {doc.slots.length > 0 && <p className="mb-1 mt-4 text-sm font-medium">Fill the slots</p>}
        {doc.slots.map((s) => {
          const opts = options(s);
          return (
            <label key={s.name} className="mb-2 block text-sm">
              {s.name}{s.accepts ? <span className="text-[var(--tf-color-text-muted)]"> ({s.accepts})</span> : null}
              <select className={`${input} mt-1 w-full`} value={slots[s.name] ?? ""} onChange={(e) => setSlots({ ...slots, [s.name]: e.target.value })}>
                <option value="">{opts.length === 0 ? "Nothing to choose yet: add one to the canvas first" : "Choose…"}</option>
                {opts.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </label>
          );
        })}
        {doc.params.length > 0 && <p className="mb-1 mt-4 text-sm font-medium">Parameters (a value per environment)</p>}
        {doc.params.map((p) => (
          <div key={p.name} className="mb-2">
            <div className="font-mono text-xs">{p.name} <span className="text-[var(--tf-color-text-muted)]">(default {p.default || "none"})</span></div>
            <div className="mt-1 flex flex-wrap gap-2">
              {chosen.map((e) => (
                <input key={e} aria-label={`${p.name} in ${e}`} className={`${input} w-32 font-mono text-xs`} placeholder={`${e}: ${p.default ?? ""}`}
                  value={values[p.name]?.[e] ?? ""} onChange={(ev) => setValues({ ...values, [p.name]: { ...values[p.name], [e]: ev.target.value } })} />
              ))}
            </div>
          </div>
        ))}
        <Problems error={error} />
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className="tf-btn tf-btn--ghost" onClick={onClose}>Cancel</button>
          <button type="submit" className="tf-btn" disabled={busy || chosen.length === 0}>
            {busy && <Loader2 className="size-4 animate-spin" />} Add to the draft
          </button>
        </div>
      </form>
    </Dialog>
  );
}

/** The project's templates, in the palette's third tab. */
export function StackList({ templates, canManage, picking, pickedCount, onStartPicking, onCancelPicking, onApply, onDelete }: {
  templates: StackSummary[];
  canManage: boolean;
  picking: boolean;
  pickedCount: number;
  onStartPicking: () => void;
  onCancelPicking: () => void;
  onApply: (t: StackSummary) => void;
  onDelete: (t: StackSummary) => void;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto p-3 text-sm">
      {canManage && (
        <div className="mb-3 rounded-lg border border-[var(--tf-color-border)] p-3">
          <p className="m-0 font-medium">Make a template</p>
          <p className="m-0 mt-1 text-xs text-[var(--tf-color-text-muted)]">
            {picking ? `Click the resources on the canvas to pick them (${pickedCount} picked), then create.` : "Pick resources on the canvas and save them as a reusable group."}
          </p>
          <button type="button" className="tf-btn mt-2" onClick={onStartPicking}>
            {picking ? "Create from the picked" : "Pick resources"}
          </button>
          {picking && (
            <button type="button" className="tf-btn tf-btn--ghost ml-2" onClick={onCancelPicking}>
              Cancel
            </button>
          )}
        </div>
      )}
      {templates.length === 0 ? (
        <p className="m-0 text-xs text-[var(--tf-color-text-muted)]">No stack templates yet.</p>
      ) : (
        <ul className="m-0 list-none space-y-2 p-0">
          {templates.map((t) => (
            <li key={t.id} className="flex items-center gap-2 rounded-lg border border-[var(--tf-color-border)] p-2">
              <Layers className="size-4 shrink-0 text-[var(--tf-color-text-muted)]" />
              <span className="min-w-0 flex-1 truncate" title={t.name}>{t.name}</span>
              <button type="button" className="tf-btn" onClick={() => onApply(t)}>Apply</button>
              {canManage && (
                <button type="button" aria-label={`Delete ${t.name}`} className="p-1 text-[var(--tf-color-text-muted)]" onClick={() => onDelete(t)}>
                  <Trash2 className="size-4" />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
