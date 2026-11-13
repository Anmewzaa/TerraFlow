import { useState, useEffect } from "react";
import type { ModuleSummary } from "../types/catalog";
import { catalogPaths } from "../lib/catalogApi";

/** The modules to build from: the global catalog, plus the open project's own (D-060). */
export function useCatalog(projectId?: string | null) {
  const [modules, setModules] = useState<ModuleSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let off = false;
    fetch(`/api${catalogPaths(projectId).read}`)
      .then(async (r) => {
        // An error body is an object, not a list: never feed it to the panel.
        if (!r.ok) {
          let reason = "";
          try {
            reason = (await r.json())?.error ?? "";
          } catch {
            /* no body */
          }
          throw new Error(reason || `HTTP ${r.status}`);
        }
        const body = await r.json();
        if (!Array.isArray(body)) throw new Error("unexpected answer");
        if (off) return;
        setModules(body);
        setError(null);
      })
      .catch((e) => {
        if (off) return;
        setModules([]);
        setError(e instanceof Error ? e.message : "cannot load the catalog");
      })
      .finally(() => !off && setLoading(false));
    return () => {
      off = true;
    };
  }, [projectId, tick]);

  return { modules, loading, error, reload: () => setTick((t) => t + 1) };
}
