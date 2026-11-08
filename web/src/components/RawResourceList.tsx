import type { RawResourceTemplate } from "../hooks/useRawResources";

interface Props {
  templates: RawResourceTemplate[];
  disabled: boolean;
  onAdd: (t: RawResourceTemplate) => void;
}

/** Raw resources an admin prepared. Not a module: it becomes a plain `resource` block. */
export function RawResourceList({ templates, disabled, onAdd }: Props) {
  if (templates.length === 0) return null;
  return (
    <section className="border-b border-[var(--tf-color-border)] p-3 pb-5">
      <h3 className="mb-3 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">
        Raw resources <span className="font-normal">({templates.length})</span>
      </h3>
      <ul className="m-0 list-none space-y-2 p-0">
        {templates.map((t) => (
          <li key={t.name}>
            <button
              disabled={disabled}
              onClick={() => onAdd(t)}
              title={t.description || `Add ${t.resource_type}`}
              className="flex w-full flex-col items-start rounded-lg border border-dashed border-[var(--tf-color-border-strong)] bg-[var(--tf-color-surface-raised)] px-2.5 py-2 text-left hover:border-[var(--tf-color-action)] disabled:opacity-50"
            >
              <span className="text-[13px] font-medium text-[var(--tf-color-text)]">{t.title}</span>
              <span className="font-mono text-[11px] text-[var(--tf-color-text-muted)]">{t.resource_type}</span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
