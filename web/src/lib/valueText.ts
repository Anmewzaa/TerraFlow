/**
 * Text for a node value that is shown on screen. A value is a string or number
 * as typed, or {"$expr": "<text>"} when it came from a reference (var.x,
 * module.a.id) that cannot be shown as a plain value.
 * Never returns an object: React cannot render one.
 */
export function valueText(v: unknown): string {
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  if (v && typeof v === "object" && typeof (v as { $expr?: unknown }).$expr === "string") {
    return (v as { $expr: string }).$expr;
  }
  return "";
}

/** True for {"$expr": "<text>"}: a reference kept as written in the repository. */
export function isExpr(v: unknown): v is { $expr: string } {
  return !!v && typeof v === "object" && !Array.isArray(v) && typeof (v as { $expr?: unknown }).$expr === "string";
}

/** The name a node shows: its name value when there is one, else its instance name. */
export function displayName(values: Record<string, unknown> | undefined, instanceName: string): string {
  return valueText(values?.name) || instanceName;
}

/** Parents must come before their children in the React Flow node list. */
export function parentsFirst<T extends { id: string; parentId?: string }>(nodes: T[]): T[] {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const depth = (n: T): number => {
    let d = 0;
    for (let cur: T | undefined = n; cur?.parentId && d <= nodes.length; d++) cur = byId.get(cur.parentId);
    return d;
  };
  // Array.sort is stable, so siblings keep their order.
  return [...nodes].sort((a, b) => depth(a) - depth(b));
}
