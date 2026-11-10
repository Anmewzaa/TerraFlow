import { useState, useEffect } from "react";
import type { DevUser, LoginProvider, User } from "../types/auth";

/**
 * Who is signed in. With auth disabled the server offers made-up users to
 * pick from (devUsers); with Entra a missing user means "redirect to login".
 */
export function useAuth() {
  const [user, setUser] = useState<User | null>(null);
  const [devUsers, setDevUsers] = useState<DevUser[] | null>(null);
  const [devMode, setDevMode] = useState(false);
  const [providers, setProviders] = useState<LoginProvider[] | null>(null);
  // The server sends a refused person back to /?signin=<code>; read once, then tidy the URL.
  const [signinError] = useState<string | null>(() => {
    const code = new URLSearchParams(window.location.search).get("signin");
    if (code) window.history.replaceState(null, "", window.location.pathname + window.location.hash);
    return code;
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const r = await fetch("/api/me");
        if (r.ok) {
          if (!cancelled) setUser(await r.json());
          // Is this the local sign-in? Then the menu says "Switch user".
          const d = await fetch("/api/dev/users").catch(() => null);
          if (d?.ok && !cancelled) setDevMode(true);
          return;
        }
      } catch {
        /* fall through: not signed in */
      }
      try {
        const d = await fetch("/api/dev/users");
        if (d.ok) {
          const body = (await d.json()) as { users: DevUser[] };
          if (!cancelled) {
            setDevUsers(body.users);
            setDevMode(true);
          }
        }
      } catch {
        /* no dev sign-in: a real provider */
      }
      try {
        const p = await fetch("/auth/providers");
        if (p.ok) {
          const body = (await p.json()) as { providers: LoginProvider[] };
          if (!cancelled) setProviders(body.providers);
        }
      } catch {
        /* the server does not list providers: the guard falls back to Entra */
      }
    })().finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  return { user, devUsers, devMode, providers, signinError, loading };
}

/** Pick a dev user, then reload so everything is fetched as them. */
export async function devSignIn(oid: string): Promise<void> {
  const r = await fetch("/api/dev/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ oid }),
  });
  if (!r.ok) throw new Error("Could not sign in");
  window.location.reload();
}

export async function devSignOut(): Promise<void> {
  await fetch("/api/dev/logout", { method: "POST" });
  window.location.hash = "";
  window.location.reload();
}
