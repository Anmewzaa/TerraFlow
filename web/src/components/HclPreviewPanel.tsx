import { useMemo, useState } from "react";
import { FileCode2, Loader2, RefreshCw, X } from "lucide-react";
import { CodeBlock } from "@/components/ui/code-block";
import { diffWithRepo } from "@/lib/diff";
import { blockAt, blocksOf, condense, type HclBlock } from "@/lib/hclBlocks";
import type { GenerateOutput } from "../types/changeset";

interface Props {
  preview: GenerateOutput | null;
  loading: boolean;
  error: string | null;
  onLoad: () => void;
  onClose: () => void;
  /** A page of its own (fills the area) instead of a side panel. */
  full?: boolean;
}

const CHANGE_COLOR: Record<HclBlock["change"], string> = {
  added: "var(--tf-code-add-fg)",
  modified: "var(--tf-code-mod-fg)",
  removed: "var(--tf-code-del-fg)",
  unchanged: "var(--tf-code-muted)",
};
const CHANGE_WORD: Record<HclBlock["change"], string> = {
  added: "new",
  modified: "changed",
  removed: "removed",
  unchanged: "unchanged",
};

interface BoardProps {
  file: string;
  blocks: HclBlock[];
  active: number;
  onPick: (i: number) => void;
}

/**
 * The small board over the code: which components this file touches, and
 * details of the one you are looking at. Click a chip or any line to switch.
 */
