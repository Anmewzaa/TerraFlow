import { useCallback, useEffect, useState } from "react";
import type { CanvasNode, CanvasState } from "../types/changeset";
import { errorOf } from "./useGitHub";

/** A stack template (8.15): nodes, the arrows between them, slots left open, and parameters. */
export interface StackSlot {
  name: string;
  kind: "parent" | "reference";
  node: string;
  accepts?: string;
}
export interface StackParam {
  name: string;
  node: string;
  field: string;
  default?: string;
}
export interface StackDoc {
  name: string;
  nodes: CanvasNode[];
  slots: StackSlot[];
  params: StackParam[];
}
export interface StackSummary {
  id: string;
  name: string;
  created_by: string;
  created_at: string;
}

/** Problems the server lists when a template cannot be made or applied. */
export class StackError extends Error {
  constructor(
    message: string,
    readonly problems: string[],
  ) {
    super(message);
  }
}

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`/api${path}`, init);
  if (r.status === 204) return undefined as T;
  if (!r.ok) {
    const body = await r.clone().json().catch(() => ({}));
    if (Array.isArray(body?.problems)) throw new StackError(body.error ?? "Not possible", body.problems);
    throw new Error(await errorOf(r));
  }
  return (await r.json()) as T;
}

export function useStackTemplates(projectId: string | null) {
  const [templates, setTemplates] = useState<StackSummary[]>([]);
  const [tick, setTick] = useState(0);
  useEffect(() => {
    if (!projectId) return;
    let off = false;
    call<StackSummary[]>(`/projects/${encodeURIComponent(projectId)}/stack-templates`)
      .then((l) => !off && setTemplates(l))
      .catch(() => !off && setTemplates([]));
    return () => {
      off = true;
    };
  }, [projectId, tick]);
  const base = `/projects/${encodeURIComponent(projectId ?? "")}/stack-templates`;
  const json = (body: unknown): RequestInit => ({ method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  return {
    templates,
    create: useCallback(
      async (name: string, canvas: CanvasState, selected: string[], params: StackParam[]) => {
        await call(base, json({ name, canvas, selected, params }));
        setTick((t) => t + 1);
      },
      [base],
    ),
    remove: useCallback(
      async (id: string) => {
        await call(`${base}/${encodeURIComponent(id)}`, { method: "DELETE" });
        setTick((t) => t + 1);
      },
      [base],
    ),
    get: useCallback((id: string) => call<{ template: StackDoc }>(`${base}/${encodeURIComponent(id)}`).then((t) => t.template), [base]),
    apply: useCallback((id: string, body: ApplyBody) => call<ApplyResult>(`${base}/${encodeURIComponent(id)}/apply`, json(body)), [base]),
  };
}

export interface ApplyBody {
  env: string;
  prefix: string;
  suffix: string;
  slots: Record<string, string>;
  values: Record<string, Record<string, string>>;
  draft_labels: string[];
  offset_x: number;
  offset_y: number;
}
export interface ApplyResult {
  nodes: CanvasNode[];
  env: string;
  variables: { name: string; values: Record<string, string> }[];
}

/** The canvas nodes a parameter can come from: plain single values (text or number), never a reference. */
export function parameterChoices(nodes: CanvasNode[]): { node: string; field: string; value: string }[] {
  const out: { node: string; field: string; value: string }[] = [];
  for (const n of nodes) {
    for (const [field, v] of Object.entries(n.values ?? {})) {
      if (typeof v === "string" && v !== "" && !/\{[a-z_][a-z0-9_]*\}/.test(v)) out.push({ node: n.instance_name, field, value: v });
      else if (typeof v === "number") out.push({ node: n.instance_name, field, value: String(v) });
    }
  }
  return out;
}

/** Merge the variables an applied template needs into the project's, keeping what is there. */
export function mergeVariables(
  existing: { name: string; description?: string; values?: Record<string, string> }[],
  patch: { name: string; values: Record<string, string> }[],
) {
  const out = existing.map((v) => ({ ...v, values: { ...(v.values ?? {}) } }));
  for (const p of patch) {
    const cur = out.find((v) => v.name === p.name);
    // One place owns a value: what the project already says for an environment is kept.
    if (cur) cur.values = { ...p.values, ...cur.values };
    else out.push({ name: p.name, values: { ...p.values } });
  }
  return out;
}
