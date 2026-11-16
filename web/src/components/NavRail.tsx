import { useEffect, useRef, useState } from "react";
import { LogOut } from "lucide-react";
import { Logo } from "./Logo";
import { devSignOut } from "../hooks/useAuth";
import type { User } from "../types/auth";

interface Props {
  activeView: string;
  onViewChange: (view: string) => void;
  onHome?: () => void;
  user?: User | null;
  devSignIn?: boolean;
}

export function NavRail({ activeView, onViewChange, onHome, user, devSignIn }: Props) {
  return (
    <nav className="tf-nav-rail" aria-label="Main navigation">
      <div className="tf-nav-rail__logo" title="All projects" role="button" tabIndex={0} style={{ cursor: "pointer" }} onClick={onHome} onKeyDown={(e) => { if (e.key === "Enter") onHome?.(); }}>
        <Logo size={26} />
      </div>

      <NavItem icon="canvas" label="Canvas" active={activeView === "canvas"} onClick={() => onViewChange("canvas")} />
      <NavItem icon="hcl" label="HCL Preview" active={activeView === "hcl"} onClick={() => onViewChange("hcl")} />
      <NavItem icon="changesets" label="Changesets and review" active={activeView === "changesets"} onClick={() => onViewChange("changesets")} />
      <NavItem icon="compare" label="Compare environments" active={activeView === "compare"} onClick={() => onViewChange("compare")} />
      <NavItem icon="settings" label="Settings" active={activeView === "project"} onClick={() => onViewChange("project")} />

      <div className="tf-nav-rail__spacer" />
      {user && <RailAccount user={user} dev={!!devSignIn} />}

    </nav>
  );
}

const icons: Record<string, JSX.Element> = {
  canvas: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <rect x="14" y="14" width="7" height="7" rx="1" />
    </svg>
  ),
  changesets: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <polyline points="14 2 14 8 20 8" />
      <line x1="12" y1="18" x2="12" y2="12" />
      <line x1="9" y1="15" x2="15" y2="15" />
    </svg>
  ),
  hcl: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  ),
  review: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M9 11l3 3L22 4" />
      <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
    </svg>
  ),
  team: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  ),
  compare: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="m16 3 4 4-4 4" />
      <path d="M20 7H4" />
      <path d="m8 21-4-4 4-4" />
      <path d="M4 17h16" />
    </svg>
  ),
  admin: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 3l8 3v6c0 4.5-3.2 8.2-8 9-4.8-.8-8-4.5-8-9V6l8-3z" />
      <path d="M9 12l2 2 4-4" />
    </svg>
  ),
  audit: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <polyline points="14 2 14 8 20 8" />
      <line x1="8" y1="13" x2="16" y2="13" />
      <line x1="8" y1="17" x2="16" y2="17" />
    </svg>
  ),
  settings: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  ),
};

function NavItem({ icon, label, active, onClick }: { icon: string; label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      className={`tf-nav-rail__item${active ? " tf-nav-rail__item--active" : ""}`}
      data-label={label}
      aria-label={label}
      aria-current={active ? "page" : undefined}
      onClick={onClick}
    >
      {icons[icon]}
    </button>
  );
}

/** Who you are and signing out, at the bottom of the rail: an initial that opens a small card beside it. */
function RailAccount({ user, dev }: { user: User; dev: boolean }) {
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setOpen(false); };
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", away);
    document.addEventListener("keydown", esc);
    return () => { document.removeEventListener("mousedown", away); document.removeEventListener("keydown", esc); };
  }, [open]);
  const admin = user.roles?.includes("devops");
  const initial = (user.name || user.email || "?").trim().charAt(0).toUpperCase();
  const row = "flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-sm text-[var(--tf-color-text-secondary)] no-underline hover:bg-[var(--tf-glass-strong)] hover:text-[var(--tf-color-text)]";
  return (
    <div ref={box} className="relative mb-1">
      <button
        type="button"
        className="tf-nav-rail__avatar"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Account: ${user.name}`}
        title={user.name}
        onClick={() => setOpen((v) => !v)}
        style={{ marginBottom: 0 }}
      >
        {initial}
      </button>
      {open && (
        <div role="menu" className="tf-rail-pop absolute bottom-0 left-[calc(100%+10px)] z-[70] w-60 rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-2 text-[var(--tf-color-text)] shadow-[var(--tf-shadow-lift)]">
          <div className="flex items-center gap-3 px-2.5 py-2">
            <span aria-hidden className="grid size-9 shrink-0 place-items-center rounded-full bg-[var(--tf-color-action)] text-sm font-semibold text-[var(--tf-color-on-action)]">{initial}</span>
            <span className="min-w-0 leading-tight">
              <span className="block truncate text-sm font-semibold">{user.name}</span>
              <span className="block text-xs text-[var(--tf-color-text-muted)]">{admin ? "Administrator" : "Developer"}</span>
            </span>
          </div>
          <div className="my-1 h-px bg-[var(--tf-color-border)]" />
          {dev ? (
            <button type="button" role="menuitem" className={row} onClick={() => devSignOut()}>
              <LogOut className="size-4" aria-hidden /> Switch user
            </button>
          ) : (
            <a role="menuitem" href="/auth/logout" className={row}>
              <LogOut className="size-4" aria-hidden /> Sign out
            </a>
          )}
        </div>
      )}
    </div>
  );
}
