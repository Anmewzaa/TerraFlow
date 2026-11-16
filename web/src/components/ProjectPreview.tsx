import { useEffect, useMemo, useState } from "react";
import type { GraphNode, ProjectGraph } from "../types/graph";
import { getAzureIcon } from "./nodes/domainStyles";
import { ResourceGlyph } from "./nodes/ResourceGlyph";

interface Item {
  type: string;
  icon: string;
  domain: string;
}
interface Summary {
  items: Item[];
  folders: number;
  /** "repository": committed on the branch; "canvas": only on a canvas so far (not published). */
  from: "repository" | "canvas";
  env: string;
}

/** One summary per project, kept for the visit, so going back to the list does not fetch them all again. */
const cache = new Map<string, Summary | "error">();

function tidy(type: string): string {
  return type.replace(/^azurerm_/, "").replace(/_/g, " ");
}

interface DraftNode {
  module_name: string;
  kind?: string;
  resource_type?: string;
  target_folder?: string;
}
interface DraftChangeset {
  env: string;
  status: string;
  updated_at: string;
  canvas?: { nodes?: DraftNode[] };
}

async function load(projectId: string, signal: AbortSignal): Promise<Summary> {
  const id = encodeURIComponent(projectId);
  const [g, c] = await Promise.all([
    fetch(`/api/projects/${id}/graph?env=dev`, { signal }).then((r) => (r.ok ? (r.json() as Promise<ProjectGraph>) : null)).catch(() => null),
    fetch(`/api/projects/${id}/changesets`, { signal }).then((r) => (r.ok ? (r.json() as Promise<DraftChangeset[] | null>) : null)).catch(() => null),
  ]);
  const repoNodes: GraphNode[] = (g?.folders ?? []).flatMap((f) => f.nodes ?? []);
  if (repoNodes.length > 0) {
    return {
      items: repoNodes.map((n) => ({ type: n.type, icon: n.label ?? n.type, domain: n.domain })),
      folders: (g?.folders ?? []).length,
      from: "repository",
      env: "dev",
    };
  }
  // Nothing committed yet: show what is on the canvases (the open draft of an environment, else its newest saved version).
  let best: { env: string; nodes: DraftNode[] } | null = null;
  const envs = [...new Set((c ?? []).map((x) => x.env))];
  for (const env of envs) {
    const mine = (c ?? []).filter((x) => x.env === env && x.status !== "rejected");
    const pick = mine.find((x) => x.status === "draft") ?? [...mine].sort((x, y) => y.updated_at.localeCompare(x.updated_at))[0];
    const nodes = pick?.canvas?.nodes ?? [];
    if (nodes.length > (best?.nodes.length ?? 0)) best = { env, nodes };
  }
  if (!best && !g && !c) throw new Error("unreadable");
  const nodes = best?.nodes ?? [];
  return {
    items: nodes.map((n) => ({ type: n.kind === "resource" ? (n.resource_type ?? n.module_name) : n.module_name, icon: n.module_name, domain: "neutral" })),
    folders: new Set(nodes.map((n) => n.target_folder ?? "")).size,
    from: "canvas",
    env: best?.env ?? "dev",
  };
}

/**
 * What a project holds, at a glance: the kinds of resource with a count each, shown with their icons. It reads the
 * repository, and when nothing is committed yet it shows what is on the canvases and says so. A project whose
 * repository cannot be read says so; it never shows an empty box as if it had nothing.
 */
export function ProjectPreview({ projectId }: { projectId: string }) {
  const [sum, setSum] = useState<Summary | "error" | null>(() => cache.get(projectId) ?? null);

  useEffect(() => {
    if (cache.has(projectId)) return;
    const ctl = new AbortController();
    load(projectId, ctl.signal)
      .then((v) => {
        cache.set(projectId, v);
        setSum(v);
      })
      .catch((e: unknown) => {
        if ((e as { name?: string }).name === "AbortError") return;
        cache.set(projectId, "error");
        setSum("error");
      });
    return () => ctl.abort();
  }, [projectId]);

  const summary = useMemo(() => {
    if (!sum || sum === "error") return null;
    const byType = new Map<string, { count: number; item: Item }>();
    for (const it of sum.items) {
      const hit = byType.get(it.type);
      if (hit) hit.count++;
      else byType.set(it.type, { count: 1, item: it });
    }
    return { ...sum, total: sum.items.length, kinds: [...byType.entries()].sort((a, b) => b[1].count - a[1].count) };
  }, [sum]);

  const box = "mb-4 min-h-[4.25rem] rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-glass-strong)] p-3";

  if (sum === null) {
    return <div className={`${box} animate-pulse`} aria-hidden />;
  }
  if (sum === "error" || !summary) {
    return <p className={`${box} m-0 flex items-center text-xs text-[var(--tf-color-text-muted)]`}>Preview unavailable: the repository could not be read.</p>;
  }
  if (summary.total === 0) {
    return <p className={`${box} m-0 flex items-center text-xs text-[var(--tf-color-text-muted)]`}>No resources yet. Open the canvas to add some.</p>;
  }
  const shown = summary.kinds.slice(0, 5);
  const more = summary.kinds.length - shown.length;
  return (
    <div className={box}>
      <ul className="m-0 flex list-none flex-wrap gap-2 p-0" aria-label="Resources in this project">
        {shown.map(([type, { count, item }]) => {
          const icon = getAzureIcon(item.icon);
          return (
            <li key={type} title={`${count} × ${tidy(type)}`} className="relative grid size-10 place-items-center rounded-lg border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)]">
              {icon ? <img src={icon} alt="" className="size-6" /> : <ResourceGlyph domain={item.domain} className="size-5 text-[var(--tf-color-text-secondary)]" />}
              {count > 1 && (
                <span className="absolute -right-1.5 -top-1.5 grid min-w-4 place-items-center rounded-full bg-[var(--tf-color-action)] px-1 text-[10px] font-semibold leading-4 text-[var(--tf-color-on-action)]">{count}</span>
              )}
            </li>
          );
        })}
        {more > 0 && <li className="grid h-10 place-items-center px-1 text-xs text-[var(--tf-color-text-muted)]">+{more} more</li>}
      </ul>
      <p className="m-0 mt-2 text-xs text-[var(--tf-color-text-muted)]">
        {summary.total} resource{summary.total === 1 ? "" : "s"} · {summary.folders} folder{summary.folders === 1 ? "" : "s"}
        {summary.from === "canvas" && ` · on the ${summary.env} canvas, not published yet`}
      </p>
    </div>
  );
}
