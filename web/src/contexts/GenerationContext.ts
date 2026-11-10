import { createContext, useContext } from "react";
import { EMPTY_SETTINGS, type GenerationSettings } from "../lib/generation";

/** The open project's generation settings, for forms and the canvas (no prop drilling). */
export const GenerationContext = createContext<GenerationSettings>(EMPTY_SETTINGS);

export const useGenerationSettings = () => useContext(GenerationContext);
