import { Hourglass } from "lucide-react";
import { DarkGradientBg } from "@/components/ui/elegant-dark-pattern";
import type { User } from "../types/auth";
import { Logo } from "./Logo";
import { ThemeToggle } from "./ThemeToggle";

const TEXT: Record<string, { title: string; body: string }> = {
  pending: {
    title: "Waiting for approval",
    body: "An administrator has to approve your account before you can use TerraFlow. You will not need to sign in again: this page updates when you reload it after approval.",
  },
  rejected: { title: "Not approved", body: "Your sign-up was not approved. Ask an administrator if you think this is a mistake." },
  suspended: { title: "Account suspended", body: "Your account is suspended. Ask an administrator." },
};

/** What a signed-in person who is not active (yet) sees: nothing else is reachable. */
export function PendingPage({ user }: { user: User }) {
  const t = TEXT[user.status ?? "pending"] ?? TEXT.pending;
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
        <h1 className="flex items-center gap-3 text-3xl font-bold tracking-tight">
          <Hourglass className="size-7" /> {t.title}
        </h1>
        <p className="mt-3 text-[var(--tf-color-text-secondary)]">{t.body}</p>
        <p className="mt-2 text-sm text-[var(--tf-color-text-muted)]">Signed in as {user.name || user.oid}.</p>
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
