import { useState } from "react";
import { Layers, Loader2, ShieldCheck, User as UserIcon } from "lucide-react";
import { DarkGradientBg } from "@/components/ui/elegant-dark-pattern";
import { devSignIn } from "../hooks/useAuth";
import type { DevUser } from "../types/auth";
import { ThemeToggle } from "./ThemeToggle";

/** Local sign-in: pick a made-up user to try what an admin and a developer each see. */
export function LoginPage({ users }: { users: DevUser[] }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const pick = async (u: DevUser) => {
    setBusy(u.oid);
    setError(null);
    try {
      await devSignIn(u.oid);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not sign in");
      setBusy(null);
    }
  };

  return (
    <DarkGradientBg className="min-h-screen text-[var(--tf-color-text)]">
      <div className="absolute right-5 top-4">
        <ThemeToggle />
      </div>
      <main className="mx-auto flex min-h-screen max-w-3xl flex-col justify-center px-6 py-16">
        <div className="mb-3 flex items-center gap-2 text-[var(--tf-color-action)]">
          <Layers className="size-6" />
          <span className="text-sm font-semibold uppercase tracking-widest">TerraFlow</span>
        </div>
        <h1 className="text-4xl font-bold tracking-tight">Sign in</h1>
        <p className="mb-8 mt-2 max-w-xl text-[var(--tf-color-text-secondary)]">
          Authentication is switched off in this environment. Pick who you want to be; an admin and a developer see
          different things.
        </p>
        {error && (
          <p role="alert" className="mb-4 text-sm text-[var(--tf-color-error)]">
            {error}
          </p>
        )}
        <ul className="m-0 grid list-none gap-4 p-0 sm:grid-cols-3">
          {users.map((u) => {
            const admin = u.role === "devops";
            return (
              <li key={u.oid}>
                <button
                  onClick={() => pick(u)}
                  disabled={busy !== null}
                  className="flex h-full w-full flex-col items-start gap-3 rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-glass)] p-5 text-left backdrop-blur transition hover:-translate-y-0.5 hover:border-[var(--tf-color-action)] hover:bg-[var(--tf-glass-hover)] disabled:opacity-60"
                >
                  <span className="flex size-10 items-center justify-center rounded-xl bg-[var(--tf-glass-strong)] text-[var(--tf-color-action)]">
                    {busy === u.oid ? <Loader2 className="size-5 animate-spin" /> : admin ? <ShieldCheck className="size-5" /> : <UserIcon className="size-5" />}
                  </span>
                  <span>
                    <span className="block text-base font-semibold">{u.name}</span>
                    <span className="text-xs text-[var(--tf-color-text-muted)]">{u.email}</span>
                  </span>
                  <span className="tf-badge" data-tone={admin ? "good" : "info"}>
                    {admin ? "admin" : "developer"}
                  </span>
                  <span className="text-xs text-[var(--tf-color-text-secondary)]">{u.note}</span>
                </button>
              </li>
            );
          })}
        </ul>
      </main>
    </DarkGradientBg>
  );
}
