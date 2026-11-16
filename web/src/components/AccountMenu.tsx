import { LogOut } from "lucide-react";
import { devSignOut } from "../hooks/useAuth";
import type { User } from "../types/auth";

/** Who you are (an initial, the name, your role) and a way to sign out, or to switch user on the local sign-in. */
export function AccountMenu({ user, dev }: { user: User; dev: boolean }) {
  const admin = user.roles?.includes("devops");
  const initial = (user.name || "?").trim().charAt(0).toUpperCase();
  const out = "ml-auto grid size-8 place-items-center rounded-full text-[var(--tf-color-text-muted)] transition hover:bg-[var(--tf-glass-strong)] hover:text-[var(--tf-color-text)]";
  return (
    <div className="flex w-full items-center gap-2.5">
      <span aria-hidden className="grid size-8 shrink-0 place-items-center rounded-full bg-[var(--tf-color-action)] text-xs font-semibold text-[var(--tf-color-on-action)]">
        {initial}
      </span>
      <span className="hidden min-w-0 flex-col leading-tight md:flex">
        <span className="max-w-[10rem] truncate text-[13px] font-medium text-[var(--tf-color-text)]">{user.name}</span>
        <span className="text-[11px] text-[var(--tf-color-text-muted)]">{admin ? "Administrator" : "Developer"}</span>
      </span>
      {dev ? (
        <button type="button" className={out} onClick={() => devSignOut()} title="Sign out and pick another user" aria-label="Switch user">
          <LogOut className="size-4" aria-hidden />
        </button>
      ) : (
        <a href="/auth/logout" className={out} title="Sign out" aria-label="Sign out">
          <LogOut className="size-4" aria-hidden />
        </a>
      )}
    </div>
  );
}

