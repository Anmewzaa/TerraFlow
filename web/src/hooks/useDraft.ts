import { useCallback, useEffect, useRef, useState } from "react";

export function useDraft(projectId: string | null, moduleId: string | null) {
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [loaded, setLoaded] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!projectId || !moduleId) return;
    fetch(`/api/projects/${projectId}/drafts/${moduleId}`)
      .then((r) => {
        if (!r.ok) return {};
        return r.json().then((d) => d.values ?? {});
      })
      .then((v) => {
        setValues(v);
        setLoaded(true);
      })
      .catch(() => setLoaded(true));
  }, [projectId, moduleId]);

  const save = useCallback(
    (newValues: Record<string, unknown>) => {
      if (!projectId || !moduleId) return;
      setValues(newValues);
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => {
        fetch(`/api/projects/${projectId}/drafts/${moduleId}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(newValues),
        }).catch(() => {});
      }, 1500);
    },
    [projectId, moduleId]
  );

  return { values, save, loaded };
}
