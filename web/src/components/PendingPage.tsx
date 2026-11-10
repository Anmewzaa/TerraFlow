import { Clock } from "lucide-react";
import { DarkGradientBg } from "@/components/ui/elegant-dark-pattern";
import type { User } from "../types/auth";
import { Logo } from "./Logo";
import { ThemeToggle } from "./ThemeToggle";

/** Shown to a person who signed in but has not been approved yet: nothing else is open to them. */
export function PendingPage({ user }: { user: User }) {
  return (
    <DarkGradientBg className="min-h-screen text-[var(--tf-color-text)]">
      <div className="absolute right-5 top-4">
        <ThemeToggle />
      </div>
      <main className="mx-auto flex min-h-screen max-w-lg flex-col justify-center px-6 py-16">
        <div className="mb-3 flex items-center gap-2 text-[var(--tf-color-action-active)]">
          <Logo size={26} />
          <span className="text-sm font-semibold uppercase tracking-widest">TerraFlow</span>
        </div>
        <span className="mb-4 flex size-12 items-center justify-center rounded-2xl bg-[var(--tf-glass-strong)] text-[var(--tf-color-action)]">
          <Clock className="size-6" />
        </span>
        <h1 className="text-3xl font-bold tracking-tight">Waiting for approval</h1>
        <p className="mt-2 text-[var(--tf-color-text-secondary)]">
          Thanks, {user.name || "you"}. An administrator has to approve your account before you can use TerraFlow. You can
          close this page; sign in again later, or reload once you have been told you are approved.
        </p>
        <div className="mt-6 flex gap-2">
          <button className="tf-btn" onClick={() => window.location.reload()}>
            Check again
          </button>
          <a className="tf-btn tf-btn--ghost" href="/auth/logout">
            Sign out
          </a>
        </div>
      </main>
    </DarkGradientBg>
  );
}
