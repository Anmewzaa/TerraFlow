import { useMemo, useState, type ReactNode } from "react";
import { CodeBlock } from "@/components/ui/code-block";
import type { FileDiff } from "@/lib/diff";
import { blockAt, blocksOf, condense, type HclBlock } from "@/lib/hclBlocks";

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


/** What the panel shows while the first preview is generated: the stages, and the shape of the code to come. */

/** Totals for the header of any screen that shows generated files. */
export function diffStats(shown: FileDiff[]) {
  const stats = shown.reduce(
    (acc, d) => ({ add: acc.add + d.diff.added, mod: acc.mod + d.diff.modified, del: acc.del + d.diff.removed }),
    { add: 0, mod: 0, del: 0 },
  );
  return { ...stats, existing: shown.filter((d) => d.status === "unchanged").length, folders: new Set(shown.map((d) => d.path.split("/")[0])).size };
}

/**
 * The generated Terraform as files: a "show only what changed" switch, one tab per file, the changed lines marked,
 * and a board of the components each file touches. The HCL preview page and the changeset page both use it.
 */
export function DiffFiles({
  shown,
  full = false,
  loading = false,
  before,
  emptyText,
  className = "flex-1",
}: {
  shown: FileDiff[];
  full?: boolean;
  loading?: boolean;
  /** Anything to show above the code: a loading skeleton, an error, a hint. */
  before?: ReactNode;
  /** Said when there are no files at all. */
  emptyText?: string;
  className?: string;
}) {
  const [changesOnly, setChangesOnly] = useState(true);
  // The component being looked at, per file: its index among the file's blocks.
  const [activeBy, setActiveBy] = useState<Record<string, number>>({});
  const [scroll, setScroll] = useState<Record<string, { line: number; nonce: number }>>({});

  const pick = (file: string, blocks: HclBlock[], i: number, scrollIt: boolean) => {
    setActiveBy((m) => ({ ...m, [file]: i }));
    if (scrollIt && blocks[i]) setScroll((m) => ({ ...m, [file]: { line: blocks[i]!.start, nonce: (m[file]?.nonce ?? 0) + 1 } }));
  };

  // With "changes only" on, files without changes drop out and the rest keep just the touched blocks.
  const tabs = useMemo(() => {
    const prepared = shown.map((d) => {
      const blocks = blocksOf(d.diff.text, d.diff.kinds);
      return { d, blocks, touched: blocks.some((b) => b.change !== "unchanged") };
    });
    const list = changesOnly ? prepared.filter((x) => x.touched || (x.blocks.length === 0 && x.d.status !== "unchanged")) : prepared;
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
        activeLines: blocks[active] ? ([blocks[active]!.start, blocks[active]!.end] as [number, number]) : undefined,
        scrollTo: scroll[d.path],
        onLineClick: (line: number) => {
          const i = blockAt(blocks, line);
          if (i >= 0) pick(d.path, blocks, i, false);
        },
        banner: <ComponentBoard file={d.path} blocks={blocks} active={active} onPick={(i) => pick(d.path, blocks, i, true)} />,
      };
    });
  }, [shown, changesOnly, activeBy, scroll]);
  const hidden = shown.length - tabs.length;

  return (
    <>
      {shown.length > 0 && (
        <label className={`flex cursor-pointer items-center gap-2 border-b border-[var(--tf-color-border)] px-4 py-2 text-xs text-[var(--tf-color-text-secondary)] ${full ? "bg-[color-mix(in_srgb,var(--tf-color-surface)_90%,transparent)] backdrop-blur-sm" : ""}`}>
          <input type="checkbox" checked={changesOnly} onChange={(e) => setChangesOnly(e.target.checked)} />
          Show only what changed
          {changesOnly && hidden > 0 && (
            <span className="text-[var(--tf-color-text-muted)]">
              ({hidden} unchanged {hidden === 1 ? "file" : "files"} hidden)
            </span>
          )}
        </label>
      )}
      <div
        className={`flex min-h-0 flex-1 flex-col gap-4 transition-opacity duration-200 ${full ? "p-6" : "p-4"}`}
        aria-busy={loading || undefined}
        style={loading && shown.length > 0 ? { opacity: 0.55, pointerEvents: "none" } : undefined}
      >
        {before}
        {emptyText && shown.length === 0 && <p className="text-sm text-[var(--tf-color-text-muted)]">{emptyText}</p>}
        {shown.length > 0 && tabs.length === 0 && (
          <p className="text-sm text-[var(--tf-color-text-muted)]">
            Nothing changed yet. Turn off "Show only what changed" to read the files already in the repository.
          </p>
        )}
        {tabs.length > 0 && <CodeBlock language="hcl" className={className} tabs={tabs} />}
      </div>
    </>
  );
}
