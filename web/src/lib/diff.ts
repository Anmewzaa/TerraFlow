/** Line diff for generated HCL: added / modified / removed lines, VS Code style. */

export type LineKind = "same" | "add" | "mod" | "del";

export interface DiffResult {
  /** Text to display: the new file with removed lines spliced in. */
  text: string;
  /** One kind per displayed line. */
  kinds: LineKind[];
  /** For "mod" lines: the previous text (for tooltips). */
  previous: (string | undefined)[];
  added: number;
  modified: number;
  removed: number;
}

function lines(s: string): string[] {
  if (s === "") return [];
  const out = s.replace(/\r\n/g, "\n").split("\n");
  if (out[out.length - 1] === "") out.pop();
  return out;
}

/** Classic LCS table; generated files are small so O(n*m) is fine. */
function lcs(a: string[], b: string[]): number[][] {
  const t = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      t[i][j] = a[i] === b[j] ? t[i + 1][j + 1] + 1 : Math.max(t[i + 1][j], t[i][j + 1]);
    }
  }
  return t;
}

/**
 * Diff `base` (previous version) against `current`. A run of removed lines
 * directly followed by added lines is a modification: pairs become "mod"
 * (shown with the new text), leftovers stay "del" / "add".
 */
export function diffLines(base: string, current: string): DiffResult {
  const a = lines(base);
  const b = lines(current);
  const t = lcs(a, b);

  type Op = { kind: "same" | "add" | "del"; text: string };
  const ops: Op[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      ops.push({ kind: "same", text: b[j] });
      i++;
      j++;
    } else if (t[i + 1][j] >= t[i][j + 1]) {
      ops.push({ kind: "del", text: a[i++] });
    } else {
      ops.push({ kind: "add", text: b[j++] });
    }
  }
  while (i < a.length) ops.push({ kind: "del", text: a[i++] });
  while (j < b.length) ops.push({ kind: "add", text: b[j++] });

  const out: string[] = [];
  const kinds: LineKind[] = [];
  const previous: (string | undefined)[] = [];
  let added = 0;
  let modified = 0;
  let removed = 0;

  for (let k = 0; k < ops.length; ) {
    if (ops[k].kind === "same") {
      out.push(ops[k].text);
      kinds.push("same");
      previous.push(undefined);
      k++;
      continue;
    }
    const dels: string[] = [];
    const adds: string[] = [];
    while (k < ops.length && ops[k].kind === "del") dels.push(ops[k++].text);
    while (k < ops.length && ops[k].kind === "add") adds.push(ops[k++].text);
    const pairs = Math.min(dels.length, adds.length);
    for (let p = 0; p < pairs; p++) {
      out.push(adds[p]);
      kinds.push("mod");
      previous.push(dels[p]);
      modified++;
    }
    for (let p = pairs; p < dels.length; p++) {
      out.push(dels[p]);
      kinds.push("del");
      previous.push(undefined);
      removed++;
    }
    for (let p = pairs; p < adds.length; p++) {
      out.push(adds[p]);
      kinds.push("add");
      previous.push(undefined);
      added++;
    }
  }
  return { text: out.join("\n") + (out.length ? "\n" : ""), kinds, previous, added, modified, removed };
}

export type FileStatus = "added" | "modified" | "removed" | "unchanged";

export interface FileDiff {
  path: string;
  status: FileStatus;
  diff: DiffResult;
}

export interface GeneratedLike {
  folder: string;
  filename: string;
  content: string;
}

/** Diff two sets of generated files, matching by folder/filename. */
export function diffFiles(base: GeneratedLike[], current: GeneratedLike[]): FileDiff[] {
  const key = (f: GeneratedLike) => `${f.folder}/${f.filename}`;
  const baseMap = new Map(base.map((f) => [key(f), f]));
  const curMap = new Map(current.map((f) => [key(f), f]));
  const paths = [...new Set([...current.map(key), ...base.map(key)])].sort();

  return paths.map((path) => {
    const b = baseMap.get(path);
    const c = curMap.get(path);
    const diff = diffLines(b?.content ?? "", c?.content ?? "");
    let status: FileStatus;
    if (!b) status = "added";
    else if (!c) status = "removed";
    else status = diff.added + diff.modified + diff.removed > 0 ? "modified" : "unchanged";
    return { path, status, diff };
  });
}

/**
 * Files to show in the preview: everything already in the repo (unchanged) plus
 * what the draft adds, changes or removes.
 *
 * Baseline for a generated file = the changeset's base version when it is a
 * revision, otherwise the file already in the repo at the same path.
 */
export function diffWithRepo(repo: GeneratedLike[], base: GeneratedLike[], current: GeneratedLike[]): FileDiff[] {
  const key = (f: GeneratedLike) => `${f.folder}/${f.filename}`;
  const repoMap = new Map(repo.map((f) => [key(f), f]));
  const baseMap = new Map(base.map((f) => [key(f), f]));
  const curMap = new Map(current.map((f) => [key(f), f]));
  const paths = [...new Set([...repo.map(key), ...base.map(key), ...current.map(key)])].sort();

  return paths.map((path) => {
    const cur = curMap.get(path);
    const baseline = baseMap.get(path) ?? repoMap.get(path);
    if (cur) {
      const diff = diffLines(baseline?.content ?? "", cur.content);
      const changed = diff.added + diff.modified + diff.removed > 0;
      const status: FileStatus = !baseline ? "added" : changed ? "modified" : "unchanged";
      return { path, status, diff };
    }
    const b = baseMap.get(path);
    if (b) return { path, status: "removed" as FileStatus, diff: diffLines(b.content, "") };
    const r = repoMap.get(path)!;
    return { path, status: "unchanged" as FileStatus, diff: diffLines(r.content, r.content) };
  });
}
