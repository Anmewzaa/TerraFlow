import { useState, useEffect } from "react";
import type { CatalogModule } from "../types/catalog";
import { catalogPaths } from "../lib/catalogApi";
import { useProjectId } from "../contexts/ProjectContext";

export function useModuleSchema(name: string | null, version: string | null) {
  const projectId = useProjectId();
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
    fetch(`/api${catalogPaths(projectId).read}/${encodeURIComponent(name)}/${encodeURIComponent(version)}`)
      .then((r) => {
        if (!r.ok) throw new Error(`${name} ${version} is not in the catalog, so its fields cannot be shown here.`);
        return r.json();
      })
      .then(setModule)
      .catch(setError)
      .finally(() => setLoading(false));
  }, [name, version, projectId]);

  return { module, loading, error };
}
