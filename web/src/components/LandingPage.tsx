import { useEffect, useState } from "react";
import { ArrowLeft, ArrowUpRight, GitBranch, Github, LayoutGrid, Loader2, LogIn, ShieldCheck, User as UserIcon, } from "lucide-react";
import { cn } from "@/lib/utils";
import { devSignIn } from "../hooks/useAuth";
import type { DevUser, LoginProvider, User } from "../types/auth";
import { APP_VERSION } from "../lib/releases";
import { Logo } from "./Logo";
import { CanvasGlyphs } from "./CanvasGlyphs";
import { ReleaseNotes } from "./ReleaseNotes";
import { SIGNIN_ERRORS } from "./SignInPage";

interface Props {
  /** Sign-in buttons of a real server. */
  providers?: LoginProvider[] | null;
  /** The local sign-in: made-up people to pick. */
  devUsers?: DevUser[] | null;
  error?: string | null;
  /** Already signed in: the buttons open the app instead of the sign-in dialog. */
  user?: User | null;
  onEnter?: () => void;
}

const STEPS = [
  { icon: LayoutGrid, title: "Compose", text: "Drag approved modules onto a canvas. Every form is drawn from the module's schema, so nothing is typed from memory." },
  { icon: ShieldCheck, title: "Review", text: "The server checks the change, and an owner or co-owner approves before anything is written." },
  { icon: GitBranch, title: "Branch", text: "TerraFlow writes module calls on a new branch of your repository, then stops. The pull request, plan and apply stay with your team." },
];

const GUARANTEES = ["Never talks to Azure", "Holds no Azure credential", "Stops at the branch", "Reads your repository"];

const sans = "font-[family-name:var(--tf-font-sans)]";
const serif = "font-[Georgia,'Times_New_Roman',serif]";

/** The page before sign-in: what TerraFlow is, and the way in. Replaces the bare sign-in screens. */
export function LandingPage({ providers, devUsers, error, user, onEnter }: Props) {
  const [open, setOpen] = useState(!!error);
  useEffect(() => {
    if (error) setOpen(true);
  }, [error]);

  const start = () => {
    if (user && onEnter) return onEnter();
    try {
      sessionStorage.setItem("tf-enter", "1");
    } catch {
      /* the landing page shows once more after sign-in */
    }
    setOpen(true);
  };
  const mainLabel = user ? "Open workspace" : "Get Started";

  return (
    <div id="top" className={cn("h-screen bg-[var(--tf-color-canvas)] text-[var(--tf-color-text)]", sans)}>
      <CanvasGlyphs enabled soft>
        <div className="relative z-10 h-full overflow-y-auto overflow-x-hidden">

          <main>
        <section className="mx-auto max-w-5xl px-5 pb-20 pt-20 text-center sm:px-8 sm:pt-28">
          <span className="mx-auto mb-8 block w-fit text-[var(--tf-color-action-active)]">
            <Logo size={56} />
          </span>
          <br />
          <a href="#releases" className="inline-flex items-center gap-2 rounded-full border no-underline border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] px-3 py-1 font-mono text-xs text-[var(--tf-color-text-secondary)]">
            <span className="size-1.5 rounded-full bg-[var(--tf-color-success)]" aria-hidden /> {APP_VERSION} · what is new
          </a>
          <h1 className="m-0 mt-6 text-balance text-4xl font-bold leading-[1.08] tracking-tight sm:text-6xl lg:text-7xl">
            Building Azure infrastructure
            <br />
            from <em className={cn("bg-gradient-to-r from-[var(--tf-color-action)] to-[var(--tf-color-info)] bg-clip-text font-normal italic text-transparent", serif)}>approved modules</em>
          </h1>
          <p className="mx-auto mb-0 mt-6 max-w-2xl text-pretty text-lg text-[var(--tf-color-text-secondary)]">
            TerraFlow is a visual portal for Terraform. Compose resources on a canvas, have an owner review the change, and ship a branch your
            team already knows how to plan and apply.
          </p>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-5">
            <button
              type="button"
              onClick={start}
              className="group flex min-w-[15rem] items-center justify-between gap-4 rounded-full bg-[var(--tf-color-text)] py-2 pl-7 pr-2 text-base font-semibold text-[var(--tf-color-canvas)] transition hover:opacity-90"
            >
              {mainLabel}
              <span className="grid size-11 place-items-center rounded-full bg-[var(--tf-color-canvas)] text-[var(--tf-color-text)] transition group-hover:rotate-45">
                <ArrowUpRight className="size-5" aria-hidden />
              </span>
            </button>
            <a href="#how" className="text-sm font-medium text-[var(--tf-color-text-secondary)] no-underline hover:text-[var(--tf-color-text)]">
              See how it works
            </a>
          </div>
        </section>

        <section id="rules" aria-label="Guarantees" className="mx-auto max-w-5xl px-5 sm:px-8">
          <div className="flex items-center gap-6">
            <span className="h-px flex-1 bg-[var(--tf-color-border)]" aria-hidden />
            <p className="m-0 text-center text-sm text-[var(--tf-color-text-muted)]">What TerraFlow promises</p>
            <span className="h-px flex-1 bg-[var(--tf-color-border)]" aria-hidden />
          </div>
          <ul className="m-0 mt-8 grid list-none grid-cols-2 gap-3 p-0 md:grid-cols-4">
            {GUARANTEES.map((g) => (
              <li
                key={g}
                className="rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-glass)] px-4 py-5 text-center text-sm font-semibold backdrop-blur"
              >
                {g}
              </li>
            ))}
          </ul>
        </section>

        <section id="how" aria-labelledby="how-heading" className="mx-auto max-w-6xl scroll-mt-8 px-5 py-24 sm:px-8">
          <h2 id="how-heading" className="m-0 text-center text-3xl font-bold tracking-tight sm:text-4xl">
            Three steps, and the last one is yours
          </h2>
          <ol className="m-0 mt-12 grid list-none gap-5 p-0 md:grid-cols-3">
            {STEPS.map((s, i) => (
              <li key={s.title} className="rounded-3xl border border-[var(--tf-color-border)] bg-[var(--tf-glass)] p-7 shadow-sm backdrop-blur transition hover:-translate-y-1 hover:shadow-[var(--tf-shadow-lift)]">
                <span className="grid size-11 place-items-center rounded-xl bg-[var(--tf-color-info-surface)] text-[var(--tf-color-info)]">
                  <s.icon className="size-5" aria-hidden />
                </span>
                <p className="m-0 mt-5 font-mono text-xs text-[var(--tf-color-text-muted)]">Step {i + 1}</p>
                <h3 className="m-0 mt-1 text-xl font-semibold">{s.title}</h3>
                <p className="mb-0 mt-2 text-sm leading-6 text-[var(--tf-color-text-secondary)]">{s.text}</p>
              </li>
            ))}
          </ol>
        </section>

            <section id="see" aria-labelledby="see-heading" className="mx-auto max-w-6xl scroll-mt-8 px-5 pb-24 sm:px-8">
              <h2 id="see-heading" className="m-0 text-center text-3xl font-bold tracking-tight sm:text-4xl">See it in use</h2>
              <p className="mx-auto mb-0 mt-3 max-w-2xl text-center text-[var(--tf-color-text-secondary)]">
                These are screens of the running app, not mock-ups.
              </p>
              <Showcase />
            </section>

            <section id="releases" aria-label="Release notes" className="mx-auto max-w-3xl scroll-mt-8 px-5 pb-28 sm:px-8">
              <ReleaseNotes showAbout={false} />
            </section>
          </main>

          <footer className="border-t border-[var(--tf-color-border)] bg-[color-mix(in_srgb,var(--tf-color-canvas)_80%,transparent)] px-5 py-8 text-center text-xs text-[var(--tf-color-text-muted)] backdrop-blur">
            TerraFlow {APP_VERSION}. The work ends at the branch.
          </footer>
        </div>
      </CanvasGlyphs>

      {open && <SignInDialog providers={providers ?? []} devUsers={devUsers ?? null} error={error ?? null} onClose={() => setOpen(false)} />}
    </div>
  );
}

