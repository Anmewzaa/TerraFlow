import { useState, useEffect } from "react";
import type { ModuleSummary } from "../types/catalog";

export function useCatalog() {
  const [modules, setModules] = useState<ModuleSummary[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("/api/catalog")
      .then((r) => r.json())
      .then(setModules)
      .catch(() => setModules([]))
      .finally(() => setLoading(false));
  }, []);

  return { modules, loading };
}
