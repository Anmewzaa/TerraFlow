import { Github, LogIn } from "lucide-react";
import { DarkGradientBg } from "@/components/ui/elegant-dark-pattern";
import type { LoginProvider } from "../types/auth";
import { Logo } from "./Logo";

/** What the server puts in `?signin=` when it refuses a sign-in, in words a person can act on. */
export const SIGNIN_ERRORS: Record<string, string> = {
  state: "The sign-in could not be verified. Please try again.",
  denied: "Access was not granted on the provider's page.",
  code: "The provider did not accept the sign-in. Please try again.",
  github: "GitHub could not be reached. Please try again.",
  error: "Something went wrong while signing in. Please try again.",
  rejected: "Your sign-up was not approved. Ask an administrator if you think this is a mistake.",
  suspended: "Your account has been suspended. Ask an administrator.",
  full: "Too many people are waiting for approval right now. Please try again later.",
  not_invited: "Sign-up is by invitation only. Ask an administrator for an invitation.",
};

/** The sign-in page of a real server: one button per provider the server has set up. */
export function SignInPage({ providers, error }: { providers: LoginProvider[]; error: string | null }) {
  return (
    <DarkGradientBg className="min-h-screen text-[var(--tf-color-text)]">
      <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-6 py-16">
        <div className="mb-3 flex items-center gap-2 text-[var(--tf-color-action-active)]">
          <Logo size={26} />
          <span className="text-sm font-semibold uppercase tracking-widest">TerraFlow</span>
        </div>
        <h1 className="text-4xl font-bold tracking-tight">Sign in</h1>
        <p className="mb-8 mt-2 text-[var(--tf-color-text-secondary)]">
          Compose Terraform from approved modules and ship a branch for review.
        </p>
        {error && (
          <p role="alert" className="mb-4 rounded-lg bg-[var(--tf-color-error-surface)] px-3 py-2 text-sm text-[var(--tf-color-error)]">
            {SIGNIN_ERRORS[error] ?? "Sign-in failed."}
          </p>
        )}
        <ul className="m-0 grid list-none gap-3 p-0">
          {providers.map((p) => (
            <li key={p.id}>
              <a href={p.href} className="tf-btn flex w-full items-center justify-center gap-2 py-3">
                {p.id === "github" ? <Github className="size-5" /> : <LogIn className="size-5" />}
                Continue with {p.label}
              </a>
            </li>
          ))}
        </ul>
        <p className="mt-6 text-xs text-[var(--tf-color-text-muted)]">
          New here? After your first sign-in an administrator approves your account before you can open anything.
        </p>
      </main>
    </DarkGradientBg>
  );
}
