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
  onPublish: () => void;
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
  onPublish,
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
          onClick={onPublish}
          disabled={draftCount === 0}
          title={draftCount === 0 ? "Nothing to publish yet" : "Send the draft for review"}
          style={{
            height: 28,
            padding: "0 var(--tf-space-3)",
            fontSize: 12,
            background: "var(--tf-color-success)",
          }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M22 2L11 13" />
            <path d="M22 2l-7 20-4-9-9-4 20-7z" />
          </svg>
          Publish
        </button>
        {user && <AccountMenu user={user} dev={!!devSignIn} />}
        <ThemeToggle />
      </div>
    </header>
  );
}
