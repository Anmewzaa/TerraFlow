import type { GenerateOutput } from "../types/changeset";

interface Props {
  preview: GenerateOutput | null;
  loading: boolean;
  error: string | null;
  onLoad: () => void;
  onClose: () => void;
}

export function HclPreviewPanel({ preview, loading, error, onLoad, onClose }: Props) {
  return (
    <div
      style={{
        width: 480,
        borderLeft: "1px solid var(--tf-color-border)",
        background: "var(--tf-color-surface)",
        display: "flex",
        flexDirection: "column",
        flexShrink: 0,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "var(--tf-space-2) var(--tf-space-3)",
          borderBottom: "1px solid var(--tf-color-border)",
        }}
      >
        <span style={{ fontWeight: 600, fontSize: "var(--tf-text-body)" }}>
          HCL Preview
        </span>
        <div style={{ display: "flex", gap: "var(--tf-space-2)" }}>
          <button
            onClick={onLoad}
            disabled={loading}
            style={{
              padding: "var(--tf-space-1) var(--tf-space-3)",
              fontSize: "var(--tf-text-small)",
              cursor: loading ? "default" : "pointer",
              borderRadius: "var(--tf-radius-s)",
              border: "1px solid var(--tf-color-border)",
              background: "transparent",
              color: "var(--tf-color-text-secondary)",
            }}
          >
            {loading ? "Generating…" : "Refresh"}
          </button>
          <button
            onClick={onClose}
            style={{
              padding: "var(--tf-space-1) var(--tf-space-2)",
              fontSize: "var(--tf-text-small)",
              cursor: "pointer",
              borderRadius: "var(--tf-radius-s)",
              border: "1px solid var(--tf-color-border)",
              background: "transparent",
              color: "var(--tf-color-text-secondary)",
            }}
          >
            Close
          </button>
        </div>
      </div>
      <div style={{ flex: 1, overflow: "auto", padding: "var(--tf-space-3)" }}>
        {error && (
          <div style={{ color: "var(--tf-color-bad)", marginBottom: "var(--tf-space-3)" }}>
            {error}
          </div>
        )}
        {!preview && !loading && !error && (
          <div style={{ color: "var(--tf-color-text-muted)" }}>
            Click Refresh to generate HCL preview.
          </div>
        )}
        {preview && preview.files.length === 0 && (
          <div style={{ color: "var(--tf-color-text-muted)" }}>
            No modules on canvas. Drag modules from the catalog to get started.
          </div>
        )}
        {preview &&
          preview.files.map((f) => (
            <div key={`${f.folder}/${f.filename}`} style={{ marginBottom: "var(--tf-space-4)" }}>
              <div
                style={{
                  fontSize: "var(--tf-text-small)",
                  fontWeight: 600,
                  color: "var(--tf-color-text-secondary)",
                  marginBottom: "var(--tf-space-1)",
                  fontFamily: "var(--tf-font-mono)",
                }}
              >
                {f.folder}/{f.filename}
              </div>
              <pre
                style={{
                  margin: 0,
                  padding: "var(--tf-space-3)",
                  background: "var(--tf-color-canvas)",
                  borderRadius: "var(--tf-radius-m)",
                  border: "1px solid var(--tf-color-border)",
                  fontSize: "var(--tf-text-small)",
                  fontFamily: "var(--tf-font-mono)",
                  whiteSpace: "pre-wrap",
                  overflow: "auto",
                }}
              >
                {f.content}
              </pre>
            </div>
          ))}
      </div>
    </div>
  );
}
