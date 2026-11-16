import { useState } from "react";
import { ArrowLeft, ArrowRight, GitBranch, LayoutGrid, ShieldCheck, Workflow } from "lucide-react";
import { cn } from "@/lib/utils";
import { RELEASES, type ChangeKind } from "../lib/releases";

const dateFormat = new Intl.DateTimeFormat("en-US", { year: "numeric", month: "long", day: "numeric" });

/** A tint and the word, never the tint alone, so the list survives greyscale and colour blindness. */
const kindStyle: Record<ChangeKind, string> = {
  Added: "bg-[var(--tf-color-success-surface)] text-[var(--tf-color-success)]",
  Improved: "bg-[var(--tf-color-info-surface)] text-[var(--tf-color-info)]",
  Fixed: "bg-[var(--tf-color-warning-surface)] text-[var(--tf-color-warning)]",
  Removed: "bg-[var(--tf-color-error-surface)] text-[var(--tf-color-error)]",
};

const PILLARS = [
  { icon: LayoutGrid, title: "Compose", text: "Drag modules from the catalog onto a canvas. Every form comes from the module's schema." },
  { icon: ShieldCheck, title: "Review", text: "Checks run on the server, and an owner or co-owner approves before anything is written." },
  { icon: GitBranch, title: "Branch", text: "TerraFlow writes module calls on a new branch of your repository, then stops. The pull request, plan and apply are yours." },
];

/**
 * What TerraFlow is, and what each version added, improved, fixed or removed. The list is `lib/releases.ts`,
 * the same place the version on the Home page comes from.
 */
const PER_PAGE = 3;

export function ReleaseNotes({ initial, showAbout = true }: { initial?: string; showAbout?: boolean }) {
  const pages = Math.ceil(RELEASES.length / PER_PAGE);
  const [page, setPage] = useState(() => {
    const at = RELEASES.findIndex((r) => r.version === initial);
    return at < 0 ? 0 : Math.floor(at / PER_PAGE);
  });
  const shown = RELEASES.slice(page * PER_PAGE, page * PER_PAGE + PER_PAGE);
  return (
    <div className="space-y-12">
      {showAbout && (
      <section aria-labelledby="about-heading" className="rounded-3xl border border-[var(--tf-color-border)] bg-[color-mix(in_srgb,var(--tf-color-surface)_88%,transparent)] p-8 shadow-sm backdrop-blur-sm sm:p-10">
        <p className="m-0 flex items-center gap-2 text-sm font-medium text-[var(--tf-color-text-muted)]">
          <Workflow className="size-4" aria-hidden /> About TerraFlow
        </p>
        <h1 id="about-heading" className="m-0 mt-3 max-w-2xl text-3xl font-bold tracking-tight sm:text-4xl">
          Compose Azure infrastructure from approved modules, and ship a Terraform branch.
        </h1>
        <p className="mb-0 mt-3 max-w-2xl text-pretty text-[var(--tf-color-text-secondary)]">
          TerraFlow is a visual portal for Terraform. It reads your repository, never Azure, and holds no Azure credential. Its work ends at the branch.
        </p>
        <ul className="m-0 mt-8 grid list-none gap-4 p-0 sm:grid-cols-3">
          {PILLARS.map((p) => (
            <li key={p.title} className="rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-4">
              <span className="grid size-9 place-items-center rounded-lg bg-[var(--tf-color-info-surface)] text-[var(--tf-color-info)]">
                <p.icon className="size-5" aria-hidden />
              </span>
              <h2 className="m-0 mt-3 text-base font-semibold">{p.title}</h2>
              <p className="mb-0 mt-1 text-sm text-[var(--tf-color-text-secondary)]">{p.text}</p>
            </li>
          ))}
        </ul>
      </section>
      )}

      <section aria-labelledby="changelog-heading">
        <p className="m-0 text-sm font-medium text-[var(--tf-color-text-muted)]">Changelog</p>
        <h2 id="changelog-heading" className="m-0 mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">
          What changed in each version
        </h2>
        <p className="mb-0 mt-2 max-w-xl text-pretty text-[var(--tf-color-text-secondary)]">
          The newest version is first. Each line says whether it was added, improved, fixed or removed.
        </p>

        <ol className="m-0 mt-10 flex list-none flex-col p-0">
          {shown.map((release, index) => (
            <li
              key={release.version}
              id={release.version}
              className={cn("scroll-mt-6 sm:grid sm:grid-cols-[8rem_1fr] sm:gap-x-8", index > 0 && "mt-10 border-t border-[var(--tf-color-border)] pt-10")}
            >
              <div className="mb-3 flex items-baseline gap-3 sm:mb-0 sm:flex-col sm:gap-1.5 sm:pt-1">
                <time dateTime={release.date} className="text-sm tabular-nums text-[var(--tf-color-text-muted)]">
                  {dateFormat.format(new Date(`${release.date}T00:00:00`))}
                </time>
                <span
                  className={cn(
                    "rounded-full border px-2.5 py-0.5 font-mono text-xs",
                    release.version === initial || (page === 0 && index === 0) ? "border-[var(--tf-color-action)] text-[var(--tf-color-action)]" : "border-[var(--tf-color-border-strong)]",
                  )}
                >
                  {release.version}
                  {page === 0 && index === 0 && <span className="ml-1.5 font-sans">current</span>}
                </span>
              </div>

              <div className="min-w-0">
                <h3 className="m-0 text-lg font-semibold tracking-tight">{release.title}</h3>
                <dl className="m-0 mt-4 flex flex-col gap-2.5">
                  {release.changes.map((c) => (
                    <div key={c.text} className="flex items-baseline gap-3">
                      <dt className="w-[4.5rem] shrink-0">
                        <span className={cn("inline-block rounded-full px-2.5 py-0.5 text-xs font-medium", kindStyle[c.kind])}>{c.kind}</span>
                      </dt>
                      <dd className="m-0 text-pretty text-sm leading-6 text-[var(--tf-color-text-secondary)]">{c.text}</dd>
                    </div>
                  ))}
                </dl>
              </div>
            </li>
          ))}
        </ol>

        {pages > 1 && (
          <nav aria-label="Release notes pages" className="mt-10 flex items-center justify-between gap-3 border-t border-[var(--tf-color-border)] pt-6">
            <button type="button" onClick={() => setPage(page - 1)} disabled={page === 0} className="flex items-center gap-2 rounded-full border border-[var(--tf-color-border-strong)] px-4 py-2 text-sm font-medium transition hover:border-[var(--tf-color-action)] disabled:cursor-not-allowed disabled:opacity-40">
              <ArrowLeft className="size-4" aria-hidden /> Newer
            </button>
            <span className="text-sm text-[var(--tf-color-text-muted)]">Page {page + 1} of {pages}</span>
            <button type="button" onClick={() => setPage(page + 1)} disabled={page >= pages - 1} className="flex items-center gap-2 rounded-full border border-[var(--tf-color-border-strong)] px-4 py-2 text-sm font-medium transition hover:border-[var(--tf-color-action)] disabled:cursor-not-allowed disabled:opacity-40">
              Older <ArrowRight className="size-4" aria-hidden />
            </button>
          </nav>
        )}
      </section>
    </div>
  );
}
