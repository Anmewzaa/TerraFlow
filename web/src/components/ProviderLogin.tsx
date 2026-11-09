import { useEffect, useState } from "react";
import { DarkGradientBg } from "@/components/ui/elegant-dark-pattern";
import { Github, KeyRound } from "lucide-react";
import { Logo } from "./Logo";
import { ThemeToggle } from "./ThemeToggle";

interface Provider {
  id: string;
  label: string;
  url: string;
}

/** What the server puts in `?signin=` when it sends the browser back from a failed sign-in. */
export const SIGNIN_MESSAGES: Record<string, string> = {
  rejected: "Your sign-up was not approved. Ask an administrator if you think this is a mistake.",
  suspended: "Your account is suspended. Ask an administrator.",
  invite_only: "Sign-up is by invitation only. Ask a project owner to invite you.",
  full: "Sign-up is full for now. Please try again later.",
  state: "The sign-in could not be verified. Please try again.",
  denied: "GitHub access was not granted.",
  code: "GitHub did not accept the sign-in. Please try again.",
  github: "GitHub could not be reached. Please try again.",
};

export function signinMessage(search: string): string | null {
  const code = new URLSearchParams(search).get("signin");
  return code ? (SIGNIN_MESSAGES[code] ?? "The sign-in did not work. Please try again.") : null;
}

/**
 * Real sign-in: a button for each provider the server offers. With a single provider
 * and nothing to report, the browser goes straight to it.
 */
export function ProviderLogin() {
  const [providers, setProviders] = useState<Provider[] | null>(null);
  const [failed, setFailed] = useState(false);
  const message = signinMessage(window.location.search);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/auth/providers")
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then((b: { providers: Provider[] }) => {
        if (cancelled) return;
        if (b.providers.length === 1 && !message) {
          window.location.href = b.providers[0].url;
          return;
        }
        setProviders(b.providers);
      })
      .catch(() => !cancelled && setFailed(true));
    return () => {
      cancelled = true;
    };
  }, [message]);

  return (
    <DarkGradientBg className="min-h-screen text-[var(--tf-color-text)]">
      <div className="absolute right-5 top-4">
        <ThemeToggle />
      </div>
      <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-6 py-16">
        <div className="mb-3 flex items-center gap-2 text-[var(--tf-color-action-active)]">
          <Logo size={26} />
          <span className="text-sm font-semibold uppercase tracking-widest">TerraFlow</span>
        </div>
        <h1 className="text-4xl font-bold tracking-tight">Sign in</h1>
        {message && (
          <p role="alert" className="mt-4 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
            {message}
          </p>
        )}
        {failed && (
          <p role="alert" className="mt-4 text-sm text-[var(--tf-color-error)]">
            Cannot reach the server.
          </p>
        )}
        {providers && providers.length === 0 && (
          <p className="mt-4 text-sm text-[var(--tf-color-text-secondary)]">No sign-in method is set up on this server.</p>
        )}
        <ul className="m-0 mt-6 grid list-none gap-3 p-0">
          {providers?.map((p) => (
            <li key={p.id}>
              <a className="tf-btn flex w-full justify-center" href={p.url}>
                {p.id === "github" ? <Github className="size-4" /> : <KeyRound className="size-4" />} Sign in with {p.label}
              </a>
            </li>
          ))}
        </ul>
      </main>
    </DarkGradientBg>
  );
}
