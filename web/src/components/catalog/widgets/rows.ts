/** Row helpers for the key/value and list editors: one empty row is always kept at the end. */

export interface KVRow {
  id: number;
  k: string;
  v: string;
}

let nextId = 1;
export const newId = () => nextId++;

export function toRows(value: Record<string, unknown> | undefined): KVRow[] {
  return Object.entries(value ?? {}).map(([k, v]) => ({ id: newId(), k, v: v == null ? "" : String(v) }));
}

/** Append an empty row unless the last row is already empty. */
export function withBlankRow(rows: KVRow[]): KVRow[] {
  const last = rows[rows.length - 1];
  if (last && !last.k && !last.v) return rows;
  return [...rows, { id: newId(), k: "", v: "" }];
}

/** The pairs that count: a row needs a key; the last row with a key wins. Insertion order is kept. */
export function rowsToObject(rows: KVRow[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const r of rows) {
    const k = r.k.trim();
    if (k) out[k] = r.v;
  }
  return out;
}

export interface ListRow {
  id: number;
  text: string;
}

export function withBlankItem(rows: ListRow[]): ListRow[] {
  const last = rows[rows.length - 1];
  if (last && !last.text) return rows;
  return [...rows, { id: newId(), text: "" }];
}

export const itemsToList = (rows: ListRow[]): string[] => rows.map((r) => r.text.trim()).filter(Boolean);
