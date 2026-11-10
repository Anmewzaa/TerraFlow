import { useCallback, useEffect, useMemo, useState } from "react";
import { Boxes, CheckCircle2, FileJson, FlaskConical, Loader2, Plus, Search, Upload, XCircle } from "lucide-react";
import { call } from "./api";
import { Card, Chip, EmptyState, HelpButton, inputCls, Notice, PanelHeader, Pills } from "./ui";
import { EXAMPLE_NAME, EXAMPLE_SCHEMA } from "./helpContent";

interface ModuleRow {
  name: string;
  version: string;
  title: string;
  status: "published" | "deprecated";
  target_folder: string;
  imported_by: string;
  updated_at: string;
}
interface Issue {
  path: string;
  code: string;
  message: string;
}
export interface ImportCheck {
  report: { valid: boolean; errors?: Issue[]; warnings?: Issue[]; name?: string; version?: string };
  exists: "none" | "identical" | "different";
  compared_to?: string;
  changes?: { kind: string; path: string; detail: string; breaking: boolean }[];
  breaking_changes?: number;
}

type Filter = "all" | "published" | "deprecated";

/** The catalog name a schema most likely wants: its module source, else the file name. */
export function suggestName(schemaText: string, fallback = ""): string {
  try {
    const src = (JSON.parse(schemaText) as { "x-tf-module"?: { source?: string } })["x-tf-module"]?.source;
    if (src) return src.split("/").pop()!.toLowerCase();
  } catch {
    /* not JSON yet */
  }
  return fallback;
}

const STEPS = ["Schema", "Check", "Publish"];

function Stepper({ step }: { step: number }) {
  return (
    <ol className="m-0 mb-5 flex list-none items-center gap-2 p-0 text-xs">
      {STEPS.map((s, i) => (
        <li key={s} className="flex items-center gap-2">
          <span
            className={`flex size-5 items-center justify-center rounded-full font-bold ${
              i + 1 <= step ? "bg-[var(--tf-color-action)] text-[var(--tf-color-on-action)]" : "bg-[var(--tf-glass-strong)] text-[var(--tf-color-text-muted)]"
            }`}
          >
            {i + 1}
          </span>
          <span className={i + 1 === step ? "font-semibold" : "text-[var(--tf-color-text-muted)]"}>{s}</span>
          {i < STEPS.length - 1 && <span className="mx-1 h-px w-6 bg-[var(--tf-color-border)]" />}
        </li>
      ))}
    </ol>
  );
}

