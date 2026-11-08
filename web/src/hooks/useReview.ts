import { useState, useEffect, useCallback } from "react";
import type { Changeset } from "../types/changeset";

export interface ReviewDiffFile {
  path: string;
  status: "added" | "conflict";
  content: string;
  lines_added: number;
}

export interface ReviewNode {
  id: string;
  module_name: string;
  module_version: string;
  instance_name: string;
  target_folder: string;
  values: Record<string, unknown>;
}

export interface ReviewDetail {
  changeset: Changeset & { branch_name?: string | null; commit_sha?: string | null };
  branch: string;
  canvas_diff: { added_nodes: ReviewNode[]; edges: { id: string; from: string; to: string; kind: string }[] };
  hcl_diff: ReviewDiffFile[];
  conflicts: string[];
}

export interface AuditEntry {
  id: number;
  user_oid: string;
  action: string;
  resource: string;
  detail: Record<string, unknown> | null;
  created_at: string;
}

export type ChangesetRow = Changeset & { branch_name?: string | null; commit_sha?: string | null };

async function errorOf(res: Response): Promise<string> {
  try {
    const b = await res.json();
    if (b?.error) return String(b.error);
  } catch {
    /* not JSON */
  }
  return `Request failed (${res.status})`;
}

/** All changesets of the project (own drafts + everything published/reviewed). */
export function useChangesetList(projectId: string, refreshKey: number) {
  const [rows, setRows] = useState<ChangesetRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      const r = await fetch(`/api/projects/${projectId}/changesets/`);
      if (!r.ok) throw new Error(await errorOf(r));
      setRows((await r.json()) ?? []);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Cannot load changesets");
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    reload();
  }, [reload, refreshKey]);

  return { rows, loading, error, reload };
}

export function useReviewDetail(projectId: string, csId: string | null, refreshKey: number) {
  const [detail, setDetail] = useState<ReviewDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!csId) {
      setDetail(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    fetch(`/api/projects/${projectId}/changesets/${csId}/review`)
      .then(async (r) => {
        if (!r.ok) throw new Error(await errorOf(r));
        return r.json() as Promise<ReviewDetail>;
      })
      .then((d) => {
        if (!cancelled) {
          setDetail(d);
          setError(null);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setDetail(null);
          setError(e instanceof Error ? e.message : "Cannot load review");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [projectId, csId, refreshKey]);

  return { detail, loading, error };
}

/** Approve / reject. Throws an Error carrying the server's message. */
export async function decide(
  projectId: string,
  csId: string,
  action: "approve" | "reject",
  reason?: string,
): Promise<{ branch?: string; commit?: string; repo?: string }> {
  const r = await fetch(`/api/projects/${projectId}/changesets/${csId}/${action}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: action === "reject" ? JSON.stringify({ reason }) : undefined,
  });
  if (!r.ok) throw new Error(await errorOf(r));
  return r.json();
}

export function useAudit(projectId: string, refreshKey: number) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetch(`/api/projects/${projectId}/audit?limit=200`)
      .then(async (r) => {
        if (!r.ok) throw new Error(await errorOf(r));
        return r.json() as Promise<AuditEntry[]>;
      })
      .then((d) => {
        if (!cancelled) {
          setEntries(d ?? []);
          setError(null);
        }
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Cannot load audit log");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [projectId, refreshKey]);

  return { entries, loading, error };
}

/** Open an approved or rejected changeset as a new editable draft. */
export async function reviseChangeset(projectId: string, csId: string): Promise<string> {
  const r = await fetch(`/api/projects/${projectId}/changesets/${csId}/revise`, { method: "POST" });
  if (!r.ok) throw new Error(await errorOf(r));
  return (await r.json()).id as string;
}
