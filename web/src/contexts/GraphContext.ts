import { createContext, useContext } from "react";
import type { ProjectGraph } from "../types/graph";
import type { CanvasNode } from "../types/changeset";

interface GraphContextValue {
  graph: ProjectGraph | null;
  draftNodes: CanvasNode[];
}

export const GraphContext = createContext<GraphContextValue>({ graph: null, draftNodes: [] });

export function useGraphContext(): GraphContextValue {
  return useContext(GraphContext);
}
