import { useCallback, useEffect, useState } from "react";
import { EMPTY_SETTINGS, type GenerationSettings } from "../lib/generation";

/** Read and save a project's generation settings (GET/PUT /projects/{id}/generation). */

export class SettingsError extends Error {
  constructor(
    message: string,
    readonly problems: string[],
  ) {
    super(message);
  }
}

export async function getGeneration(projectId: string): Promise<GenerationSettings> {
  const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/generation`);
  if (!res.ok) throw new Error(`Could not load the settings (${res.status})`);
  const body = (await res.json()) as Partial<GenerationSettings>;
  return { ...body, layout: body.layout ?? {}, variables: body.variables ?? [], starters: body.starters ?? [] };
}

export async function putGeneration(projectId: string, settings: GenerationSettings): Promise<GenerationSettings> {
  const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/generation`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new SettingsError(body?.error ?? `Request failed (${res.status})`, body?.problems ?? []);
  return { ...body, layout: body.layout ?? {}, variables: body.variables ?? [], starters: body.starters ?? [] };
}

/** The open project's settings; empty while loading or when it cannot be read. */
export function useGeneration(projectId: string | null) {
  const [settings, setSettings] = useState<GenerationSettings>(EMPTY_SETTINGS);
  const reload = useCallback(async () => {
    if (!projectId) return setSettings(EMPTY_SETTINGS);
    try {
      setSettings(await getGeneration(projectId));
    } catch {
      setSettings(EMPTY_SETTINGS);
    }
  }, [projectId]);
  useEffect(() => {
    reload();
  }, [reload]);
  return { settings, reload };
}

export interface LayoutProposal {
  pattern: string;
  values: string;
  environments: string[];
  folders: { path: string; stack?: string; env?: string; fits: boolean }[];
  confidence: "high" | "low" | "none";
}

/** What the server understood of the repository's folders (GET /projects/{id}/layout/detect). */
export async function detectLayout(projectId: string): Promise<LayoutProposal> {
  const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/layout/detect`);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body?.error ?? `Could not read the repository (${res.status})`);
  return body as LayoutProposal;
}

export interface LayoutCheck {
  folders: { path: string; stack?: string; env?: string; fits: boolean }[];
  misfits: number;
}

/** Which folders of the repository fit the patterns being edited (GET /projects/{id}/layout/check). */
export async function checkLayout(projectId: string, pattern: string, values: string): Promise<LayoutCheck> {
  const q = new URLSearchParams({ pattern, values });
  const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/layout/check?${q}`);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body?.error ?? `Could not check (${res.status})`);
  return body as LayoutCheck;
}