function ComponentBoard({ file, blocks, active, onPick }: BoardProps) {
  if (blocks.length === 0) return null;
  const cur = blocks[active] ?? blocks[0];
  const changed = blocks.filter((b) => b.change !== "unchanged").length;
  const parts = [
    cur.stats.add ? `${cur.stats.add} added` : "",
    cur.stats.mod ? `${cur.stats.mod} changed` : "",
    cur.stats.del ? `${cur.stats.del} removed` : "",
  ].filter(Boolean);
  return (
    <div
      className="sticky top-0 z-10 float-right mb-3 mr-4 flex w-[250px] max-w-[45%] flex-col gap-2 rounded-lg border border-[var(--tf-code-border)] bg-[var(--tf-code-bar)] p-2.5 font-sans text-[11px] shadow-lg"
      aria-label="Components in this file"
    >
      <div className="rounded-md bg-[var(--tf-code-hover)] px-2 py-1.5" aria-live="polite">
        <div className="text-[var(--tf-code-muted)]">{cur.kind}</div>
        <div className="break-all font-mono text-[13px] font-semibold text-[var(--tf-code-fg)]">{cur.name}</div>
        <div className="mt-0.5 flex flex-wrap items-center gap-x-2" style={{ color: CHANGE_COLOR[cur.change] }}>
          <span className="font-semibold">{CHANGE_WORD[cur.change]}</span>
          {parts.length > 0 && <span className="text-[var(--tf-code-muted)]">{parts.join(" · ")}</span>}
        </div>
        <div className="mt-0.5 truncate text-[var(--tf-code-muted)]" title={file}>in {file}</div>
      </div>
      <div className="text-[var(--tf-code-muted)]">
        {changed} of {blocks.length} components touched
      </div>
      {/* A map of the file: one segment per component, as wide as its code. */}
      <div aria-label="File map" className="flex h-2 gap-px overflow-hidden rounded-sm">
        {blocks.map((b, i) => (
          <button
            key={`${b.kind}.${b.name}`}
            onClick={() => onPick(i)}
            aria-label={`${b.name}: ${CHANGE_WORD[b.change]}`}
            title={`${b.kind} ${b.name}: ${CHANGE_WORD[b.change]}`}
            className="cursor-pointer border-0 p-0"
            style={{
              flex: `${Math.max(b.end - b.start + 1, 2)} 1 0`,
              minWidth: 4,
              background: CHANGE_COLOR[b.change],
              opacity: b.change === "unchanged" ? 0.35 : i === active ? 1 : 0.7,
              outline: i === active ? "1px solid var(--tf-code-fg)" : "none",
            }}
          />
        ))}
      </div>
      <ul className="m-0 flex list-none flex-col gap-1 p-0">
        {blocks.map((b, i) => (
          <li key={`${b.kind}.${b.name}`}>
            <button
              onClick={() => onPick(i)}
              aria-pressed={i === active}
              title={`${b.kind} ${b.name}: ${CHANGE_WORD[b.change]}`}
              className="flex w-full cursor-pointer items-center gap-1.5 rounded-md border-0 px-1.5 py-1 text-left"
              style={{
                background: i === active ? "var(--tf-code-hover)" : "transparent",
                outline: i === active ? "1px solid var(--tf-color-action)" : "none",
                color: "var(--tf-code-fg)",
                opacity: b.change === "unchanged" ? 0.6 : 1,
              }}
            >
              <span className="size-2 shrink-0 rounded-full" style={{ background: CHANGE_COLOR[b.change] }} />
              <span className="min-w-0 flex-1 truncate font-mono">{b.name}</span>
              <span className="shrink-0" style={{ color: CHANGE_COLOR[b.change] }}>{CHANGE_WORD[b.change]}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function HclPreviewPanel({ preview, loading, error, onLoad, onClose, full = false }: Props) {
  const [changesOnly, setChangesOnly] = useState(true);
  // The component being looked at, per file: its index among the file's blocks.
  const [activeBy, setActiveBy] = useState<Record<string, number>>({});
  const [scroll, setScroll] = useState<Record<string, { line: number; nonce: number }>>({});
  const files = preview?.files ?? [];
  // Generated files are small, so diffing on every render is cheap.
  const shown = diffWithRepo(preview?.repo_files ?? [], preview?.base_files ?? [], files);
  const existing = shown.filter((d) => d.status === "unchanged").length;
  const stats = shown.reduce(
    (a, d) => ({ add: a.add + d.diff.added, mod: a.mod + d.diff.modified, del: a.del + d.diff.removed }),
    { add: 0, mod: 0, del: 0 },
  );
  const folders = new Set(shown.map((d) => d.path.split("/")[0])).size;

  const pick = (file: string, blocks: HclBlock[], i: number, scrollIt: boolean) => {
    setActiveBy((m) => ({ ...m, [file]: i }));
    if (scrollIt && blocks[i]) setScroll((m) => ({ ...m, [file]: { line: blocks[i].start, nonce: (m[file]?.nonce ?? 0) + 1 } }));
  };

  // With "changes only" on, files without changes drop out and the rest keep just the touched blocks.
  const tabs = useMemo(() => {
    const prepared = shown.map((d) => {
      const blocks = blocksOf(d.diff.text, d.diff.kinds);
      return { d, blocks, touched: blocks.some((b) => b.change !== "unchanged") };
    });
    const list = changesOnly ? prepared.filter((p) => p.touched || (p.blocks.length === 0 && p.d.status !== "unchanged")) : prepared;
    return list.map(({ d, blocks: fullBlocks }) => {
      const c = changesOnly
        ? condense(d.diff.text, d.diff.kinds, d.diff.previous, fullBlocks)
        : { text: d.diff.text, kinds: d.diff.kinds, previous: d.diff.previous };
      // Blocks of what is on screen, so a clicked line maps to its component.
      const blocks = changesOnly ? blocksOf(c.text, c.kinds) : fullBlocks;
      const firstChanged = blocks.findIndex((b) => b.change !== "unchanged");
      const active = Math.min(activeBy[d.path] ?? (firstChanged >= 0 ? firstChanged : 0), Math.max(blocks.length - 1, 0));
      return {
        name: d.path,
        code: c.text,
        language: "hcl",
        kinds: c.kinds,
        previous: c.previous,
        status: d.status,
        activeLines: blocks[active] ? ([blocks[active].start, blocks[active].end] as [number, number]) : undefined,
        scrollTo: scroll[d.path],
        onLineClick: (line: number) => {
          const i = blockAt(blocks, line);
          if (i >= 0) pick(d.path, blocks, i, false);
        },
        banner: (
          <>
            <ComponentBoard file={d.path} blocks={blocks} active={active} onPick={(i) => pick(d.path, blocks, i, true)} />
          </>
        ),
      };
    });
  }, [shown, changesOnly, activeBy, scroll]);
  const hidden = shown.length - tabs.length;

  return (
    <aside
      className={`flex min-h-0 flex-col bg-[var(--tf-color-surface)] text-[var(--tf-color-text)] ${full ? "min-w-0 flex-1" : "w-[520px] max-w-[48vw] shrink-0 border-l border-[var(--tf-color-border)]"}`}
      data-full={full || undefined}
    >
      <div className="flex items-center justify-between gap-3 border-b border-[var(--tf-color-border)] px-4 py-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <FileCode2 className="size-5 shrink-0 text-[var(--tf-color-action)]" />
          <div className="min-w-0">
            <h2 className="text-sm font-semibold leading-tight">Terraform preview</h2>
            {shown.length > 0 && (
              <p className="flex flex-wrap items-center gap-x-3 text-xs text-[var(--tf-color-text-muted)]">
                <span>
                  {shown.length} {shown.length === 1 ? "file" : "files"} in {folders}{" "}
                  {folders === 1 ? "folder" : "folders"}
                  {existing > 0 ? ` · ${existing} unchanged` : ""}
                </span>
                <span style={{ color: "var(--tf-code-add-fg)" }}>+{stats.add}</span>
                <span style={{ color: "var(--tf-code-mod-fg)" }}>~{stats.mod}</span>
                <span style={{ color: "var(--tf-code-del-fg)" }}>-{stats.del}</span>
              </p>
            )}
          </div>
        </div>
        <div className="flex shrink-0 gap-2">
          <button
            onClick={onLoad}
            disabled={loading}
            className="tf-btn tf-btn--ghost"
            style={{ minHeight: 32, opacity: loading ? 0.7 : 1 }}
          >
            {loading ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
            {loading ? "Generating…" : "Refresh"}
          </button>
          <button
            onClick={onClose}
            aria-label="Close preview"
            className="tf-btn tf-btn--ghost"
            style={{ minHeight: 32, padding: "0 var(--tf-space-2)" }}
          >
            <X className="size-4" />
          </button>
        </div>
      </div>

      {shown.length > 0 && (
        <label className="flex cursor-pointer items-center gap-2 border-b border-[var(--tf-color-border)] px-4 py-2 text-xs text-[var(--tf-color-text-secondary)]">
          <input type="checkbox" checked={changesOnly} onChange={(e) => setChangesOnly(e.target.checked)} />
          Show only what changed
          {changesOnly && hidden > 0 && (
            <span className="text-[var(--tf-color-text-muted)]">
              ({hidden} unchanged {hidden === 1 ? "file" : "files"} hidden)
            </span>
          )}
        </label>
      )}

      <div className={`flex min-h-0 flex-1 flex-col gap-4 ${full ? "p-6" : "p-4"}`}>
        {error && (
          <div className="rounded-lg bg-[var(--tf-color-error-surface)] px-4 py-3 text-sm text-[var(--tf-color-error)]">
            {error}
          </div>
        )}
        {!preview && !loading && !error && (
          <p className="text-sm text-[var(--tf-color-text-muted)]">Click Refresh to generate the Terraform preview.</p>
        )}
        {preview && shown.length === 0 && (
          <p className="text-sm text-[var(--tf-color-text-muted)]">
            No modules on canvas. Drag modules from the catalog to get started.
          </p>
        )}
        {shown.length > 0 && tabs.length === 0 && (
          <p className="text-sm text-[var(--tf-color-text-muted)]">
            Nothing changed yet. Turn off "Show only what changed" to read the files already in the repository.
          </p>
        )}
        {tabs.length > 0 && <CodeBlock language="hcl" className="flex-1" tabs={tabs} />}
      </div>
    </aside>
  );
}
