import { useEffect, useState } from "react";
import { FileCode2, Loader2, RefreshCw, X } from "lucide-react";
import { diffWithRepo } from "@/lib/diff";
import { DiffFiles, diffStats } from "./DiffFiles";
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

const STEPS = ["Reading the repository", "Generating the module calls", "Comparing with what is there"];

function PreviewSkeleton() {
  const [step, setStep] = useState(0);
  useEffect(() => {
    const t = setInterval(() => setStep((s) => Math.min(s + 1, STEPS.length - 1)), 900);
    return () => clearInterval(t);
  }, []);
  const widths = [38, 72, 55, 64, 30, 0, 46, 80, 58, 70, 34, 0, 52, 66, 40];
  return (
    <div role="status" aria-live="polite" aria-label="Generating the Terraform preview" className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-[var(--tf-code-border)] bg-[var(--tf-code-bg)]">
      <div className="flex items-center gap-2 border-b border-[var(--tf-code-border)] bg-[var(--tf-code-bar)] px-3 py-2">
        <span className="tf-skel" style={{ width: 90 }} />
        <span className="tf-skel" style={{ width: 70, opacity: 0.6 }} />
      </div>
      <div className="flex-1 overflow-hidden p-4">
        <ol className="mb-4 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--tf-code-muted)]">
          {STEPS.map((t, i) => (
            <li key={t} className="flex items-center gap-1.5" style={{ color: i <= step ? "var(--tf-code-fg)" : undefined, opacity: i <= step ? 1 : 0.55 }}>
              <span className={`size-1.5 rounded-full ${i === step ? "tf-step-dot" : ""}`} style={{ background: i < step ? "var(--tf-code-add-fg)" : "var(--tf-color-action)" }} />
              {t}
            </li>
          ))}
        </ol>
        <div className="grid gap-2.5" aria-hidden>
          {widths.map((w, i) => (w === 0 ? <span key={i} className="h-2" /> : <span key={i} className="tf-skel" style={{ width: `${w}%`, marginLeft: i % 5 === 1 || i % 5 === 2 ? 20 : 0, animationDelay: `${i * 60}ms` }} />))}
        </div>
      </div>
    </div>
  );
}

export function HclPreviewPanel({ preview, loading, error, onLoad, onClose, full = false }: Props) {
  const files = preview?.files ?? [];
  // Generated files are small, so diffing on every render is cheap.
  const shown = diffWithRepo(preview?.repo_files ?? [], preview?.base_files ?? [], files);
  const { add, mod, del, existing, folders } = diffStats(shown);

  return (
    <aside
      className={`flex min-h-0 flex-col text-[var(--tf-color-text)] ${full ? "bg-[color-mix(in_srgb,var(--tf-color-surface)_94%,transparent)] backdrop-blur-sm" : "bg-[var(--tf-color-surface)]"} ${full ? "m-4 min-w-0 flex-1 overflow-hidden rounded-2xl border border-[var(--tf-color-border)] shadow-sm" : "w-[520px] max-w-[48vw] shrink-0 border-l border-[var(--tf-color-border)]"}`}
      data-full={full || undefined}
    >
      <div className={`flex items-center justify-between gap-3 border-b border-[var(--tf-color-border)] px-4 py-3 ${full ? "bg-[color-mix(in_srgb,var(--tf-color-surface)_90%,transparent)] backdrop-blur-sm" : ""}`}>
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
                <span style={{ color: "var(--tf-code-add-fg)" }}>+{add}</span>
                <span style={{ color: "var(--tf-code-mod-fg)" }}>~{mod}</span>
                <span style={{ color: "var(--tf-code-del-fg)" }}>-{del}</span>
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

      <div className={loading ? "tf-progress" : "h-[2px]"} role={loading ? "progressbar" : undefined} aria-label={loading ? "Generating" : undefined} />

      <DiffFiles
        shown={shown}
        full={full}
        loading={loading}
        emptyText={preview ? "No modules on canvas. Drag modules from the catalog to get started." : undefined}
        before={
          <>
            {loading && !preview && <PreviewSkeleton />}
            {error && <div className="rounded-lg bg-[var(--tf-color-error-surface)] px-4 py-3 text-sm text-[var(--tf-color-error)]">{error}</div>}
            {!preview && !loading && !error && <p className="text-sm text-[var(--tf-color-text-muted)]">Click Refresh to generate the Terraform preview.</p>}
          </>
        }
      />
    </aside>
  );
}
