import { useCallback, useEffect, useRef, useState } from "react";
import type { Changeset, CanvasState, CanvasNode } from "../types/changeset";
import { descendantIds } from "../lib/canvasLayout";

/** reloadKey: bump to re-fetch (e.g. after opening a revision). */
export function useChangeset(projectId: string | null, env: string, reloadKey = 0) {
  const [changeset, setChangeset] = useState<Changeset | null>(null);
  const [canvas, setCanvas] = useState<CanvasState>({ nodes: [], edges: [] });
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load or create a draft changeset for this project+env.
  useEffect(() => {
    if (!projectId) return;
    setLoading(true);

    fetch(`/api/projects/${projectId}/changesets`)
      .then((r) => r.ok ? r.json() : [])
      .then((list: Changeset[]) => {
        const draft = list.find((c) => c.status === "draft" && c.env === env);
        if (draft) {
          setChangeset(draft);
          setCanvas(draft.canvas ?? { nodes: [], edges: [] });
        } else {
          setChangeset(null);
          setCanvas({ nodes: [], edges: [] });
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [projectId, env, reloadKey]);

  const ensureChangeset = useCallback(async (): Promise<string | null> => {
    if (changeset) return changeset.id;
    if (!projectId) return null;

    const res = await fetch(`/api/projects/${projectId}/changesets`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ env }),
    });
    if (!res.ok) return null;
    const { id } = await res.json();

    setChangeset((prev) => prev ?? ({
      id,
      project_id: projectId,
      env,
      user_oid: "",
      title: "",
      status: "draft",
      canvas: { nodes: [], edges: [] },
      reject_reason: null,
      reviewer_oid: null,
      reviewed_at: null,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    } as Changeset));

    return id;
  }, [changeset, projectId, env]);

  const saveCanvas = useCallback(
    (newCanvas: CanvasState) => {
      setCanvas(newCanvas);
      setSaving(true);

      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(async () => {
        const csId = changeset?.id ?? (await ensureChangeset());
        if (!csId) { setSaving(false); return; }

        await fetch(`/api/projects/${projectId}/changesets/${csId}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(newCanvas),
        }).catch(() => {});
        setSaving(false);
      }, 800);
    },
    [changeset, projectId, ensureChangeset]
  );

  const addNode = useCallback(
    (node: CanvasNode) => {
      const next = { ...canvas, nodes: [...canvas.nodes, node] };
      saveCanvas(next);
    },
    [canvas, saveCanvas]
  );

  const updateNode = useCallback(
    (nodeId: string, updates: Partial<CanvasNode>) => {
      const next = {
        ...canvas,
        nodes: canvas.nodes.map((n) =>
          n.id === nodeId ? { ...n, ...updates } : n
        ),
      };
      saveCanvas(next);
    },
    [canvas, saveCanvas]
  );

  const removeNode = useCallback(
    (nodeId: string) => {
      // Removing a container removes everything placed inside it.
      const gone = descendantIds(canvas.nodes, nodeId);
      const next = {
        ...canvas,
        nodes: canvas.nodes.filter((n) => !gone.has(n.id)),
        edges: canvas.edges.filter((e) => !gone.has(e.from) && !gone.has(e.to)),
      };
      saveCanvas(next);
    },
    [canvas, saveCanvas]
  );

  const publish = useCallback(
    async (title: string) => {
      const csId = changeset?.id ?? (await ensureChangeset());
      if (!csId || !projectId) return false;

      const res = await fetch(
        `/api/projects/${projectId}/changesets/${csId}/publish`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ title }),
        }
      );
      if (res.ok) {
        setChangeset((prev) => prev ? { ...prev, status: "published" } : prev);
      }
      return res.ok;
    },
    [changeset, projectId, ensureChangeset]
  );

  return {
    changeset,
    canvas,
    loading,
    saving,
    addNode,
    updateNode,
    removeNode,
    saveCanvas,
    publish,
  };
}
