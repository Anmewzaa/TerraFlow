import { useQuery } from "@tanstack/react-query";
import type { ProjectGraph } from "../types/graph";

const API_BASE = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export function useGraph(dir: string, env: string) {
  return useQuery<ProjectGraph>({
    queryKey: ["graph", dir, env],
    queryFn: async () => {
      const res = await fetch(
        `${API_BASE}/api/graph?dir=${encodeURIComponent(dir)}&env=${encodeURIComponent(env)}`
      );
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    enabled: !!dir,
  });
}
