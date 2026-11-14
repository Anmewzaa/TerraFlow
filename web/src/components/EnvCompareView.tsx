import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { filterDiff, showValue, useEnvDiff, type DiffStatus } from "../hooks/useEnvDiff";

const LABEL: Record<DiffStatus, string> = {
  same: "Same",
  differs: "Differs",
  unknown: "Cannot tell",
  only_from: "Only here",
  only_to: "Only there",
};
const TONE: Record<DiffStatus, string> = {
  same: "text-[var(--tf-color-text-muted)]",
  differs: "text-[var(--tf-color-warning)]",
  unknown: "text-[var(--tf-color-warning)]",
  only_from: "text-[var(--tf-color-action)]",
  only_to: "text-[var(--tf-color-action)]",
};

const select =
  "rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]";

/** Two environments side by side (8.10). Read-only: it reads the repository and changes nothing. */
export function EnvCompareView({ projectId, envs, current, onCopy }: { projectId: string; envs: string[]; current: string; onCopy?: (node: unknown, env: string) => void }) {
  const [copyMsg, setCopyMsg] = useState<string | null>(null);
  const copy = async (id: string) => {
    setCopyMsg(null);
    const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/env-copy?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&id=${encodeURIComponent(id)}`);
    const body = await res.json().catch(() => ({}));
    if (!res.ok) return setCopyMsg(body?.error ?? `Could not copy (${res.status})`);
    onCopy?.(body.node, body.env);
  };
  const [from, setFrom] = useState(current);
  const [to, setTo] = useState(envs.find((e) => e !== current) ?? "");
  const [onlyDiffs, setOnlyDiffs] = useState(true);
  const [stack, setStack] = useState("");
  const { diff, error, loading } = useEnvDiff(projectId, from, to);
  const stacks = useMemo(() => [...new Set((diff?.resources ?? []).map((r) => r.stack))].sort(), [diff]);
  const rows = filterDiff(diff?.resources ?? [], onlyDiffs, stack);

  return (
    <div className="flex-1 overflow-y-auto p-6 md:p-8">
      <div className="mx-auto max-w-5xl">
        <h2 className="m-0 text-xl font-semibold">Compare environments</h2>
        <p className="m-0 mt-1 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">
          Reads both environments from the repository and pairs resources by their stack and name. A value that cannot be worked out is shown as unknown, never as equal.
        </p>
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <select aria-label="From environment" className={select} value={from} onChange={(e) => setFrom(e.target.value)}>
            {envs.map((e) => (
              <option key={e}>{e}</option>
            ))}
          </select>
          <span aria-hidden>→</span>
          <select aria-label="To environment" className={select} value={to} onChange={(e) => setTo(e.target.value)}>
            <option value="" disabled>
              Choose…
            </option>
            {envs.map((e) => (
              <option key={e}>{e}</option>
            ))}
          </select>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={onlyDiffs} onChange={(e) => setOnlyDiffs(e.target.checked)} /> Only differences
          </label>
          <select aria-label="Stack" className={select} value={stack} onChange={(e) => setStack(e.target.value)}>
            <option value="">All stacks</option>
            {stacks.map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
          {loading && <Loader2 className="size-4 animate-spin" aria-label="Comparing" />}
        </div>
        {from === to && <p className="mt-4 text-sm text-[var(--tf-color-text-secondary)]">Choose two different environments.</p>}
        {copyMsg && (
          <p role="alert" className="mt-4 text-sm text-[var(--tf-color-error)]">
            {copyMsg}
          </p>
        )}
        {error && (
          <p role="alert" className="mt-4 text-sm text-[var(--tf-color-error)]">
            {error}
          </p>
        )}
        {diff && (
          <>
            <p className="mt-4 text-sm" role="status">
              {diff.summary.differs} differ · {diff.summary.only_from} only in {diff.from} · {diff.summary.only_to} only in {diff.to} · {diff.summary.unknown} cannot be told · {diff.summary.same} the same
            </p>
            {diff.summary.unknown > 0 && (
              <p role="note" className="mt-3 rounded-lg bg-[var(--tf-color-surface-subtle)] px-3 py-2 text-sm">
                {diff.summary.unknown} resources cannot be compared: a value in {diff.from} or {diff.to} could not be worked out (a variable with no value there, or a reference).
                They are listed as "Cannot tell", never as the same. Check that both environments have their values file and the project variables they use.
              </p>
            )}
            {!!diff.violations?.length && (
              <div role="alert" className="mt-3 rounded-lg border border-[var(--tf-color-warning)] px-3 py-2.5 text-sm">
                <strong>Differences this project's checks do not allow:</strong>
                <ul className="m-0 mt-1 list-disc pl-5">
                  {diff.violations.map((v, i) => (
                    <li key={i}>{v.message}</li>
                  ))}
                </ul>
              </div>
            )}
            <ul className="m-0 mt-3 list-none space-y-3 p-0">
              {rows.map((r) => (
                <li key={r.id} className="rounded-xl border border-[var(--tf-color-border)] p-4">
                  <div className="flex flex-wrap items-baseline justify-between gap-2">
                    <span className="font-mono text-sm font-semibold">
                      {r.stack} / {r.label || r.type}
                    </span>
                    {r.status === "only_from" && r.kind === "module" && onCopy && (
                      <button type="button" onClick={() => copy(r.id)} className="rounded-md border border-[var(--tf-color-border-strong)] px-2 py-1 text-xs hover:bg-[var(--tf-glass-hover)]">
                        Copy to {diff.to}
                      </button>
                    )}
                    <span className={`text-xs font-semibold ${TONE[r.status]}`}>
                      {r.status === "only_from" ? `Only in ${diff.from}` : r.status === "only_to" ? `Only in ${diff.to}` : LABEL[r.status]}
                    </span>
                  </div>
                  {r.args && r.args.length > 0 && (
                    <table className="mt-2 w-full text-left text-xs">
                      <thead className="text-[var(--tf-color-text-muted)]">
                        <tr>
                          <th className="py-1 pr-3">Argument</th>
                          <th className="py-1 pr-3">{diff.from}</th>
                          <th className="py-1">{diff.to}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {r.args.map((a) => (
                          <tr key={a.name} className={a.status === "unknown" ? "bg-[var(--tf-color-surface-subtle)]" : ""}>
                            <td className="py-1 pr-3 font-mono">{a.name}</td>
                            <td className="py-1 pr-3 font-mono">{showValue(a.from)}</td>
                            <td className="py-1 font-mono">{showValue(a.to)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </li>
              ))}
              {rows.length === 0 && <li className="text-sm text-[var(--tf-color-text-secondary)]">Nothing to show with these filters.</li>}
            </ul>
            {diff.variables.length > 0 && (
              <div className="mt-6">
                <h3 className="m-0 text-base font-semibold">Variables with a value in only one environment</h3>
                <ul className="m-0 mt-2 list-none p-0 font-mono text-xs">
                  {diff.variables.map((v) => (
                    <li key={v.name}>
                      {v.name}: {v.has === "from" ? `${diff.from} = ${v.from}, ${diff.to} has none` : `${diff.to} = ${v.to}, ${diff.from} has none`}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
