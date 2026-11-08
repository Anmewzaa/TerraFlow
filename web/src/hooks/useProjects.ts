import { useState, useEffect, useCallback } from "react";
import type { Project } from "../types/project";

async function errorOf(res: Response): Promise<string> {
  try {
    const body = await res.json();
    if (body?.error) return String(body.error);
  } catch {
    /* non-JSON body */
  }
  return `Request failed (${res.status})`;
}

export function useProjects() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const r = await fetch("/api/projects");
      if (!r.ok) throw new Error(await errorOf(r));
      setProjects(await r.json());
      setError(null);
    } catch (e) {
      setProjects([]);
      setError(e instanceof Error ? e.message : "Cannot load projects");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  /** Returns the created project, or throws with a user-facing message. */
  const create = useCallback(
    async (name: string, iacPath?: string): Promise<Project> => {
      const r = await fetch("/api/projects", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, iac_path: iacPath || undefined }),
      });
      if (!r.ok) throw new Error(await errorOf(r));
      const p: Project = await r.json();
      await refresh();
      return p;
    },
    [refresh],
  );

  const remove = useCallback(
    async (id: string) => {
      const r = await fetch(`/api/projects/${id}`, { method: "DELETE" });
      if (!r.ok) throw new Error(await errorOf(r));
      await refresh();
    },
    [refresh],
  );

  return { projects, loading, error, refresh, create, remove };
}
