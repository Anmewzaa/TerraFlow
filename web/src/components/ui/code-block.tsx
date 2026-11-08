import React from "react";
import SyntaxHighlighter from "react-syntax-highlighter/dist/esm/prism-light";
import hcl from "react-syntax-highlighter/dist/esm/languages/prism/hcl";
import json from "react-syntax-highlighter/dist/esm/languages/prism/json";
import createElement from "react-syntax-highlighter/dist/esm/create-element";
import { Check, ChevronDown, ChevronRight, Copy, FileCode2, Folder, FolderOpen } from "lucide-react";
import { cn } from "@/lib/utils";
import type { FileStatus, LineKind } from "@/lib/diff";

SyntaxHighlighter.registerLanguage("hcl", hcl);
SyntaxHighlighter.registerLanguage("json", json);

/** VS Code Light+/Dark+ token colours, driven by --tf-code-* so the app theme decides. */
const c = (v: string, extra: React.CSSProperties = {}): React.CSSProperties => ({ color: `var(--tf-code-${v})`, ...extra });
const vscode: Record<string, React.CSSProperties> = {
  'code[class*="language-"]': c("fg", { background: "none", fontFamily: "var(--tf-font-mono)", textAlign: "left", whiteSpace: "pre", tabSize: 2 }),
  'pre[class*="language-"]': c("fg", { background: "none", fontFamily: "var(--tf-font-mono)", margin: 0, overflow: "auto" }),
  comment: c("comment"),
  prolog: c("comment"),
  heredoc: c("string"),
  string: c("string"),
  "attr-value": c("string"),
  number: c("number"),
  boolean: c("keyword"),
  keyword: c("keyword"),
  builtin: c("type"),
  "class-name": c("type"),
  function: c("function"),
  property: c("variable"),
  variable: c("variable"),
  "attr-name": c("variable"),
  operator: c("punct"),
  punctuation: c("punct"),
  interpolation: c("control"),
  "interpolation-punctuation": c("control"),
};

export interface CodeTab {
  /** Display name; a path such as "storage/main.tf" enables the folder explorer. */
  name: string;
  code: string;
  language?: string;
  highlightLines?: number[];
  /** Tint every line as an addition (diff view). */
  added?: boolean;
  /** Per-line diff kinds (added/modified/removed); overrides `added`. */
  kinds?: LineKind[];
  /** Previous text of modified lines, shown as a tooltip. */
  previous?: (string | undefined)[];
  /** File status badge in the explorer. */
  status?: FileStatus;
  /** Small notice shown above the code of this file (e.g. which components changed). */
  banner?: React.ReactNode;
  /** Called with the 0-based displayed line that was clicked. */
  onLineClick?: (line: number) => void;
  /** Lines (0-based, inclusive) to mark as the selected component. */
  activeLines?: [number, number];
  /** Scroll this line into view whenever `nonce` changes. */
  scrollTo?: { line: number; nonce: number };
}

type CodeBlockProps = {
  language: string;
  filename?: string;
  highlightLines?: number[];
  className?: string;
} & ({ code: string; tabs?: never } | { code?: never; tabs: CodeTab[] });

const STATUS_COLOR: Record<FileStatus, string> = {
  added: "var(--tf-code-add-fg)",
  modified: "var(--tf-code-mod-fg)",
  removed: "var(--tf-code-del-fg)",
  unchanged: "var(--tf-code-fg)",
};
const STATUS_LETTER: Record<FileStatus, string> = { added: "A", modified: "M", removed: "D", unchanged: "" };

interface TreeNode {
  name: string;
  path: string;
  children: Map<string, TreeNode>;
  tab?: number;
}

function buildTree(tabs: CodeTab[]): TreeNode {
  const root: TreeNode = { name: "", path: "", children: new Map() };
  tabs.forEach((t, i) => {
    const parts = t.name.split("/").filter(Boolean);
    let node = root;
    parts.forEach((part, depth) => {
      const path = parts.slice(0, depth + 1).join("/");
      if (!node.children.has(part)) node.children.set(part, { name: part, path, children: new Map() });
      node = node.children.get(part)!;
    });
    node.tab = i;
  });
  return root;
}

