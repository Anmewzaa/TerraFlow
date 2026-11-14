import { useCallback, useEffect, useState } from "react";
import { errorOf } from "./useGitHub";

export interface DriftItem {
  key: string;
  kind: "unrepresented" | "variable_differs" | "file_changed" | "file_added" | "file_removed";
  folder?: string;
  file?: string;
  label?: string;
  field?: string;
  node_id?: string;
  message: string;
  importable?: boolean;
}

export interface Drift {
  env: string;
  has_baseline: boolean;
  baseline_at?: string;
  items: DriftItem[];
  dismissed: (DriftItem & { note: string; by: string })[];
}

/** Repository drift (GET /projects/{id}/drift): what the repository holds that TerraFlow does not show or did not write. */
export function useDrift(projectId: string | null, env: string) {
  const [drift, setDrift] = useState<Drift | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    if (!projectId) return;
    let off = false;
    fetch(`/api/projects/${encodeURIComponent(projectId)}/drift?env=${encodeURIComponent(env)}`)
      .then(async (r) => {
        if (!r.ok) throw new Error(await errorOf(r));
        return (await r.json()) as Drift;
      })
      .then((d) => {
        if (off) return;
        setDrift(d);
        setError(null);
      })
      .catch((e) => !off && setError(e instanceof Error ? e.message : "Cannot read the drift"));
    return () => {
      off = true;
    };
  }, [projectId, env, tick]);

  const act = useCallback(
    async (path: string, init: RequestInit) => {
      const r = await fetch(`/api/projects/${encodeURIComponent(projectId ?? "")}/drift/${path}`, init);
      if (!r.ok) throw new Error(await errorOf(r));
      setTick((t) => t + 1);
    },
    [projectId],
  );
  return {
    drift,
    error,
    refresh: () => setTick((t) => t + 1),
    dismiss: (key: string, note: string) =>
      act("dismiss", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ key, note }) }),
    restore: (key: string) => act(`dismiss?key=${encodeURIComponent(key)}`, { method: "DELETE" }),
    acceptBaseline: () => act(`baseline?env=${encodeURIComponent(env)}`, { method: "POST" }),
  };
}

/** A short label for the kind of an item. */
export const KIND_LABEL: Record<DriftItem["kind"], string> = {
  unrepresented: "Not shown exactly",
  variable_differs: "Differs from variable",
  file_changed: "File changed",
  file_added: "File added",
  file_removed: "File removed",
};
