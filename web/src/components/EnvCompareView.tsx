import { useMemo, useState } from "react";
import { ArrowRight, ArrowLeftRight, ArrowRightLeft, CheckCircle2, CircleHelp, Copy, Layers, Loader2, Minus, Plus, Search, TriangleAlert } from "lucide-react";
import { showValue, useDraftDiff, useEnvDiff, type DiffStatus, type ResourceDiff } from "../hooks/useEnvDiff";

type Filter = "all" | "differs" | "only_from" | "only_to" | "unknown" | "same";

const STATUS: Record<DiffStatus, { icon: typeof Plus; tone: string; surface: string }> = {
  same: { icon: CheckCircle2, tone: "var(--tf-color-success)", surface: "var(--tf-color-success-surface)" },
  differs: { icon: ArrowLeftRight, tone: "var(--tf-color-warning)", surface: "var(--tf-color-warning-surface)" },
  unknown: { icon: CircleHelp, tone: "var(--tf-color-text-muted)", surface: "var(--tf-color-neutral-surface)" },
  only_from: { icon: Minus, tone: "var(--tf-color-info)", surface: "var(--tf-color-info-surface)" },
  only_to: { icon: Plus, tone: "var(--tf-color-info)", surface: "var(--tf-color-info-surface)" },
};

const field =
  "h-9 rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-surface)] px-3 text-sm outline-none transition focus:border-[var(--tf-color-action)] focus:ring-2 focus:ring-[color-mix(in_srgb,var(--tf-color-action)_25%,transparent)]";

