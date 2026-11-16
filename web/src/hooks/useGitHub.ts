import { useCallback, useEffect, useState } from "react";
import type { Project } from "../types/project";

/* GitHub connection, repository picker and the project team (Phase 8, M1). Every
 * call returns the server's own message on failure so a screen can show it. */

/** An error answer from the server: its message, and a machine code when it sent one. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
  ) {
    super(message);
  }
}

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
  if (!res.ok) {
    let code: string | undefined;
    try {
      code = (await res.clone().json())?.code;
    } catch {
      /* non-JSON body */
    }
    throw new ApiError(await errorOf(res), res.status, code);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export interface GitHubStatus {
  configured: boolean;
  install_url?: string;
  /** How this server creates projects: from a GitHub repository or from a local folder (dev). */
  project_source?: "github" | "local";
}

export interface RepoChoice {
  connection_id: string;
  account: string;
  repo_id: string;
  full_name: string;
  default_branch: string;
  private: boolean;
  /** When GitHub's confirmation of this user's access runs out (RFC 3339). */
  expires_at: string;
}

export interface MyConnection {
  id: string;
  account: string;
  account_type: string;
  status: "active" | "needs_attention" | "disconnected";
  /** Projects that use this connection. */
  projects: number;
  /** Repositories GitHub confirmed for the user a moment ago. */
  repositories: number;
  expires_at?: string;
}

export const getMyConnections = () => json<MyConnection[]>("/github/connections");
export const disconnectMine = (id: string) =>
  json<{ status: string }>(`/github/connections/${encodeURIComponent(id)}/disconnect`, { method: "POST" });

export const getGitHubStatus = () => json<GitHubStatus>("/github/status");
export const getRepos = () => json<RepoChoice[]>("/github/repos");
/** The folders that hold Terraform, read on `branch` ("" or absent = the repository's default branch). */
export const getFolders = (connectionId: string, repoId: string, branch?: string) =>
  json<{ folders: string[]; default_branch: string; branch?: string }>(
    `/github/folders?connection_id=${encodeURIComponent(connectionId)}&repo_id=${encodeURIComponent(repoId)}` +
      (branch ? `&branch=${encodeURIComponent(branch)}` : ""),
  );

/** The repository's branches for the picker: the default first, TerraFlow review branches left out. */
export const getBranches = (connectionId: string, repoId: string) =>
  json<{ branches: string[]; default_branch: string; truncated: boolean }>(
    `/github/branches?connection_id=${encodeURIComponent(connectionId)}&repo_id=${encodeURIComponent(repoId)}`,
  );

/** Creates the first commit (a README.md) of a repository that has none; the user confirmed it. */
export const initRepo = (connectionId: string, repoId: string) =>
  json<{ commit: string }>("/github/init", { method: "POST", body: { connection_id: connectionId, repo_id: repoId } });

export interface FromRepoInput {
  name: string;
  description?: string;
  connection_id: string;
  repo_id: string;
  folder: string;
  /** Start in a folder that holds no Terraform yet (the first publish creates it). */
  new_folder?: boolean;
  /** The branch the project reads and starts review branches from ("" or absent = the default). */
  branch?: string;
}

/** A folder path inside a repository: up to four plain segments (mirrors the server rules). */
export const NEW_FOLDER_RE = /^[A-Za-z0-9][A-Za-z0-9._-]*(\/[A-Za-z0-9][A-Za-z0-9._-]*){0,3}$/;
export const createFromRepo = (input: FromRepoInput) =>
  json<Project>("/projects/from-repo", { method: "POST", body: input });

// GitHub sends the browser back to #/projects/new?connected=N (or ?error=code). The app
// clears the hash as soon as it starts, before the home screen exists, so the return is
// read here, when this module loads, and kept until the home screen has used it.
let pendingReturn: URLSearchParams | null = (() => {
  const m = (typeof window === "undefined" ? "" : window.location.hash).match(/^#\/projects\/new(?:\?(.*))?$/);
  return m ? new URLSearchParams(m[1] ?? "") : null;
})();
export const peekGitHubReturn = () => pendingReturn;
export const clearGitHubReturn = () => {
  pendingReturn = null;
};

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
  /** "needs_attention": GitHub says the owner can no longer reach the repository (D-061). */
  repository_access?: "ok" | "needs_attention";
  /** The repository's default branch, and the branch chosen for the project ("" or missing = the default; 8.8). */
  default_branch?: string;
  base_branch?: string;
  /** The chosen base branch is gone from the repository (8.8). */
  base_branch_missing?: boolean;
  /** Open changesets made before the base branch was last changed. */
  pending_on_old_base?: number;
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
