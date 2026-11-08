import { createContext, useContext } from "react";
import type { ProjectGraph } from "../types/graph";

export const GraphContext = createContext<ProjectGraph | null>(null);

export function useGraphContext(): ProjectGraph | null {
  return useContext(GraphContext);
}
