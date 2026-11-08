import { useState, useEffect } from "react";
import type { CatalogModule } from "../types/catalog";

export function useModuleSchema(name: string | null, version: string | null) {
  const [module, setModule] = useState<CatalogModule | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!name || !version) {
      setModule(null);
      return;
    }
    setLoading(true);
    setError(null);
    fetch(`/api/catalog/${name}/${version}`)
      .then((r) => {
        if (!r.ok) throw new Error("Module not found");
        return r.json();
      })
      .then(setModule)
      .catch(setError)
      .finally(() => setLoading(false));
  }, [name, version]);

  return { module, loading, error };
}
