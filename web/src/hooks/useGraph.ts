import { useQuery } from "@tanstack/react-query";
import type { ProjectGraph } from "../types/graph";

export function useGraph(projectId: string | null, env: string) {
  return useQuery<ProjectGraph>({
    queryKey: ["graph", projectId, env],
    queryFn: async () => {
      const res = await fetch(
        `/api/projects/${projectId}/graph?env=${encodeURIComponent(env)}`
      );
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const g: ProjectGraph = await res.json();
      // An empty project serializes null slices; normalize so consumers can iterate.
      return { ...g, folders: g.folders ?? [], externals: g.externals ?? [] };
    },
    enabled: !!projectId,
  });
}
