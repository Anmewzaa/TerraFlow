import { useEffect, useState } from "react";
import type { CanvasState } from "../types/changeset";
import type { Finding } from "../lib/findings";

/**
 * What the server says about the canvas as it is now (D-065). The server is the one judge: the
 * panel's own quick checks only fill the gap while this answer is on its way. Asks again a moment
 * after the last edit, drops an answer that is no longer current, and says nothing when the call
 * fails (the rules still run when the work is published).
 */
export function useServerFindings(projectId: string | null, canvas: CanvasState, delayMs = 600): Finding[] {
  const [findings, setFindings] = useState<Finding[]>([]);

  useEffect(() => {
    if (!projectId || canvas.nodes.length === 0) {
      setFindings([]);
      return;
    }
    const ctrl = new AbortController();
    const timer = setTimeout(() => {
      fetch(`/api/projects/${projectId}/validate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nodes: canvas.nodes, edges: canvas.edges }),
        signal: ctrl.signal,
      })
        .then((r) => (r.ok ? r.json() : null))
        .then((body: { findings?: Finding[] } | null) => {
          if (body && Array.isArray(body.findings)) setFindings(body.findings);
        })
        .catch(() => {
          /* aborted or offline: keep what we had */
        });
    }, delayMs);
    return () => {
      clearTimeout(timer);
      ctrl.abort();
    };
  }, [projectId, canvas, delayMs]);

  return findings;
}

/** The problems of one node, as the lines the panel shows, without repeating what it already says. */
export function serverIssueTexts(findings: Finding[], nodeId: string, alreadyShown: string[]): string[] {
  const seen = new Set(alreadyShown.map((t) => t.toLowerCase()));
  const out: string[] = [];
  for (const f of findings) {
    if (f.node !== nodeId) continue;
    // Errors always; the project's own rules (policy) also when they are warnings.
    if (f.severity !== "error" && f.rule !== "policy") continue;
    const text = f.severity === "error" ? f.message : `Policy: ${f.message}`;
    if (seen.has(text.toLowerCase())) continue;
    seen.add(text.toLowerCase());
    out.push(text);
  }
  return out;
}
