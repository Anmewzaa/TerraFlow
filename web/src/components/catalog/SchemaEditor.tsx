import { useEffect, useMemo, useRef, useState } from "react";
import { CheckCircle2, Download, FileCode2, FlaskConical, Loader2, Upload, XCircle } from "lucide-react";
import type { JSONSchema } from "../../types/catalog";
import type { CatalogPaths } from "../../lib/catalogApi";
import { download, exportFile, parseSchema, readImport, suggestName } from "../../lib/schemaEditor";
import { call } from "../admin/api";
import { Card, inputCls, Notice, Pills } from "../admin/ui";
import { EXAMPLE_NAME, EXAMPLE_SCHEMA } from "../admin/helpContent";
import { ErrorBoundary } from "../ErrorBoundary";
import { SchemaForm } from "./SchemaForm";

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

/** Every problem of a schema at once, and what publishing it would change. */
export function CheckReport({ check }: { check: ImportCheck }) {
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
      {report.valid && (
        <p className="m-0 mt-1 text-xs text-[var(--tf-color-text-secondary)]">
          {check.exists === "identical" && "This exact version is already published, so publishing changes nothing."}
          {check.exists === "different" && "This version already exists with different content and cannot be replaced. Publish a new version (change x-tf-module.version)."}
          {check.exists === "none" && "This is a new version."}
        </p>
      )}
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

interface Props {
  paths: CatalogPaths;
  /** A schema to start from (a published version opened to make the next one). */
  initial?: { name: string; schema: string };
  /** Counts up when the Docs page asks for the example. */
  exampleTick?: number;
  onCancel: () => void;
  onPublished: (message: string) => void;
  onDenied: () => void;
}

const CHECK_DELAY_MS = 500;

/**
 * The schema editor (D-060): write or load a module schema, see every problem while typing,
 * try the form it draws, see what changed since the last version, then publish. It works on
 * the global catalog or on one project's own, depending on `paths`.
 */
export function SchemaEditor({ paths, initial, exampleTick = 0, onCancel, onPublished, onDenied }: Props) {
  const [name, setName] = useState(initial?.name ?? "");
  const [schema, setSchema] = useState(initial?.schema ?? "");
  const [check, setCheck] = useState<ImportCheck | null>(null);
  const [checking, setChecking] = useState(false);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);
  const [tab, setTab] = useState<"check" | "preview">("check");
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [tfOpen, setTfOpen] = useState(false);
  const [tfText, setTfText] = useState("");
  const [notes, setNotes] = useState<string[]>([]);
  const seq = useRef(0);

  const parsed = useMemo(() => parseSchema(schema), [schema]);

  const load = (next: { name: string; schema: string }) => {
    setName(next.name);
    setSchema(next.schema);
    setValues({});
    setNotes([]);
    setMsg(null);
  };

  useEffect(() => {
    if (exampleTick > 0) load({ name: EXAMPLE_NAME, schema: JSON.stringify(EXAMPLE_SCHEMA, null, 2) });
  }, [exampleTick]);

  // Live check: a moment after the last key stroke, ask the server for the full report.
  // Only the answer to the latest text is shown.
  useEffect(() => {
    const mine = ++seq.current;
    if (!name || !parsed?.ok) {
      setCheck(null);
      setChecking(false);
      return;
    }
    setChecking(true);
    const timer = setTimeout(async () => {
      const r = await call<ImportCheck>(`${paths.manage}/validate`, { method: "POST", json: { name, schema: parsed.value } });
      if (mine !== seq.current) return;
      setChecking(false);
      if (r.status === 403) return onDenied();
      if (!r.ok) {
        setCheck(null);
        return setMsg({ tone: "error", text: r.error });
      }
      setCheck(r.data);
    }, CHECK_DELAY_MS);
    return () => clearTimeout(timer);
  }, [name, parsed, paths.manage, onDenied]);

  const publish = async () => {
    if (!parsed?.ok) return;
    setBusy(true);
    setMsg(null);
    const r = await call<{ status: string; result?: ImportCheck }>(`${paths.manage}/modules`, { method: "POST", json: { name, schema: parsed.value } });
    setBusy(false);
    if (r.status === 403) return onDenied();
    if (r.data?.result) setCheck(r.data.result);
    if (!r.ok) return setMsg({ tone: "error", text: r.error || "Publishing failed" });
    onPublished(r.data?.status === "unchanged" ? "Already published, nothing changed." : `${name} is published. People can use it from the palette now.`);
  };

  const onFile = async (f: File | undefined) => {
    if (!f) return;
    load(readImport(await f.text(), f.name));
  };

  const draftFromVariables = async () => {
    setBusy(true);
    const r = await call<{ schema: unknown; notes: string[] }>(`${paths.manage}/draft`, { method: "POST", json: { name: name || "my-module", variables: tfText } });
    setBusy(false);
    if (r.status === 403) return onDenied();
    if (!r.ok || !r.data) return setMsg({ tone: "error", text: r.error });
    load({ name: name || "my-module", schema: JSON.stringify(r.data.schema, null, 2) });
    setNotes(r.data.notes);
    setTfOpen(false);
  };

  const canPublish = !!check?.report.valid && check.exists !== "different" && !checking && !busy;
  const startCls = "flex items-start gap-3 rounded-xl border border-[var(--tf-color-border)] p-3 text-left hover:border-[var(--tf-color-action)]";

  return (
    <Card className="mb-6">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="m-0 text-base font-semibold">Schema editor</h3>
        <button className="text-xs underline" onClick={onCancel}>
          Close
        </button>
      </div>
      {msg && <Notice tone={msg.tone} onClose={() => setMsg(null)}>{msg.text}</Notice>}

      <div className="mb-4 grid gap-2 sm:grid-cols-3">
        <button type="button" onClick={() => load({ name: EXAMPLE_NAME, schema: JSON.stringify(EXAMPLE_SCHEMA, null, 2) })} className={startCls}>
          <FlaskConical className="mt-0.5 size-5 shrink-0 text-[var(--tf-color-action)]" />
          <span>
            <span className="block text-sm font-semibold">Start from an example</span>
            <span className="text-xs text-[var(--tf-color-text-muted)]">A complete schema that already passes the check.</span>
          </span>
        </button>
        <label className={`${startCls} cursor-pointer`}>
          <Upload className="mt-0.5 size-5 shrink-0 text-[var(--tf-color-action)]" />
          <span>
            <span className="block text-sm font-semibold">Import a file</span>
            <span className="text-xs text-[var(--tf-color-text-muted)]">A schema .json, or one exported from another project.</span>
          </span>
          <input type="file" accept="application/json,.json" className="sr-only" onChange={(e) => onFile(e.target.files?.[0])} />
        </label>
        <button type="button" onClick={() => setTfOpen((o) => !o)} aria-expanded={tfOpen} className={startCls}>
          <FileCode2 className="mt-0.5 size-5 shrink-0 text-[var(--tf-color-action)]" />
          <span>
            <span className="block text-sm font-semibold">Draft from variables.tf</span>
            <span className="text-xs text-[var(--tf-color-text-muted)]">Paste the module's variables and get a first draft.</span>
          </span>
        </button>
      </div>

      {tfOpen && (
        <div className="mb-4 rounded-xl border border-[var(--tf-color-border)] p-3">
          <label className="block text-sm font-medium">
            variables.tf
            <textarea
              value={tfText}
              onChange={(e) => setTfText(e.target.value)}
              rows={8}
              spellCheck={false}
              placeholder={'variable "name" {\n  type        = string\n  description = "..."\n}'}
              className={`${inputCls} font-mono text-xs`}
            />
          </label>
          <p className="m-0 mt-1 text-xs text-[var(--tf-color-text-muted)]">Only this text is read. The draft replaces the schema below; you still set the module source and tidy the titles.</p>
          <button className="tf-btn mt-2" disabled={busy || !tfText.trim()} onClick={draftFromVariables}>
            {busy && <Loader2 className="size-4 animate-spin" />} Make a draft
          </button>
        </div>
      )}
      {notes.length > 0 && (
        <div className="mb-4 rounded-xl bg-[var(--tf-glass-strong)] p-3 text-xs text-[var(--tf-color-text-secondary)]">
          <p className="m-0 font-semibold">About this draft</p>
          <ul className="m-0 mt-1 list-disc pl-5">
            {notes.map((n, i) => (
              <li key={i}>{n}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="grid gap-5 lg:grid-cols-2">
        <div className="min-w-0">
          <label className="block text-sm font-medium">
            Catalog name
            <input value={name} onChange={(e) => setName(e.target.value.toLowerCase())} placeholder="e.g. storage-account" className={`${inputCls} font-mono`} />
            <span className="mt-1 block text-xs font-normal text-[var(--tf-color-text-muted)]">
              Lower case letters, digits and dashes. This is how people and the generator refer to the module
              {paths.scope === "project" ? "; it must differ from the names of the global catalog." : "."}
            </span>
          </label>
          <label className="mt-3 block text-sm font-medium">
            Schema (JSON)
            <textarea
              value={schema}
              onChange={(e) => {
                setSchema(e.target.value);
                if (!name) setName(suggestName(e.target.value));
              }}
              rows={22}
              spellCheck={false}
              placeholder="Paste a schema here, or start from one of the three ways above."
              className={`${inputCls} font-mono text-xs`}
            />
          </label>
          {parsed && !parsed.ok && <p className="m-0 mt-1 text-xs text-[var(--tf-color-error)]">Not valid JSON yet: {parsed.error}</p>}
        </div>

        <div className="min-w-0">
          <div className="mb-3 flex items-center justify-between gap-2">
            <Pills
              value={tab}
              onChange={setTab}
              items={[
                { id: "check", label: "Check", count: check ? (check.report.errors ?? []).length : undefined },
                { id: "preview", label: "Form preview" },
              ]}
            />
            {checking && (
              <span className="flex items-center gap-1 text-xs text-[var(--tf-color-text-muted)]" role="status">
                <Loader2 className="size-3.5 animate-spin" /> checking
              </span>
            )}
          </div>
          {!check && !checking && (
            <p className="m-0 text-sm text-[var(--tf-color-text-muted)]">
              {!parsed ? "The check runs while you type, once there is a schema." : !parsed.ok ? "Fix the JSON first." : !name ? "Give the module a catalog name." : ""}
            </p>
          )}
          {check && tab === "check" && <CheckReport check={check} />}
          {check && tab === "preview" &&
            (check.report.valid && parsed?.ok ? (
              <div className="rounded-xl border border-[var(--tf-color-border)] p-4">
                <p className="m-0 mb-3 text-xs text-[var(--tf-color-text-muted)]">This is the form people fill on the canvas. Nothing typed here is saved.</p>
                <ErrorBoundary key={schema}>
                  <SchemaForm schema={parsed.value as JSONSchema} values={values} onChange={setValues} />
                </ErrorBoundary>
              </div>
            ) : (
              <p className="m-0 text-sm text-[var(--tf-color-text-muted)]">The preview appears when the schema is valid.</p>
            ))}
        </div>
      </div>

      <div className="mt-5 flex flex-wrap items-center gap-2">
        <button className="tf-btn" disabled={!canPublish} onClick={publish}>
          {busy && <Loader2 className="size-4 animate-spin" />} Publish
        </button>
        <button
          className="tf-btn tf-btn--ghost"
          disabled={!parsed?.ok || !name}
          onClick={() => {
            const f = exportFile(name, schema);
            download(f.filename, f.content);
          }}
        >
          <Download className="size-4" /> Export
        </button>
        {!canPublish && <span className="text-xs text-[var(--tf-color-text-muted)]">Publishing unlocks when the check passes. A published version never changes.</span>}
      </div>
    </Card>
  );
}