function Tree({
  node,
  depth,
  active,
  closed,
  toggle,
  open,
  statusOf,
}: {
  node: TreeNode;
  depth: number;
  active: number;
  closed: Set<string>;
  toggle: (p: string) => void;
  open: (i: number) => void;
  statusOf?: (tab: number) => FileStatus | undefined;
}) {
  const entries = [...node.children.values()].sort((a, b) => {
    const af = a.tab === undefined ? 0 : 1;
    const bf = b.tab === undefined ? 0 : 1;
    return af - bf || a.name.localeCompare(b.name);
  });
  return (
    <>
      {entries.map((n) => {
        const pad = { paddingLeft: 10 + depth * 14 };
        if (n.tab === undefined) {
          const isClosed = closed.has(n.path);
          return (
            <div key={n.path}>
              <button
                onClick={() => toggle(n.path)}
                style={pad}
                className="flex w-full cursor-pointer items-center gap-1.5 py-1 pr-3 text-left text-[13px] text-[var(--tf-code-fg)] hover:bg-[var(--tf-code-hover)]"
              >
                {isClosed ? <ChevronRight className="size-3.5 shrink-0" /> : <ChevronDown className="size-3.5 shrink-0" />}
                {isClosed ? (
                  <Folder className="size-4 shrink-0 text-[#dcb67a]" />
                ) : (
                  <FolderOpen className="size-4 shrink-0 text-[#dcb67a]" />
                )}
                <span className="truncate">{n.name}</span>
              </button>
              {!isClosed && (
                <Tree node={n} depth={depth + 1} active={active} closed={closed} toggle={toggle} open={open} statusOf={statusOf} />
              )}
            </div>
          );
        }
        const on = n.tab === active;
        return (
          <button
            key={n.path}
            onClick={() => open(n.tab!)}
            style={{ paddingLeft: 10 + depth * 14 + 18 }}
            className={cn(
              "flex w-full cursor-pointer items-center gap-1.5 py-1 pr-3 text-left text-[13px] text-[var(--tf-code-fg)]",
              on ? "bg-[var(--tf-code-active)]" : "hover:bg-[var(--tf-code-hover)]",
            )}
          >
            <FileCode2 className="size-4 shrink-0 text-[#a074c4]" />
            <span
              className={cn("truncate", statusOf?.(n.tab!) === "removed" && "line-through opacity-70")}
              style={{ color: STATUS_COLOR[statusOf?.(n.tab!) ?? "unchanged"] }}
            >
              {n.name}
            </span>
            {STATUS_LETTER[statusOf?.(n.tab!) ?? "unchanged"] && (
              <span
                className="ml-auto text-[11px] font-semibold"
                style={{ color: STATUS_COLOR[statusOf?.(n.tab!) ?? "unchanged"] }}
              >
                {STATUS_LETTER[statusOf?.(n.tab!) ?? "unchanged"]}
              </span>
            )}
          </button>
        );
      })}
    </>
  );
}

