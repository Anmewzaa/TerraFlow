import type { ReactNode } from "react";
import type { DevUser, LoginProvider, User } from "../types/auth";
import { LoginPage } from "./LoginPage";
import { PendingPage } from "./PendingPage";
import { SignInPage } from "./SignInPage";

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
  if (!user && providers && providers.length > 0) return <SignInPage providers={providers} error={signinError ?? null} />;
  if (user?.status === "pending") return <PendingPage user={user} />;
  if (!user) {
    window.location.href = "/auth/login";
    return null;
  }
  return <>{children}</>;
}
