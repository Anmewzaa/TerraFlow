import { useCallback, useState } from "react";
import type { GenerateOutput } from "../types/changeset";

export function useHclPreview(projectId: string | null, changesetId: string | null) {
  const [preview, setPreview] = useState<GenerateOutput | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadPreview = useCallback(async () => {
    if (!projectId) return;
    setLoading(true);
    setError(null);
    try {
      if (!changesetId) {
        // No draft yet: still show the project's current code.
        const res = await fetch(`/api/projects/${projectId}/files`);
        if (!res.ok) throw new Error("Failed to load the project files");
        const data = await res.json();
        setPreview({ files: [], base_files: [], repo_files: data.repo_files ?? [] });
        return;
      }
      const res = await fetch(
        `/api/projects/${projectId}/changesets/${changesetId}/preview`
      );
      if (!res.ok) {
        let msg = "Failed to load preview";
        try {
          const b = await res.json();
          if (b?.error) msg = String(b.error);
        } catch {
          /* not JSON */
        }
        throw new Error(msg);
      }
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
