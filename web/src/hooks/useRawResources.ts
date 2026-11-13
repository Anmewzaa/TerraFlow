import { useCallback, useEffect, useState } from "react";
import { catalogPaths } from "../lib/catalogApi";

/** A raw provider resource an admin prepared (D-041). */
export interface RawResourceTemplate {
  name: string;
  title: string;
  resource_type: string;
  description: string;
  default_values: Record<string, unknown>;
  target_folder: string;
  /** The cloud provider (azurerm), its palette category, and the provider versions (major.minor) it exists in. */
  provider?: string;
  category?: string;
  min_version?: string;
  max_version?: string;
  /** "project" for a template of the project's own catalog (D-060). */
  scope?: "global" | "project";
  shadows_global?: boolean;
}

export function useRawResources(projectId?: string | null) {
  const [templates, setTemplates] = useState<RawResourceTemplate[]>([]);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(() => {
    fetch(`/api${catalogPaths(projectId).resources}`)
      .then((r) => (r.ok ? r.json() : []))
      .then((list: RawResourceTemplate[]) => setTemplates(list))
      .catch(() => setTemplates([]))
      .finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => {
    reload();
  }, [reload]);

  return { templates, loading, reload };
}