/** Two environments side by side (8.10). Read-only: it reads the repository and changes nothing. */
export function EnvCompareView({ projectId, envs, current, onCopy }: { projectId: string; envs: string[]; current: string; onCopy?: (node: unknown, env: string) => void }) {
  const [copyMsg, setCopyMsg] = useState<string | null>(null);
  const [from, setFrom] = useState(current);
  const [to, setTo] = useState(envs.find((e) => e !== current) ?? "");
  const [filter, setFilter] = useState<Filter>("differs");
  const [stack, setStack] = useState("");
  const [query, setQuery] = useState("");
  // Where the two sides come from: the repository, or the canvases that are not published yet.
  const [source, setSource] = useState<"repo" | "drafts">("repo");
  const repo = useEnvDiff(projectId, from, to, source === "repo");
  const drafts = useDraftDiff(projectId, from, to, source === "drafts");
  const { diff, error, loading } = source === "repo" ? repo : drafts;

  const copy = async (id: string) => {
    setCopyMsg(null);
    const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/env-copy?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&id=${encodeURIComponent(id)}`);
    const body = await res.json().catch(() => ({}));
    if (!res.ok) return setCopyMsg(body?.error ?? `Could not copy (${res.status})`);
    onCopy?.(body.node, body.env);
  };

  const stacks = useMemo(() => [...new Set((diff?.resources ?? []).map((r) => r.stack))].sort(), [diff]);
  const q = query.trim().toLowerCase();
  const rows = (diff?.resources ?? []).filter((r) => {
    if (filter === "differs" ? r.status === "same" : filter !== "all" && r.status !== filter) return false;
    if (stack && r.stack !== stack) return false;
    return !q || `${r.stack} ${r.label ?? ""} ${r.type}`.toLowerCase().includes(q);
  });
  const grouped = useMemo(() => {
    const m = new Map<string, ResourceDiff[]>();
    for (const r of rows) m.set(r.stack, [...(m.get(r.stack) ?? []), r]);
    return [...m.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [rows]);

  const sameEnv = from === to;
  const tiles: { key: Filter; label: string; n: number; status: DiffStatus | "all" }[] = diff
    ? [
        { key: "differs", label: "Differ", n: diff.summary.differs, status: "differs" },
        { key: "only_from", label: `Only in ${diff.from}`, n: diff.summary.only_from, status: "only_from" },
        { key: "only_to", label: `Only in ${diff.to}`, n: diff.summary.only_to, status: "only_to" },
        { key: "unknown", label: "Cannot tell", n: diff.summary.unknown, status: "unknown" },
        { key: "same", label: "The same", n: diff.summary.same, status: "same" },
      ]
    : [];

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="mx-4 my-4 max-w-5xl space-y-5 rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-6 shadow-sm md:mx-auto md:my-6 md:p-8">
        <header className="flex items-start gap-3">
          <span
            className="mt-0.5 grid size-10 shrink-0 place-items-center rounded-xl text-[var(--tf-color-on-action)] shadow-[0_6px_16px_-6px_var(--tf-color-action),inset_0_1px_0_rgb(255_255_255/0.3)]"
            style={{ background: "linear-gradient(145deg, color-mix(in srgb, var(--tf-color-action) 82%, white), var(--tf-color-action))" }}
            aria-hidden
          >
            <ArrowRightLeft className="size-5" strokeWidth={2.2} />
          </span>
          <div>
            <h2 className="m-0 text-xl font-semibold leading-tight">Compare environments</h2>
            <p className="m-0 mt-1 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">
              {source === "repo" ? "Reads both environments from the repository" : "Reads the canvas of both environments (the open draft, or the newest saved version)"} and pairs resources by stack and name. A value that cannot be worked out is shown as unknown, never as equal. Nothing here changes the repository.
            </p>
          </div>
        </header>

        <section aria-label="What to compare" className="rounded-xl border border-[var(--tf-color-border)] bg-[color-mix(in_srgb,var(--tf-color-surface)_90%,transparent)] backdrop-blur-sm p-4 shadow-[var(--tf-shadow-rest)]">
          <div className="flex flex-wrap items-center gap-2">
            <select aria-label="From environment" className={`${field} min-w-28 font-medium`} value={from} onChange={(e) => setFrom(e.target.value)}>
              {envs.map((e) => (
                <option key={e}>{e}</option>
              ))}
            </select>
            <button
              type="button"
              className="tf-btn tf-btn--ghost"
              style={{ height: 36, padding: "0 10px" }}
              title="Swap the two environments"
              aria-label="Swap the two environments"
              disabled={!to}
              onClick={() => {
                setFrom(to);
                setTo(from);
              }}
            >
              <ArrowRight className="size-4" aria-hidden />
            </button>
            <select aria-label="To environment" className={`${field} min-w-28 font-medium`} value={to} onChange={(e) => setTo(e.target.value)}>
              <option value="" disabled>
                Choose…
              </option>
              {envs.map((e) => (
                <option key={e}>{e}</option>
              ))}
            </select>
            <span className="mx-1 hidden h-6 w-px bg-[var(--tf-color-border)] sm:block" aria-hidden />
            <div className="relative min-w-40 flex-1">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" aria-hidden />
              <input aria-label="Search resources" placeholder="Search resources" className={`${field} w-full pl-8`} value={query} onChange={(e) => setQuery(e.target.value)} />
            </div>
            <select aria-label="Stack" className={field} value={stack} onChange={(e) => setStack(e.target.value)}>
              <option value="">All stacks</option>
              {stacks.map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
            {loading && <Loader2 className="size-4 animate-spin text-[var(--tf-color-action)]" aria-label="Comparing" />}
          </div>
          <div role="tablist" aria-label="What to compare" className="mt-3 inline-flex gap-1 rounded-full border border-[var(--tf-color-border)] bg-[var(--tf-color-surface-subtle)] p-1">
            {([
              { id: "repo", label: "Repository", hint: "What is committed on the repository's branch" },
              { id: "drafts", label: "Canvas drafts", hint: "Work on each environment's canvas that is not published yet" },
            ] as const).map((t) => (
              <button
                key={t.id}
                role="tab"
                aria-selected={source === t.id}
                title={t.hint}
                onClick={() => setSource(t.id)}
                className={`rounded-full px-4 py-1 text-sm font-medium transition ${source === t.id ? "bg-[var(--tf-color-text)] text-[var(--tf-color-canvas)]" : "text-[var(--tf-color-text-secondary)] hover:text-[var(--tf-color-text)]"}`}
              >
                {t.label}
              </button>
            ))}
          </div>
          {sameEnv && <p className="m-0 mt-3 text-sm text-[var(--tf-color-text-secondary)]">Choose two different environments to compare.</p>}
        </section>

        {copyMsg && (
          <p role="alert" className="m-0 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
            {copyMsg}
          </p>
        )}
        {error && (
          <p role="alert" className="m-0 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
            {error}
          </p>
        )}

        {loading && !diff && !sameEnv && (
          <div role="status" aria-label="Comparing" className="grid gap-3">
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
              {[0, 1, 2, 3, 4].map((i) => (
                <span key={i} className="tf-skel" style={{ height: 64, borderRadius: 12, animationDelay: `${i * 80}ms` }} />
              ))}
            </div>
            {[0, 1].map((i) => (
              <span key={i} className="tf-skel" style={{ height: 110, borderRadius: 12 }} />
            ))}
          </div>
        )}

        {diff && (
          <div className={`space-y-5 transition-opacity ${loading ? "opacity-60" : ""}`} aria-busy={loading || undefined}>
            <div role="status" className="grid grid-cols-2 gap-3 sm:grid-cols-5">
              {tiles.map((t) => {
                const s = STATUS[t.status as DiffStatus];
                const on = filter === t.key;
                return (
                  <button
                    key={t.key}
                    type="button"
                    aria-pressed={on}
                    onClick={() => setFilter(on ? "all" : t.key)}
                    className="rounded-xl border bg-[color-mix(in_srgb,var(--tf-color-surface)_90%,transparent)] backdrop-blur-sm p-3 text-left transition hover:shadow-[var(--tf-shadow-rest)]"
                    style={{ borderColor: on ? s.tone : "var(--tf-color-border)", background: on ? s.surface : undefined }}
                  >
                    <div className="text-2xl font-semibold leading-none tabular-nums" style={{ color: t.n > 0 && t.key !== "same" ? s.tone : "var(--tf-color-text)" }}>
                      {t.n}
                    </div>
                    <div className="mt-1.5 truncate text-xs text-[var(--tf-color-text-secondary)]">{t.label}</div>
                  </button>
                );
              })}
            </div>

            {diff.summary.unknown > 0 && (
              <div role="note" className="flex gap-2.5 rounded-xl bg-[var(--tf-color-neutral-surface)] px-4 py-3 text-sm">
                <CircleHelp className="mt-0.5 size-4 shrink-0 text-[var(--tf-color-text-muted)]" aria-hidden />
                <p className="m-0">
                  {diff.summary.unknown} resources cannot be compared: a value in {diff.from} or {diff.to} could not be worked out (a variable with no value there, or a reference). They are listed as "Cannot tell", never as the same. Check that both environments have their values file and the project variables they use.
                </p>
              </div>
            )}
            {!!diff.violations?.length && (
              <div role="alert" className="flex gap-2.5 rounded-xl border border-[var(--tf-color-warning)] bg-[var(--tf-color-warning-surface)] px-4 py-3 text-sm">
                <TriangleAlert className="mt-0.5 size-4 shrink-0 text-[var(--tf-color-warning)]" aria-hidden />
                <div>
                  <strong>Differences this project's checks do not allow</strong>
                  <ul className="m-0 mt-1 list-disc pl-5">
                    {diff.violations.map((v, i) => (
                      <li key={i}>{v.message}</li>
                    ))}
                  </ul>
                </div>
              </div>
            )}

            {source === "repo" && diff.resources.length === 0 && (
              <div role="note" className="flex flex-wrap items-center gap-3 rounded-xl bg-[var(--tf-color-info-surface)] px-4 py-3 text-sm">
                <CircleHelp className="size-4 shrink-0 text-[var(--tf-color-info)]" aria-hidden />
                <p className="m-0 min-w-0 flex-1">There are no resources on the repository's branch for {diff.from} or {diff.to}. Work that is still on the canvas is not in the repository yet.</p>
                <button type="button" className="tf-btn" style={{ height: 30, padding: "0 12px", fontSize: 12 }} onClick={() => setSource("drafts")}>Compare canvas drafts</button>
              </div>
            )}
            {grouped.length === 0 ? (
              <div className="rounded-xl border border-dashed border-[var(--tf-color-border-strong)] px-6 py-10 text-center">
                <CheckCircle2 className="mx-auto size-8 text-[var(--tf-color-success)]" aria-hidden />
                <p className="m-0 mt-2 text-sm font-medium">{filter === "differs" && !q && !stack ? "No differences between these environments." : "Nothing matches these filters."}</p>
                {filter !== "all" && (
                  <button type="button" className="mt-2 text-sm underline" onClick={() => { setFilter("all"); setQuery(""); setStack(""); }}>
                    Show everything
                  </button>
                )}
              </div>
            ) : (
              grouped.map(([name, list]) => (
                <section key={name} aria-label={`Stack ${name}`}>
                  <h3 className="m-0 mb-2 flex items-center gap-2 text-sm font-semibold">
                    <Layers className="size-4 text-[var(--tf-color-text-muted)]" aria-hidden /> {name}
                    <span className="rounded-full bg-[var(--tf-color-surface-subtle)] px-2 py-0.5 text-xs font-normal text-[var(--tf-color-text-secondary)]">{list.length}</span>
                  </h3>
                  <ul className="m-0 grid list-none gap-3 p-0">
                    {list.map((r) => (
                      <ResourceCard key={r.id} r={r} from={diff.from} to={diff.to} onCopy={onCopy && source === "repo" ? () => copy(r.id) : undefined} />
                    ))}
                  </ul>
                </section>
              ))
            )}

            {diff.variables.length > 0 && (
              <section aria-label="Variables with a value in only one environment" className="rounded-xl border border-[var(--tf-color-border)] bg-[color-mix(in_srgb,var(--tf-color-surface)_90%,transparent)] backdrop-blur-sm p-4">
                <h3 className="m-0 text-sm font-semibold">Variables with a value in only one environment</h3>
                <ul className="m-0 mt-2 grid list-none gap-1 p-0 font-mono text-xs">
                  {diff.variables.map((v) => (
                    <li key={v.name}>
                      <strong>{v.name}</strong>: {v.has === "from" ? `${diff.from} = ${v.from}, ${diff.to} has none` : `${diff.to} = ${v.to}, ${diff.from} has none`}
                    </li>
                  ))}
                </ul>
              </section>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function ResourceCard({ r, from, to, onCopy }: { r: ResourceDiff; from: string; to: string; onCopy?: () => void }) {
  const s = STATUS[r.status];
  const Icon = s.icon;
  const word = r.status === "only_from" ? `Only in ${from}` : r.status === "only_to" ? `Only in ${to}` : r.status === "differs" ? "Differs" : r.status === "unknown" ? "Cannot tell" : "Same";
  return (
    <li className="overflow-hidden rounded-xl border border-[var(--tf-color-border)] bg-[color-mix(in_srgb,var(--tf-color-surface)_90%,transparent)] backdrop-blur-sm" style={{ borderLeft: `3px solid ${s.tone}` }}>
      <div className="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
        <div className="min-w-0">
          <div className="truncate font-mono text-sm font-semibold">{r.label || r.type}</div>
          <div className="text-xs text-[var(--tf-color-text-muted)]">{r.kind === "module" ? r.type : `resource ${r.type}`}</div>
        </div>
        <div className="flex items-center gap-2">
          {r.status === "only_from" && r.kind === "module" && onCopy && (
            <button type="button" onClick={onCopy} className="tf-btn tf-btn--ghost" style={{ height: 28, padding: "0 10px", fontSize: 12 }}>
              <Copy className="size-3.5" aria-hidden /> Copy to {to}
            </button>
          )}
          <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold" style={{ color: s.tone, background: s.surface }}>
            <Icon className="size-3.5" aria-hidden /> {word}
          </span>
        </div>
      </div>
      {r.args && r.args.length > 0 && (
        <div className="overflow-x-auto border-t border-[var(--tf-color-border)]">
          <table className="w-full min-w-[520px] border-collapse text-left text-xs">
            <thead>
              <tr className="bg-[var(--tf-color-surface-subtle)] text-[var(--tf-color-text-muted)]">
                <th className="w-1/4 px-4 py-2 font-medium">Argument</th>
                <th className="w-[37.5%] px-3 py-2 font-medium">{from}</th>
                <th className="w-[37.5%] px-3 py-2 font-medium">{to}</th>
              </tr>
            </thead>
            <tbody>
              {r.args.map((a) => {
                const unknown = a.status === "unknown";
                return (
                  <tr key={a.name} className="border-t border-[var(--tf-color-border)] align-top">
                    <td className="px-4 py-2 font-mono font-medium">{a.name}</td>
                    <td className="px-3 py-2 font-mono break-all" style={a.status === "differs" || a.status === "only_from" ? { background: "color-mix(in srgb, var(--tf-color-error) 8%, transparent)" } : undefined}>
                      {unknown ? <span className="italic text-[var(--tf-color-text-muted)]">{showValue(a.from)}</span> : showValue(a.from)}
                    </td>
                    <td className="px-3 py-2 font-mono break-all" style={a.status === "differs" || a.status === "only_to" ? { background: "color-mix(in srgb, var(--tf-color-success) 10%, transparent)" } : undefined}>
                      {unknown ? <span className="italic text-[var(--tf-color-text-muted)]">{showValue(a.to)}</span> : showValue(a.to)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </li>
  );
}
