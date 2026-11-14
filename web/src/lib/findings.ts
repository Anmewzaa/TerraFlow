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
