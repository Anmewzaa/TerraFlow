import type { ReactNode } from "react";
import type { DevUser, User } from "../types/auth";
import { LoginPage } from "./LoginPage";

interface AuthGuardProps {
  user: User | null;
  loading: boolean;
  /** Set when the local sign-in is on: who one can pick. */
  devUsers?: DevUser[] | null;
  children: ReactNode;
}

export function AuthGuard({ user, loading, devUsers, children }: AuthGuardProps) {
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
  if (!user && devUsers) return <LoginPage users={devUsers} />;
  if (!user) {
    window.location.href = "/auth/login";
    return null;
  }
  return <>{children}</>;
}