function CheckReport({ check }: { check: ImportCheck }) {
  const { report } = check;
  const errors = report.errors ?? [];
  const warnings = report.warnings ?? [];
  return (
    <div className="rounded-xl border border-[var(--tf-color-border)] p-4 text-sm">
      <p className="m-0 flex items-center gap-2 font-medium">
        {report.valid ? (
          <CheckCircle2 className="size-4 text-[var(--tf-color-success)]" />
        ) : (
          <XCircle className="size-4 text-[var(--tf-color-error)]" />
        )}
        {report.valid ? "The schema is valid" : `${errors.length} ${errors.length === 1 ? "problem" : "problems"} to fix`}
        {report.name && report.version ? ` — ${report.name}@${report.version}` : ""}
      </p>
      <p className="m-0 mt-1 text-xs text-[var(--tf-color-text-secondary)]">
        {check.exists === "identical" && "This exact version is already published, so importing changes nothing."}
        {check.exists === "different" && "This version already exists with different content and cannot be replaced. Publish a new version (change x-tf-module.version)."}
        {check.exists === "none" && "This is a new version."}
      </p>
      {errors.length > 0 && (
        <ul className="m-0 mt-3 list-none space-y-1.5 p-0">
          {errors.map((e, i) => (
            <li key={`e${i}`} className="rounded-md bg-[var(--tf-color-error-surface)] px-2.5 py-1.5 text-xs text-[var(--tf-color-error)]">
              <code className="font-semibold">{e.path || "/"}</code> {e.message}
            </li>
          ))}
        </ul>
      )}
      {warnings.length > 0 && (
        <ul className="m-0 mt-3 list-none space-y-1.5 p-0">
          {warnings.map((e, i) => (
            <li key={`w${i}`} className="rounded-md bg-[var(--tf-glass-strong)] px-2.5 py-1.5 text-xs text-[var(--tf-color-text-secondary)]">
              <span className="font-semibold">Tip </span>
              <code>{e.path || "/"}</code> {e.message}
            </li>
          ))}
        </ul>
      )}
      {(check.changes ?? []).length > 0 && (
        <div className="mt-3">
          <p className="m-0 text-xs font-semibold">
            Changes since {check.compared_to}: {check.breaking_changes ?? 0} breaking
          </p>
          <ul className="m-0 mt-1 list-disc pl-5 text-xs">
            {check.changes!.map((c, i) => (
              <li key={i} className={c.breaking ? "text-[var(--tf-color-error)]" : undefined}>
                {c.kind} <code>{c.path}</code> {c.detail}
                {c.breaking ? " (breaking)" : ""}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

export function ModulesPanel({ onDenied, onHelp, exampleTick }: { onDenied: () => void; onHelp: () => void; exampleTick: number }) {
  const [rows, setRows] = useState<ModuleRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [schema, setSchema] = useState("");
  const [check, setCheck] = useState<ImportCheck | null>(null);
  const [busy, setBusy] = useState(false);
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<Filter>("all");

  const load = useCallback(async () => {
    const r = await call<ModuleRow[]>("/admin/catalog/modules");
    if (r.status === 403) return onDenied();
    if (!r.ok) setMsg({ tone: "error", text: r.error });
    setRows(r.data ?? []);
    setLoading(false);
  }, [onDenied]);

  useEffect(() => {
    load();
  }, [load]);

  const useExample = useCallback(() => {
    setAdding(true);
    setSchema(JSON.stringify(EXAMPLE_SCHEMA, null, 2));
    setName(EXAMPLE_NAME);
    setCheck(null);
    setMsg(null);
  }, []);

  // The help drawer's "Use the example" button asks for the example through this counter.
  useEffect(() => {
    if (exampleTick > 0) useExample();
  }, [exampleTick, useExample]);

  const parsed = useMemo((): { ok: true; value: unknown } | { ok: false; error: string } | null => {
    if (!schema.trim()) return null;
    try {
      return { ok: true, value: JSON.parse(schema) };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : "not valid JSON" };
    }
  }, [schema]);

  const run = async (publish: boolean) => {
    if (!parsed?.ok) return;
    setBusy(true);
    setMsg(null);
    if (!publish) {
      const r = await call<ImportCheck>("/admin/catalog/validate", { method: "POST", json: { name, schema: parsed.value } });
      setBusy(false);
      if (r.status === 403) return onDenied();
      if (!r.ok) return setMsg({ tone: "error", text: r.error });
      return setCheck(r.data);
    }
    const r = await call<{ status: string; result?: ImportCheck }>("/admin/catalog/modules", { method: "POST", json: { name, schema: parsed.value } });
    setBusy(false);
    if (r.status === 403) return onDenied();
    if (r.status === 422 && r.data?.result) setCheck(r.data.result);
    if (!r.ok) return setMsg({ tone: "error", text: r.error || "Import failed" });
    setMsg({ tone: "ok", text: r.data?.status === "unchanged" ? "Already published, nothing changed." : `${name} is published. People can use it from the palette now.` });
    setAdding(false);
    setSchema("");
    setName("");
    setCheck(null);
    load();
  };

  const setStatus = async (m: ModuleRow, status: ModuleRow["status"]) => {
    const r = await call(`/admin/catalog/modules/${encodeURIComponent(m.name)}/${encodeURIComponent(m.version)}`, { method: "PATCH", json: { status } });
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    load();
  };

  const onFile = async (f: File | undefined) => {
    if (!f) return;
    const text = await f.text();
    setSchema(text);
    setCheck(null);
    setName(suggestName(text, f.name.replace(/\.json$/i, "").toLowerCase()));
  };

  const shown = rows.filter(
    (m) => (filter === "all" || m.status === filter) && (!q.trim() || `${m.name} ${m.title}`.toLowerCase().includes(q.trim().toLowerCase())),
  );
  const step = check ? (check.report.valid ? 3 : 2) : 1;

  return (
    <div>
      <PanelHeader
        title="Modules"
        intro="Terraform modules your team has agreed to use. People drag them onto the canvas and fill the form that TerraFlow draws from each module's schema."
        actions={
          <>
            <HelpButton onClick={onHelp} label="Docs" />
            {!adding && (
              <button className="tf-btn" onClick={() => { setAdding(true); setMsg(null); }}>
                <Plus className="size-4" /> Add a module
              </button>
            )}
          </>
        }
      />
      {msg && <Notice tone={msg.tone} onClose={() => setMsg(null)}>{msg.text}</Notice>}

      {adding && (
        <Card className="mb-6">
          <div className="mb-1 flex items-center justify-between">
            <h3 className="m-0 text-base font-semibold">Add a module</h3>
            <button className="text-xs underline" onClick={() => { setAdding(false); setCheck(null); }}>
              Cancel
            </button>
          </div>
          <Stepper step={step} />

          <div className="mb-4 grid gap-2 sm:grid-cols-3">
            <button type="button" onClick={useExample} className="flex items-start gap-3 rounded-xl border border-[var(--tf-color-border)] p-3 text-left hover:border-[var(--tf-color-action)]">
              <FlaskConical className="mt-0.5 size-5 shrink-0 text-[var(--tf-color-action)]" />
              <span>
                <span className="block text-sm font-semibold">Start from an example</span>
                <span className="text-xs text-[var(--tf-color-text-muted)]">A complete schema that already passes the check. Best for the first time.</span>
              </span>
            </button>
            <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-[var(--tf-color-border)] p-3 hover:border-[var(--tf-color-action)]">
              <Upload className="mt-0.5 size-5 shrink-0 text-[var(--tf-color-action)]" />
              <span>
                <span className="block text-sm font-semibold">Upload a .json file</span>
                <span className="text-xs text-[var(--tf-color-text-muted)]">A schema file from your module repository.</span>
              </span>
              <input type="file" accept="application/json,.json" className="sr-only" onChange={(e) => onFile(e.target.files?.[0])} />
            </label>
            <div className="flex items-start gap-3 rounded-xl border border-[var(--tf-color-border)] p-3">
              <FileJson className="mt-0.5 size-5 shrink-0 text-[var(--tf-color-action)]" />
              <span>
                <span className="block text-sm font-semibold">Paste the JSON</span>
                <span className="text-xs text-[var(--tf-color-text-muted)]">Use the box below.</span>
              </span>
            </div>
          </div>

          <label className="block text-sm font-medium">
            Catalog name
            <input
              value={name}
              onChange={(e) => { setName(e.target.value.toLowerCase()); setCheck(null); }}
              placeholder="e.g. storage-account"
              className={`${inputCls} max-w-sm font-mono`}
            />
            <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">Lower case letters, digits and dashes. This is how people and the generator refer to the module.</span>
          </label>

          <label className="mt-3 block text-sm font-medium">
            Schema (JSON)
            <textarea
              value={schema}
              onChange={(e) => {
                setSchema(e.target.value);
                setCheck(null);
                if (!name) setName(suggestName(e.target.value));
              }}
              rows={12}
              spellCheck={false}
              placeholder="Paste a schema here, or start from an example above."
              className={`${inputCls} font-mono text-xs`}
            />
          </label>
          {parsed && (
            <p className={`m-0 mt-1 text-xs ${parsed.ok ? "text-[var(--tf-color-success)]" : "text-[var(--tf-color-error)]"}`}>
              {parsed.ok ? "Valid JSON." : `Not valid JSON yet: ${parsed.error}`}
            </p>
          )}

          <div className="mt-4 flex flex-wrap gap-2">
            <button className="tf-btn tf-btn--ghost" disabled={busy || !name || !parsed?.ok} onClick={() => run(false)}>
              {busy && <Loader2 className="size-4 animate-spin" />} Check
            </button>
            <button className="tf-btn" disabled={busy || !name || !check?.report.valid || check.exists === "different"} onClick={() => run(true)}>
              Import and publish
            </button>
            {!check && <span className="self-center text-xs text-[var(--tf-color-text-muted)]">Check first; publishing unlocks when the schema is valid.</span>}
          </div>
          {check && <div className="mt-4"><CheckReport check={check} /></div>}
        </Card>
      )}

      {loading ? (
        <p className="text-sm text-[var(--tf-color-text-muted)]">Loading…</p>
      ) : rows.length === 0 ? (
        <EmptyState
          icon={<Boxes className="size-5" />}
          title="No modules yet"
          action={
            <button className="tf-btn" onClick={useExample}>
              <FlaskConical className="size-4" /> Start from an example
            </button>
          }
        >
          A module gives people a ready-made form for something your team builds again and again. Start from the example to see how a schema
          looks, or open Docs for every key.
        </EmptyState>
      ) : (
        <>
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <Pills
              value={filter}
              onChange={setFilter}
              items={[
                { id: "all", label: "All", count: rows.length },
                { id: "published", label: "Published", count: rows.filter((r) => r.status === "published").length },
                { id: "deprecated", label: "Deprecated", count: rows.filter((r) => r.status === "deprecated").length },
              ]}
            />
            <div className="relative w-full max-w-xs">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
              <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search modules" aria-label="Search modules" className={`${inputCls} mt-0 pl-9`} />
            </div>
          </div>
          <ul className="m-0 grid list-none gap-3 p-0 sm:grid-cols-2">
            {shown.length === 0 && <li className="col-span-full py-6 text-center text-sm text-[var(--tf-color-text-muted)]">No module matches.</li>}
            {shown.map((m) => (
              <li key={`${m.name}@${m.version}`}>
                <Card className="flex h-full flex-col">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="m-0 truncate font-semibold">{m.title || m.name}</p>
                      <p className="m-0 font-mono text-xs text-[var(--tf-color-text-muted)]">
                        {m.name}@{m.version}
                      </p>
                    </div>
                    <Chip tone={m.status === "published" ? "good" : "neutral"}>{m.status}</Chip>
                  </div>
                  <p className="m-0 mt-3 text-xs text-[var(--tf-color-text-secondary)]">
                    Goes to <code>{m.target_folder || "the first folder"}</code> · added by {m.imported_by || "seed"}
                  </p>
                  <div className="mt-auto pt-3">
                    {m.status === "published" ? (
                      <button className="tf-btn tf-btn--ghost" onClick={() => setStatus(m, "deprecated")}>
                        Deprecate
                      </button>
                    ) : (
                      <button className="tf-btn tf-btn--ghost" onClick={() => setStatus(m, "published")}>
                        Publish again
                      </button>
                    )}
                  </div>
                </Card>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
