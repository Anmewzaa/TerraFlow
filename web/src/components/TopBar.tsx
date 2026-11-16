interface Props {
  projectName: string;
  env: string;
  envOptions: string[];
  onEnvChange: (env: string) => void;
  draftCount: number;
  saving: boolean;
  onSave: () => void;
}

export function TopBar({
  projectName,
  env,
  envOptions,
  onEnvChange,
  draftCount,
  saving,
  onSave,
}: Props) {
  return (
    <header className="tf-topbar">
      <div className="tf-topbar__breadcrumb">
        <strong title={projectName}>{projectName}</strong>
        <span aria-hidden className="tf-topbar__env-crumb">/</span>
        <span className="tf-topbar__env-crumb">{env}</span>
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
          title={draftCount === 0 ? "Nothing to save yet: add or change a resource first" : "Save a version; publish it from Changesets"}
          style={{
            height: 28,
            padding: "0 var(--tf-space-3)",
            fontSize: 12,
            ...(draftCount === 0 ? { opacity: 0.5, cursor: "not-allowed" } : { background: "var(--tf-color-success)" }),
          }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" />
            <polyline points="17 21 17 13 7 13 7 21" />
            <polyline points="7 3 7 8 15 8" />
          </svg>
          <span className="whitespace-nowrap">Save version</span>
        </button>
      </div>
    </header>
  );
}
