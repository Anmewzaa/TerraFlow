import type { ReactNode } from "react";
import { Logo } from "./Logo";
import type { DevUser, LoginProvider, User } from "../types/auth";
import { LandingPage } from "./LandingPage";
import { PendingPage } from "./PendingPage";

interface AuthGuardProps {
  user: User | null;
  loading: boolean;
  /** Set when the local sign-in is on: who one can pick. */
  devUsers?: DevUser[] | null;
  /** The sign-in buttons a real server offers (GitHub, Microsoft). */
  providers?: LoginProvider[] | null;
  /** Why the server sent a refused person back (?signin=code). */
  signinError?: string | null;
  children: ReactNode;
}

export function AuthGuard({ user, loading, devUsers, providers, signinError, children }: AuthGuardProps) {
  if (loading) {
    return (
      <div
        role="status"
        aria-label="Loading"
        style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 16, height: "100vh", color: "var(--tf-color-text-secondary)", fontFamily: "var(--tf-font-sans)" }}
      >
        <Logo size={44} />
        <div className="tf-progress" style={{ width: 120, borderRadius: 2 }} aria-hidden />
      </div>
    );
  }
  if (!user && (devUsers || (providers && providers.length > 0))) {
    return <LandingPage devUsers={devUsers} providers={providers} error={signinError ?? null} />;
  }
  if (user?.status === "pending") return <PendingPage user={user} />;
  if (!user) {
    window.location.href = "/auth/login";
    return null;
  }
  return <>{children}</>;
}
