import { FileCode2, Loader2, RefreshCw, X } from "lucide-react";
import { CodeBlock } from "@/components/ui/code-block";
import { diffWithRepo } from "@/lib/diff";
import type { GenerateOutput } from "../types/changeset";

interface Props {
  preview: GenerateOutput | null;
  loading: boolean;
  error: string | null;
  onLoad: () => void;
  onClose: () => void;
}

export function HclPreviewPanel({ preview, loading, error, onLoad, onClose }: Props) {
  const files = preview?.files ?? [];
  // Generated files are small, so diffing on every render is cheap.
  const shown = diffWithRepo(preview?.repo_files ?? [], preview?.base_files ?? [], files);
  const existing = shown.filter((d) => d.status === "unchanged").length;
  const stats = shown.reduce(
    (a, d) => ({ add: a.add + d.diff.added, mod: a.mod + d.diff.modified, del: a.del + d.diff.removed }),
    { add: 0, mod: 0, del: 0 },
  );
  const folders = new Set(shown.map((d) => d.path.split("/")[0])).size;

  return (
    <aside className="flex w-[680px] max-w-[65vw] shrink-0 flex-col border-l border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] text-[var(--tf-color-text)] shadow-[-8px_0_24px_rgb(10_31_77/0.06)]">
      <div className="flex items-center justify-between gap-3 border-b border-[var(--tf-color-border)] px-5 py-4">
        <div className="flex min-w-0 items-center gap-2.5">
          <FileCode2 className="size-5 shrink-0 text-[var(--tf-color-action)]" />
          <div className="min-w-0">
            <h2 className="text-sm font-semibold leading-tight">Terraform preview</h2>
            {shown.length > 0 && (
              <p className="flex flex-wrap items-center gap-x-3 text-xs text-[var(--tf-color-text-muted)]">
                <span>
                  {shown.length} {shown.length === 1 ? "file" : "files"} in {folders}{" "}
                  {folders === 1 ? "folder" : "folders"}
                  {existing > 0 ? ` · ${existing} already in repo` : ""}
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

      <div className="flex min-h-0 flex-1 flex-col gap-4 p-5">
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
        {shown.length > 0 && (
          <CodeBlock
            language="hcl"
            className="flex-1"
            tabs={shown.map((d) => ({
              name: d.path,
              code: d.diff.text,
              language: "hcl",
              kinds: d.diff.kinds,
              previous: d.diff.previous,
              status: d.status,
            }))}
          />
        )}
      </div>
    </aside>
  );
}
