import { useCallback, useEffect, useState } from "react";
import type { Project } from "../types/project";

/* GitHub connection, repository picker and the project team (Phase 8, M1). Every
 * call returns the server's own message on failure so a screen can show it. */

export async function errorOf(res: Response): Promise<string> {
  try {
    const body = await res.json();
    if (body?.error) return String(body.error);
  } catch {
    /* non-JSON body */
  }
  // No message from the server: say which call failed, so a report can be traced.
  let where = "";
  try {
    where = ` ${new URL(res.url).pathname}`;
  } catch {
    /* res.url can be empty */
  }
  return `Request failed (${res.status})${where}`;
}

async function json<T>(path: string, init?: Omit<RequestInit, "body"> & { body?: unknown }): Promise<T> {
  const { body, ...rest } = init ?? {};
  const res = await fetch(`/api${path}`, {
    ...rest,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(await errorOf(res));
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export interface GitHubStatus {
  configured: boolean;
  install_url?: string;
}

export interface RepoChoice {
  connection_id: string;
  account: string;
  repo_id: string;
  full_name: string;
  default_branch: string;
  private: boolean;
}

export const getGitHubStatus = () => json<GitHubStatus>("/github/status");
export const getRepos = () => json<RepoChoice[]>("/github/repos");
export const getFolders = (connectionId: string, repoId: string) =>
  json<{ folders: string[]; default_branch: string }>(
    `/github/folders?connection_id=${encodeURIComponent(connectionId)}&repo_id=${encodeURIComponent(repoId)}`,
  );

export interface FromRepoInput {
  name: string;
  description?: string;
  connection_id: string;
  repo_id: string;
  folder: string;
}
export const createFromRepo = (input: FromRepoInput) =>
  json<Project>("/projects/from-repo", { method: "POST", body: input });

/** Where the browser goes to authorize with GitHub (a full page navigation, not a fetch). */
export const AUTHORIZE_URL = "/api/github/authorize";

// ---- Team ----

export type Role = "owner" | "co_owner" | "contributor";

export interface TeamMember {
  user_oid: string;
  email: string;
  name: string;
  role: Role;
  added_by: string;
  created_at: string;
}

export interface Team {
  members: TeamMember[];
  /** "" when the caller is an admin who is not a member. */
  my_role: Role | "";
  second_reviewer: boolean;
  repository?: string;
  folder?: string;
}

export const ROLE_LABEL: Record<Role, string> = { owner: "Owner", co_owner: "Co-owner", contributor: "Contributor" };

/** The caller may approve and reject changesets (role only; admins are checked separately). */
export const canApproveRole = (role: Team["my_role"] | undefined) => role === "owner" || role === "co_owner";

/** The team of one project, with actions that reload it. */
export function useTeam(projectId: string | null) {
  const [team, setTeam] = useState<Team | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    if (!projectId) return;
    let cancelled = false;
    json<Team>(`/projects/${encodeURIComponent(projectId)}/team`)
      .then((t) => {
        if (!cancelled) {
          setTeam(t);
          setError(null);
        }
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Cannot load the team");
      });
    return () => {
      cancelled = true;
    };
  }, [projectId, tick]);

  const act = useCallback(
    async (fn: () => Promise<unknown>) => {
      await fn();
      setTick((t) => t + 1);
    },
    [],
  );
  const base = `/projects/${encodeURIComponent(projectId ?? "")}`;
  return {
    team,
    error,
    invite: (user: string, role: Role) => act(() => json(`${base}/team`, { method: "POST", body: { user, role } })),
    setRole: (oid: string, role: Role) =>
      act(() => json(`${base}/team/${encodeURIComponent(oid)}`, { method: "PATCH", body: { role } })),
    remove: (oid: string) => act(() => json(`${base}/team/${encodeURIComponent(oid)}`, { method: "DELETE" })),
    setSecondReviewer: (on: boolean) =>
      act(() => json(`${base}/settings`, { method: "PUT", body: { second_reviewer: on } })),
    transfer: (to: string) => act(() => json(`${base}/transfer`, { method: "POST", body: { to } })),
    deleteProject: () => json<void>(`/projects/${encodeURIComponent(projectId ?? "")}`, { method: "DELETE" }),
  };
}

// ---- Admin: connections ----

export interface ConnectionRow {
  id: string;
  provider: string;
  account_login: string;
  account_type: string;
  status: "active" | "needs_attention" | "disconnected";
  created_by: string;
  created_at: string;
  projects: number;
}

export const getConnections = () => json<ConnectionRow[]>("/admin/connections");
export const disconnectConnection = (id: string) =>
  json<{ status: string }>(`/admin/connections/${encodeURIComponent(id)}/disconnect`, { method: "POST" });
export const recheckConnection = (id: string) =>
  json<{ status: string }>(`/admin/connections/${encodeURIComponent(id)}/recheck`, { method: "POST" });
