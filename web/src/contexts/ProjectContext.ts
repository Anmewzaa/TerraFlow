import { createContext, useContext } from "react";

/** The open project's id (null on Home), so a form can read the catalog that project sees (D-060). */
export const ProjectContext = createContext<string | null>(null);

export const useProjectId = () => useContext(ProjectContext);