/** VS Code-like viewer: folder explorer + breadcrumb + highlighted code, themed. */
export function CodeBlock({ language, filename, code, highlightLines = [], tabs = [], className }: CodeBlockProps) {
  const [copied, setCopied] = React.useState(false);
  const [active, setActive] = React.useState(0);
  const [closed, setClosed] = React.useState<Set<string>>(new Set());

  const hasTabs = tabs.length > 0;
  const idx = Math.min(active, Math.max(tabs.length - 1, 0));
  const current = hasTabs ? tabs[idx] : undefined;
  const text = current ? current.code : (code ?? "");
  const lang = current?.language ?? language;
  const hl = current ? (current.highlightLines ?? []) : highlightLines;
  const added = !!current?.added;
  const kinds = current?.kinds;
  const previous = current?.previous;
  const title = current ? current.name : filename;
  const explorer = hasTabs && tabs.some((t) => t.name.includes("/"));
  const tree = React.useMemo(() => buildTree(tabs), [tabs]);
  const crumbs = (title ?? "").split("/").filter(Boolean);
  const bodyRef = React.useRef<HTMLDivElement>(null);
  const scrollNonce = current?.scrollTo?.nonce;
  const scrollLine = current?.scrollTo?.line;
  React.useEffect(() => {
    if (scrollNonce === undefined || scrollLine === undefined) return;
    bodyRef.current?.querySelector(`[data-line="${scrollLine}"]`)?.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [scrollNonce, scrollLine]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      /* clipboard unavailable (insecure context) */
    }
  };
  const toggle = (p: string) =>
    setClosed((s) => {
      const n = new Set(s);
      if (n.has(p)) n.delete(p);
      else n.add(p);
      return n;
    });

  return (
    <div
      className={cn(
        "flex min-h-[220px] w-full min-w-0 overflow-hidden rounded-xl border border-[var(--tf-code-border)] bg-[var(--tf-code-bg)] font-mono text-sm shadow-sm",
        className,
      )}
    >
      {explorer && (
        <nav
          aria-label="Files"
          className="tf-scroll-dark flex w-[168px] shrink-0 flex-col overflow-y-auto border-r border-[var(--tf-code-border)] bg-[var(--tf-code-side)] py-3 font-sans"
        >
          <div className="mb-2 px-4 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-code-muted)]">
            Project files
          </div>
          <Tree node={tree} depth={0} active={idx} closed={closed} toggle={toggle} open={setActive} statusOf={(i) => tabs[i]?.status} />
        </nav>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-2 border-b border-[var(--tf-code-border)] bg-[var(--tf-code-bar)] px-4 font-sans">
          {hasTabs && !explorer ? (
            <div className="flex min-w-0 flex-1 overflow-x-auto" role="tablist">
              {tabs.map((tab, i) => (
                <button
                  key={tab.name}
                  role="tab"
                  aria-selected={i === idx}
                  onClick={() => setActive(i)}
                  className={cn(
                    "cursor-pointer whitespace-nowrap border-0 border-b-2 border-solid px-3 py-3 text-xs transition-colors",
                    i === idx
                      ? "border-[var(--tf-color-action)] bg-transparent text-[var(--tf-code-fg)]"
                      : "border-transparent bg-transparent text-[var(--tf-code-muted)] hover:text-[var(--tf-code-fg)]",
                  )}
                >
                  {tab.name}
                </button>
              ))}
            </div>
          ) : (
            <div className="flex min-w-0 flex-1 items-center gap-1.5 py-3 text-xs text-[var(--tf-code-muted)]">
              <FileCode2 className="size-3.5 shrink-0" />
              {crumbs.map((p, i) => (
                <React.Fragment key={i}>
                  {i > 0 && <ChevronRight className="size-3 shrink-0 opacity-60" />}
                  <span className={cn("truncate", i === crumbs.length - 1 && "font-medium text-[var(--tf-code-fg)]")}>
                    {p}
                  </span>
                </React.Fragment>
              ))}
            </div>
          )}
          <button
            onClick={copy}
            aria-label="Copy code"
            className="flex cursor-pointer items-center gap-1.5 rounded-md border-0 bg-transparent px-2 py-1.5 text-xs text-[var(--tf-code-muted)] transition-colors hover:bg-[var(--tf-code-hover)] hover:text-[var(--tf-code-fg)]"
          >
            {copied ? <Check className="size-3.5 text-[var(--tf-code-comment)]" /> : <Copy className="size-3.5" />}
            {copied ? "Copied" : "Copy"}
          </button>
        </div>

        <div ref={bodyRef} className="tf-scroll-dark min-h-0 flex-1 overflow-auto py-4">
          {current?.banner}
          <SyntaxHighlighter
            language={lang}
            style={vscode}
            customStyle={{ margin: 0, padding: 0, background: "transparent", fontSize: "0.8125rem", lineHeight: 1.65, overflow: "visible" }}
            codeTagProps={{ style: { whiteSpace: "pre-wrap", overflowWrap: "anywhere" } }}
            PreTag="div"
            renderer={({ rows, stylesheet, useInlineStyles }) => {
              let n = 0;
              return rows.map((row, i) => {
                const kind: LineKind = kinds ? (kinds[i] ?? "same") : added ? "add" : "same";
                const isDel = kind === "del";
                if (!isDel) n++;
                const marker = { add: "+", mod: "~", del: "-", same: "" }[kind];
                const highlighted = kind === "same" && hl.includes(n);
                return (
                  <div
                    key={i}
                    data-line={i}
                    onClick={current?.onLineClick ? () => current.onLineClick!(i) : undefined}
                    title={kind === "mod" && previous?.[i] !== undefined ? `Was: ${previous[i]?.trim()}` : undefined}
                    className={cn("flex", current?.onLineClick && "cursor-pointer")}
                    style={{
                      boxShadow:
                        current?.activeLines && i >= current.activeLines[0] && i <= current.activeLines[1]
                          ? "inset 3px 0 0 var(--tf-color-action)"
                          : undefined,
                      background:
                        kind === "add" ? "var(--tf-code-add-bg)"
                        : kind === "mod" ? "var(--tf-code-mod-bg)"
                        : kind === "del" ? "var(--tf-code-del-bg)"
                        : highlighted ? "var(--tf-code-hl)" : "transparent",
                    }}
                  >
                    <span
                      aria-hidden
                      className="shrink-0 select-none text-right"
                      style={{ width: "3.25em", paddingRight: "0.75em", color: "var(--tf-code-line-no)", opacity: 0.75 }}
                    >
                      {isDel ? "" : n}
                    </span>
                    <span
                      aria-hidden
                      className="shrink-0 select-none text-center font-semibold"
                      style={{ width: "1.5em", color: marker ? `var(--tf-code-${{ "+": "add", "~": "mod", "-": "del" }[marker]}-fg)` : undefined }}
                    >
                      {marker}
                    </span>
                    <span className={cn("min-w-0 flex-1 pr-5", isDel && "opacity-80")} style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>
                      {createElement({ node: row, stylesheet, useInlineStyles, key: `r${i}` }) ?? " "}
                    </span>
                  </div>
                );
              });
            }}
          >
            {text}
          </SyntaxHighlighter>
        </div>
      </div>
    </div>
  );
}
