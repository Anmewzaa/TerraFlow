import { useMemo, useState } from "react";
import { Check, GitBranch, Lock, Search } from "lucide-react";
import type { RepoChoice } from "../hooks/useGitHub";

export const repoKeyOf = (r: RepoChoice) => `${r.connection_id}/${r.repo_id}`;

/** Minutes until GitHub's confirmation of the user's access runs out (never below 0). */
export function minutesLeft(repos: RepoChoice[], now = Date.now()): number | null {
  const times = repos.map((r) => Date.parse(r.expires_at)).filter((t) => !Number.isNaN(t));
  if (times.length === 0) return null;
  return Math.max(0, Math.ceil((Math.min(...times) - now) / 60000));
}

interface Props {
  repos: RepoChoice[];
  value: string;
  onChange: (key: string) => void;
}

/**
 * The repositories the user can use, shown as a list to read and click (not a drop-down):
 * a search box on top, the account and default branch on every row, private ones marked.
 */
export function RepoPicker({ repos, value, onChange }: Props) {
  const [q, setQ] = useState("");
  // Once a repository is chosen the list folds into one line, so the next steps stay in view.
  const [open, setOpen] = useState(true);
  const chosen = repos.find((r) => repoKeyOf(r) === value);
  const shown = useMemo(() => {
    const needle = q.trim().toLowerCase();
    const list = needle ? repos.filter((r) => r.full_name.toLowerCase().includes(needle)) : repos;
    return [...list].sort((a, b) => a.full_name.localeCompare(b.full_name));
  }, [repos, q]);

  if (chosen && !open) {
    return (
      <div className="flex items-center gap-3 rounded-lg border border-[var(--tf-color-action)] bg-[var(--tf-glass-strong)] px-3 py-2 text-sm">
        <span className="min-w-0 flex-1">
          <span className="block truncate font-medium">{chosen.full_name}</span>
          <span className="flex items-center gap-1 text-xs text-[var(--tf-color-text-muted)]">
            <GitBranch className="size-3" /> {chosen.default_branch}
            {chosen.private && (
              <>
                {" "}
                · <Lock className="size-3" /> private
              </>
            )}
          </span>
        </span>
        <button type="button" className="text-xs underline" onClick={() => setOpen(true)}>
          Change
        </button>
      </div>
    );
  }

  return (
    <div>
      <div className="relative mb-2">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
        <input
          type="search"
          aria-label="Search repositories"
          placeholder={`Search ${repos.length} ${repos.length === 1 ? "repository" : "repositories"}…`}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] py-2 pl-9 pr-3 text-sm outline-none focus:border-[var(--tf-color-action)]"
        />
      </div>
      <ul
        role="listbox"
        aria-label="Repositories"
        className="m-0 max-h-60 list-none overflow-y-auto rounded-lg border border-[var(--tf-color-border)] p-1"
      >
        {shown.length === 0 && <li className="px-3 py-4 text-center text-sm text-[var(--tf-color-text-muted)]">No repository matches “{q}”.</li>}
        {shown.map((r) => {
          const key = repoKeyOf(r);
          const on = key === value;
          return (
            <li key={key} role="option" aria-selected={on}>
              <button
                type="button"
                onClick={() => {
                  onChange(on ? "" : key);
                  setOpen(on);
                }}
                className={`flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition ${
                  on ? "bg-[var(--tf-color-action)] text-[var(--tf-color-on-action)]" : "hover:bg-[var(--tf-glass-hover)]"
                }`}
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{r.full_name}</span>
                  <span className={`flex items-center gap-1 text-xs ${on ? "opacity-90" : "text-[var(--tf-color-text-muted)]"}`}>
                    <GitBranch className="size-3" /> {r.default_branch}
                  </span>
                </span>
                {r.private && (
                  <span className="flex shrink-0 items-center gap-1 rounded-full bg-[var(--tf-glass-strong)] px-2 py-0.5 text-xs text-[var(--tf-color-text)]">
                    <Lock className="size-3" /> private
                  </span>
                )}
                {on && <Check className="size-4 shrink-0" aria-hidden />}
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
