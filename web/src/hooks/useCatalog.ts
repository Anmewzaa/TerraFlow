import { useState, useEffect } from "react";
import type { ModuleSummary } from "../types/catalog";

export function useCatalog() {
  const [modules, setModules] = useState<ModuleSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/catalog")
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
        setModules(body);
        setError(null);
      })
      .catch((e) => {
        setModules([]);
        setError(e instanceof Error ? e.message : "cannot load the catalog");
      })
      .finally(() => setLoading(false));
  }, []);

  return { modules, loading, error };
}
