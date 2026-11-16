import { useCallback, useEffect, useState } from "react";
import { Boxes, FlaskConical, Plus, Search } from "lucide-react";
import { call } from "./api";
import { Card, Chip, EmptyState, HelpButton, inputCls, Notice, PanelHeader, Pills } from "./ui";
import { catalogPaths, type CatalogPaths } from "../../lib/catalogApi";
import { download, exportFile } from "../../lib/schemaEditor";
import { SchemaEditor } from "../catalog/SchemaEditor";
import { PanelLoading } from "@/components/ui/skeleton";

export { suggestName } from "../../lib/schemaEditor";

interface ModuleRow {
  name: string;
  version: string;
  title: string;
  status: "published" | "deprecated";
  target_folder: string;
  imported_by: string;
  updated_at: string;
}

type Filter = "all" | "published" | "deprecated";

const GLOBAL = catalogPaths(null);

interface Props {
  onDenied: () => void;
  /** Opens the documentation (the admin area has it; a project page does not). */
  onHelp?: () => void;
  exampleTick?: number;
  /** Which catalog: the global one (default) or a project's own (D-060). */
  paths?: CatalogPaths;
  /** Called after a module is published or retired, so a palette can reload. */
  onChanged?: () => void;
}

/** Module management with the schema editor, for the global catalog or one project's own. */
export function ModulesPanel({ onDenied, onHelp, exampleTick = 0, paths = GLOBAL, onChanged }: Props) {
  const [rows, setRows] = useState<ModuleRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState<{ tone: "error" | "ok"; text: string } | null>(null);
  // The editor is closed (null), open and empty, or open on a published version.
  const [editor, setEditor] = useState<{ key: number; initial?: { name: string; schema: string }; example?: number } | null>(null);
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const project = paths.scope === "project";

  const load = useCallback(async () => {
    const r = await call<ModuleRow[]>(`${paths.manage}/modules`);
    if (r.status === 403) return onDenied();
    if (!r.ok) setMsg({ tone: "error", text: r.error });
    setRows(r.data ?? []);
    setLoading(false);
  }, [onDenied, paths.manage]);

  useEffect(() => {
    load();
  }, [load]);

  const openEditor = useCallback((initial?: { name: string; schema: string }, example = 0) => {
    setMsg(null);
    setEditor((e) => ({ key: (e?.key ?? 0) + 1, initial, example }));
  }, []);

  // The Docs page's "Use the example" button asks for the example through this counter.
  useEffect(() => {
    if (exampleTick > 0) openEditor(undefined, exampleTick);
  }, [exampleTick, openEditor]);

  /** The schema text of a published version, as it was imported. */
  const schemaOf = async (m: ModuleRow): Promise<string | null> => {
    const r = await call<{ schema: unknown }>(`${paths.read}/${encodeURIComponent(m.name)}/${encodeURIComponent(m.version)}`);
    if (!r.ok || !r.data) {
      setMsg({ tone: "error", text: r.error || "Could not read the schema" });
      return null;
    }
    return JSON.stringify(r.data.schema, null, 2);
  };

  const setStatus = async (m: ModuleRow, status: ModuleRow["status"]) => {
    const r = await call(`${paths.manage}/modules/${encodeURIComponent(m.name)}/${encodeURIComponent(m.version)}`, { method: "PATCH", json: { status } });
    if (r.status === 403) return onDenied();
    if (!r.ok) return setMsg({ tone: "error", text: r.error });
    load();
    onChanged?.();
  };

  const shown = rows.filter(
    (m) => (filter === "all" || m.status === filter) && (!q.trim() || `${m.name} ${m.title}`.toLowerCase().includes(q.trim().toLowerCase())),
  );

  return (
    <div>
      <PanelHeader
        title={project ? "Project modules" : "Modules"}
        intro={
          project
            ? "Modules only this project uses, next to the ones of the global catalog. The owner and co-owners publish them; everyone in the project builds with them."
            : "Terraform modules your team has agreed to use. People drag them onto the canvas and fill the form that TerraFlow draws from each module's schema."
        }
        actions={
          <>
            {onHelp && <HelpButton onClick={onHelp} label="Docs" />}
            {!editor && (
              <button className="tf-btn" onClick={() => openEditor()}>
                <Plus className="size-4" /> Add a module
              </button>
            )}
          </>
        }
      />
      {msg && <Notice tone={msg.tone} onClose={() => setMsg(null)}>{msg.text}</Notice>}

      {editor && (
        <SchemaEditor
          key={editor.key}
          paths={paths}
          initial={editor.initial}
          exampleTick={editor.example}
          onDenied={onDenied}
          onCancel={() => setEditor(null)}
          onPublished={(text) => {
            setEditor(null);
            setMsg({ tone: "ok", text });
            load();
            onChanged?.();
          }}
        />
      )}

      {loading ? (
        <PanelLoading rows={4} />
      ) : rows.length === 0 ? (
        !editor && (
          <EmptyState
            icon={<Boxes className="size-5" />}
            title={project ? "No modules of this project yet" : "No modules yet"}
            action={
              <button className="tf-btn" onClick={() => openEditor(undefined, 1)}>
                <FlaskConical className="size-4" /> Start from an example
              </button>
            }
          >
            A module gives people a ready-made form for something your team builds again and again. Start from the example to see how a schema
            looks{onHelp ? ", or open Docs for every key." : "."}
          </EmptyState>
        )
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
                  <div className="mt-auto flex flex-wrap gap-2 pt-3">
                    <button
                      className="tf-btn tf-btn--ghost"
                      onClick={async () => {
                        const schema = await schemaOf(m);
                        if (schema) openEditor({ name: m.name, schema });
                      }}
                    >
                      New version
                    </button>
                    <button
                      className="tf-btn tf-btn--ghost"
                      onClick={async () => {
                        const schema = await schemaOf(m);
                        if (!schema) return;
                        const f = exportFile(m.name, schema);
                        download(f.filename, f.content);
                      }}
                    >
                      Export
                    </button>
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