function SignInDialog({ providers, devUsers, error, onClose }: { providers: LoginProvider[]; devUsers: DevUser[] | null; error: string | null; onClose: () => void }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const pick = async (u: DevUser) => {
    setBusy(u.oid);
    setFailure(null);
    try {
      await devSignIn(u.oid);
    } catch (e) {
      setFailure(e instanceof Error ? e.message : "Could not sign in");
      setBusy(null);
    }
  };

  const message = failure ?? (error ? (SIGNIN_ERRORS[error] ?? "Sign-in failed.") : null);

  return (
    <div className={cn("fixed inset-0 z-50 bg-[var(--tf-color-canvas)] text-[var(--tf-color-text)]", sans)} role="dialog" aria-modal="true" aria-labelledby="signin-title">
      <CanvasGlyphs enabled soft>
        <div className="relative z-10 flex h-full flex-col items-center justify-center overflow-y-auto px-5 py-10">
          <button type="button" onClick={onClose} className="absolute left-5 top-5 flex items-center gap-1.5 text-sm text-[var(--tf-color-text-secondary)] hover:text-[var(--tf-color-text)]">
            <ArrowLeft className="size-4" aria-hidden /> Back
          </button>
          <div className="w-full max-w-md rounded-3xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-8 shadow-xl">
            <span className="mx-auto block w-fit text-[var(--tf-color-action-active)]"><Logo size={44} /></span>
            <h2 id="signin-title" className="m-0 mt-4 text-center text-3xl font-bold tracking-tight">Sign in to TerraFlow</h2>
            <p className="mx-auto mb-0 mt-2 max-w-sm text-center text-sm text-[var(--tf-color-text-secondary)]">
              {devUsers ? "Authentication is off here. Pick who you want to be; an admin and a developer see different things." : "After your first sign-in an administrator approves your account before you can open anything."}
            </p>
        {message && (
          <p role="alert" className="mb-0 mt-4 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
            {message}
          </p>
        )}

        {devUsers ? (
          <ul className="m-0 mt-5 grid list-none grid-cols-[minmax(0,1fr)] gap-3 p-0">
            {devUsers.map((u) => {
              const admin = u.role === "devops";
              return (
                <li key={u.oid} className="min-w-0">
                  <button
                    onClick={() => pick(u)}
                    disabled={busy !== null}
                    className="box-border flex w-full min-w-0 items-center gap-4 rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface-subtle)] p-4 text-left transition hover:border-[var(--tf-color-action)] disabled:opacity-60"
                  >
                    <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-[var(--tf-glass-strong)] text-[var(--tf-color-action)]">
                      {busy === u.oid ? <Loader2 className="size-5 animate-spin" /> : admin ? <ShieldCheck className="size-5" /> : <UserIcon className="size-5" />}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block font-semibold">{u.name}</span>
                      <span className="block text-xs text-[var(--tf-color-text-muted)]">{u.note || u.email}</span>
                    </span>
                    <span className="tf-badge shrink-0" data-tone={admin ? "good" : "info"}>{admin ? "admin" : "developer"}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        ) : providers.length > 0 ? (
          <ul className="m-0 mt-5 grid list-none grid-cols-[minmax(0,1fr)] gap-3 p-0">
            {providers.map((p) => (
              <li key={p.id}>
                <a href={p.href} className="tf-btn flex w-full items-center justify-center gap-2 py-3">
                  {p.id === "github" ? <Github className="size-5" /> : <LogIn className="size-5" />}
                  Continue with {p.label}
                </a>
              </li>
            ))}
          </ul>
        ) : (
          <p className="mb-0 mt-5 text-center text-sm text-[var(--tf-color-text-muted)]">No sign-in method is set up on this server.</p>
        )}
          </div>
        </div>
      </CanvasGlyphs>
    </div>
  );
}

const SHOTS = [
  { id: "canvas", tab: "Canvas", title: "Compose on a canvas", text: "Drag modules from the palette. Resources nest the way they do in Azure, and each one has a form drawn from its module's schema." },
  { id: "hcl", tab: "HCL Preview", title: "See the Terraform before it is written", text: "The preview shows the module calls TerraFlow would write, file by file, with only what changed highlighted." },
  { id: "changesets", tab: "Changesets", title: "Review before anything is written", text: "Save a version, send it for review, and an owner or co-owner approves. The progress line shows where each version stands." },
  { id: "compare", tab: "Compare", title: "Compare environments", text: "Put two environments side by side and see what differs. A value that cannot be worked out is shown as unknown, never as the same." },
];

/** Whether the page is in dark mode now, following the theme switch. */
function useDark(): boolean {
  const read = () => {
    const attr = document.documentElement.getAttribute("data-theme");
    return attr ? attr === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
  };
  const [dark, setDark] = useState(read);
  useEffect(() => {
    const observer = new MutationObserver(() => setDark(read()));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    return () => observer.disconnect();
  }, []);
  return dark;
}

function Showcase() {
  const [i, setI] = useState(0);
  const dark = useDark();
  const shot = SHOTS[i]!;
  return (
    <div className="mt-10">
      <div role="tablist" aria-label="Screens" className="mx-auto flex w-fit max-w-full flex-wrap justify-center gap-1 rounded-full border border-[var(--tf-color-border)] bg-[var(--tf-glass)] p-1 backdrop-blur">
        {SHOTS.map((s, n) => (
          <button
            key={s.id}
            role="tab"
            aria-selected={n === i}
            onClick={() => setI(n)}
            className={cn("rounded-full px-5 py-1.5 text-sm font-medium transition", n === i ? "bg-[var(--tf-color-text)] text-[var(--tf-color-canvas)]" : "text-[var(--tf-color-text-secondary)] hover:text-[var(--tf-color-text)]")}
          >
            {s.tab}
          </button>
        ))}
      </div>
      <div className="mx-auto mt-8 overflow-hidden rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] shadow-2xl">
        <div className="flex items-center gap-1.5 border-b border-[var(--tf-color-border)] bg-[var(--tf-color-surface-subtle)] px-4 py-2.5" aria-hidden>
          <span className="size-2.5 rounded-full bg-[var(--tf-color-error)] opacity-60" />
          <span className="size-2.5 rounded-full bg-[var(--tf-color-warning)] opacity-60" />
          <span className="size-2.5 rounded-full bg-[var(--tf-color-success)] opacity-60" />
        </div>
        <img src={`/landing/${shot.id}${dark ? "-dark" : ""}.png`} alt={`The ${shot.tab} screen of TerraFlow`} className="block w-full" loading="lazy" />
      </div>
      {/* A wrapper carries the gap: tokens.css resets h3 margins outside any layer, which beats a utility on the h3. */}
      <div className="mt-12">
        <h3 className="text-center text-xl font-semibold">{shot.title}</h3>
        <p className="mx-auto mb-0 mt-2 max-w-2xl text-center text-sm leading-6 text-[var(--tf-color-text-secondary)]">{shot.text}</p>
      </div>
    </div>
  );
}
