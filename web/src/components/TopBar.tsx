import { ThemeToggle } from "./ThemeToggle";
import { AccountMenu } from "./AccountMenu";
import type { User } from "../types/auth";
interface Props {
  projectName: string;
  env: string;
  envOptions: string[];
  onEnvChange: (env: string) => void;
  draftCount: number;
  saving: boolean;
  onSave: () => void;
  user?: User | null;
  devSignIn?: boolean;
}

export function TopBar({
  projectName,
  env,
  envOptions,
  onEnvChange,
  draftCount,
  saving,
  onSave,
  user,
  devSignIn,
}: Props) {
  return (
    <header className="tf-topbar">
      <div className="tf-topbar__breadcrumb">
        <strong>{projectName}</strong>
        <span>/</span>
        <span>{env}</span>
      </div>

      <div className="tf-topbar__env-switch">
        {envOptions.map((e) => (
          <button
            key={e}
            className={`tf-topbar__env-btn${e === env ? " tf-topbar__env-btn--active" : ""}`}
            onClick={() => onEnvChange(e)}
          >
            {e}
          </button>
        ))}
      </div>

      <div className="tf-topbar__actions">
        {saving && (
          <span style={{ fontSize: "var(--tf-text-caption)", color: "var(--tf-color-text-muted)" }}>
            Saving...
          </span>
        )}
        {draftCount > 0 && (
          <span className="tf-badge" data-tone="warn" style={{ fontSize: 11 }}>
            Draft · {draftCount} change{draftCount !== 1 ? "s" : ""}
          </span>
        )}
        <button
          className="tf-btn"
          onClick={onSave}
          disabled={draftCount === 0}
          title={draftCount === 0 ? "Nothing to save yet" : "Save a version; publish it from Changesets"}
          style={{
            height: 28,
            padding: "0 var(--tf-space-3)",
            fontSize: 12,
            background: "var(--tf-color-success)",
          }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" />
            <polyline points="17 21 17 13 7 13 7 21" />
            <polyline points="7 3 7 8 15 8" />
          </svg>
          Save
        </button>
        {user && <AccountMenu user={user} dev={!!devSignIn} />}
        <ThemeToggle />
      </div>
    </header>
  );
}
