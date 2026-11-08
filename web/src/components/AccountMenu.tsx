import { LogOut } from "lucide-react";
import { devSignOut } from "../hooks/useAuth";
import type { User } from "../types/auth";

/** Who you are, your role, and a way to sign out (or switch user on the local sign-in). */
export function AccountMenu({ user, dev }: { user: User; dev: boolean }) {
  const admin = user.roles?.includes("devops");
  return (
    <div className="flex items-center gap-2 text-xs text-[var(--tf-color-text-secondary)]">
      <span className="hidden sm:inline">{user.name}</span>
      <span className="tf-badge" data-tone={admin ? "good" : "info"}>
        {admin ? "admin" : "developer"}
      </span>
      {dev ? (
        <button
          className="tf-btn tf-btn--ghost"
          style={{ minHeight: 28, padding: "0 var(--tf-space-2)", fontSize: 12 }}
          onClick={() => devSignOut()}
          title="Sign out and pick another user"
        >
          <LogOut className="size-3.5" /> Switch user
        </button>
      ) : (
        <a href="/auth/logout" className="text-[var(--tf-color-text-muted)] no-underline hover:underline">
          Logout
        </a>
      )}
    </div>
  );
}
