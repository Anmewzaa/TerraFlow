import { useCallback, useState } from "react";
import type { GenerateOutput } from "../types/changeset";

export function useHclPreview(projectId: string | null, changesetId: string | null) {
  const [preview, setPreview] = useState<GenerateOutput | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadPreview = useCallback(async () => {
    if (!projectId || !changesetId) return;
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${projectId}/changesets/${changesetId}/preview`
      );
      if (!res.ok) throw new Error("Failed to load preview");
      const data = await res.json();
      setPreview(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [projectId, changesetId]);

  return { preview, loading, error, loadPreview };
}
