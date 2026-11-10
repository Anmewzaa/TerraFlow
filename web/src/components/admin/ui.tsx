import type { ReactNode } from "react";
import { AlertCircle, CheckCircle2, CircleHelp } from "lucide-react";

/** Shared look of the admin screens, so each panel reads the same way. */

export const inputCls =
  "mt-1 block w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none transition focus:border-[var(--tf-color-action)] focus:ring-2 focus:ring-[var(--tf-color-action)]/25";

export function Notice({ tone, children, onClose }: { tone: "error" | "ok"; children: ReactNode; onClose?: () => void }) {
  const Icon = tone === "error" ? AlertCircle : CheckCircle2;
  const color = tone === "error" ? "var(--tf-color-error)" : "var(--tf-color-success)";
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      className="mb-4 flex items-start gap-2 rounded-lg border px-3 py-2 text-sm"
      style={{ borderColor: color, color }}
    >
      <Icon className="mt-0.5 size-4 shrink-0" />
      <span className="flex-1">{children}</span>
      {onClose && (
        <button onClick={onClose} aria-label="Dismiss" className="opacity-70 hover:opacity-100">
          ×
        </button>
      )}
    </div>
  );
}

/** A panel's title, what it is for in one sentence, and its actions (help, add...). */
export function PanelHeader({ title, intro, actions }: { title: string; intro: string; actions?: ReactNode }) {
  return (
    <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0 max-w-2xl">
        <h2 className="m-0 text-xl font-semibold tracking-tight">{title}</h2>
        <p className="m-0 mt-1 text-sm text-[var(--tf-color-text-secondary)]">{intro}</p>
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function HelpButton({ onClick, label = "Help" }: { onClick: () => void; label?: string }) {
  return (
    <button type="button" className="tf-btn tf-btn--ghost" onClick={onClick}>
      <CircleHelp className="size-4" /> {label}
    </button>
  );
}

/** What to do first, when there is nothing here yet. */
export function EmptyState({ icon, title, children, action }: { icon: ReactNode; title: string; children: ReactNode; action?: ReactNode }) {
  return (
    <div className="rounded-2xl border border-dashed border-[var(--tf-color-border-strong)] px-6 py-10 text-center">
      <div className="mx-auto mb-3 flex size-11 items-center justify-center rounded-xl bg-[var(--tf-glass-strong)] text-[var(--tf-color-action)]">{icon}</div>
      <h3 className="m-0 text-base font-semibold">{title}</h3>
      <p className="mx-auto mb-4 mt-1 max-w-md text-sm text-[var(--tf-color-text-secondary)]">{children}</p>
      {action}
    </div>
  );
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-4 ${className}`}>{children}</div>;
}

export function Chip({ children, tone = "neutral" }: { children: ReactNode; tone?: "good" | "warn" | "bad" | "neutral" | "info" }) {
  return (
    <span className="tf-badge" data-tone={tone === "warn" ? "warn" : tone === "bad" ? "bad" : tone}>
      {children}
    </span>
  );
}

/** A row of filter pills with counts. */
export function Pills<T extends string>({ value, onChange, items }: { value: T; onChange: (v: T) => void; items: { id: T; label: string; count?: number }[] }) {
  return (
    <div role="tablist" className="flex flex-wrap gap-1.5">
      {items.map((i) => (
        <button
          key={i.id}
          role="tab"
          aria-selected={value === i.id}
          onClick={() => onChange(i.id)}
          className={`rounded-full border px-3 py-1 text-xs font-medium transition ${
            value === i.id
              ? "border-[var(--tf-color-action)] bg-[var(--tf-color-action)] text-[var(--tf-color-on-action)]"
              : "border-[var(--tf-color-border)] text-[var(--tf-color-text-secondary)] hover:border-[var(--tf-color-action)]"
          }`}
        >
          {i.label}
          {i.count !== undefined && <span className="ml-1.5">{i.count}</span>}
        </button>
      ))}
    </div>
  );
}
