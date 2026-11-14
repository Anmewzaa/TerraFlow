import { createContext, useContext } from "react";
import type { ProjectGraph } from "../types/graph";
import type { ModuleSummary } from "../types/catalog";
import type { CanvasNode } from "../types/changeset";

interface GraphContextValue {
  graph: ProjectGraph | null;
  draftNodes: CanvasNode[];
  /** The catalog, so a field that asks for a link can find the modules that offer it (8.11). */
  modules?: ModuleSummary[];
}

export const GraphContext = createContext<GraphContextValue>({ graph: null, draftNodes: [], modules: [] });

export function useGraphContext(): GraphContextValue {
  return useContext(GraphContext);
}
