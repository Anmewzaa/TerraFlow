/* Findings from the server's canvas validation (D-065). The server is the judge; this file only
 * turns its answer into text the screens can show. */

export interface Finding {
  severity: "error" | "warning" | "info";
  node?: string;
  field?: string;
  rule: string;
  message: string;
  structural?: boolean;
}

/** One line for a refused request: the reason, then the first few problems. */
export function describeFindings(error: string | undefined, findings: Finding[] | undefined, max = 4): string {
  const errors = (findings ?? []).filter((f) => f.severity === "error");
  if (errors.length === 0) return error ?? "";
  const shown = errors.slice(0, max).map((f) => `${f.field ? f.field.replace(/_/g, " ") + ": " : ""}${f.message}`);
  const more = errors.length > max ? ` (and ${errors.length - max} more)` : "";
  return `${error ?? "Fix these problems first"}: ${shown.join("; ")}${more}`;
}

/** The server's problems for one node, apart: what must be fixed, and the project's policy warnings. */
export function serverProblems(findings: Finding[], nodeId: string, alreadyShown: string[] = []): { errors: string[]; warnings: string[] } {
  const seen = new Set(alreadyShown.map((t) => t.toLowerCase()));
  const errors: string[] = [];
  const warnings: string[] = [];
  for (const f of findings) {
    if (f.node !== nodeId) continue;
    const text = f.message;
    if (seen.has(text.toLowerCase())) continue;
    if (f.severity === "error") errors.push(text);
    else if (f.rule === "policy") warnings.push(text);
    else continue;
    seen.add(text.toLowerCase());
  }
  return { errors, warnings };
}
