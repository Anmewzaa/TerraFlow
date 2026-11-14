import { useEffect, useState } from "react";

export type DiffStatus = "same" | "differs" | "unknown" | "only_from" | "only_to";

export interface ArgDiff {
  name: string;
  from?: unknown;
  to?: unknown;
  status: Exclude<DiffStatus, "same">;
}

export interface ResourceDiff {
  id: string;
  stack: string;
  kind: string;
  type: string;
  label?: string;
  status: DiffStatus;
  args?: ArgDiff[];
}

export interface EnvDiff {
  from: string;
  to: string;
  resources: ResourceDiff[];
  variables: { name: string; from?: string; to?: string; has: "from" | "to" }[];
  violations?: { resource: string; field?: string; message: string }[];
  summary: { same: number; differs: number; unknown: number; only_from: number; only_to: number };
}

/** The server's comparison of two environments (GET /projects/{id}/env-diff); read-only. */
export function useEnvDiff(projectId: string | null, from: string, to: string) {
  const [diff, setDiff] = useState<EnvDiff | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    if (!projectId || !from || !to || from === to) {
      setDiff(null);
      return;
    }
    const ctrl = new AbortController();
    setLoading(true);
    fetch(`/api/projects/${encodeURIComponent(projectId)}/env-diff?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, { signal: ctrl.signal })
      .then(async (r) => {
        const body = await r.json().catch(() => ({}));
        if (!r.ok) throw new Error(body?.error ?? `Could not compare (${r.status})`);
        setDiff(body as EnvDiff);
        setError(null);
      })
      .catch((e) => {
        if (e?.name === "AbortError") return;
        setDiff(null);
        setError(e instanceof Error ? e.message : "Could not compare");
      })
      .finally(() => !ctrl.signal.aborted && setLoading(false));
    return () => ctrl.abort();
  }, [projectId, from, to]);
  return { diff, error, loading };
}

/** Filters of the compare screen: only differences, and one stack. */
export function filterDiff(rows: ResourceDiff[], onlyDiffs: boolean, stack: string): ResourceDiff[] {
  return rows.filter((r) => (!onlyDiffs || r.status !== "same") && (!stack || r.stack === stack));
}

export function showValue(v: unknown): string {
  if (v === undefined) return "—";
  if (v && typeof v === "object" && "$unknown" in (v as object)) return "unknown";
  return typeof v === "string" ? v : JSON.stringify(v);
}
