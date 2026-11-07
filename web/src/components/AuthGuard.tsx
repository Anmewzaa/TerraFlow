import type { ReactNode } from "react";
import type { User } from "../types/auth";

interface AuthGuardProps {
  user: User | null;
  loading: boolean;
  children: ReactNode;
}

export function AuthGuard({ user, loading, children }: AuthGuardProps) {
  if (loading) {
    return (
      <div style={{
        display: "flex", alignItems: "center", justifyContent: "center",
        height: "100vh", color: "var(--tf-color-text-secondary)",
        fontFamily: "var(--tf-font-sans)",
      }}>
        Loading…
      </div>
    );
  }
  if (!user) {
    window.location.href = "/auth/login";
    return null;
  }
  return <>{children}</>;
}
