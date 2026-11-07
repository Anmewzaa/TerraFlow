import { brandIdentity } from "./design-system/brand-identity";

// Phase 0 placeholder shell. The visualizer (React Flow + ELK) lands in Phase 1;
// see docs/08-dev-plan.md.
export function App() {
  return (
    <main style={{ maxWidth: 720, margin: "0 auto", padding: "var(--tf-space-8) var(--tf-space-4)" }}>
      <h1 style={{ fontSize: "var(--tf-text-heading)" }}>{brandIdentity.name}</h1>
      <p style={{ color: "var(--tf-color-text-secondary)" }}>{brandIdentity.tagline}</p>
      <p>
        <span className="tf-badge" data-tone="warn">Phase 0 — skeleton</span>
      </p>
      <p style={{ color: "var(--tf-color-text-muted)", fontSize: "var(--tf-text-small)" }}>
        {brandIdentity.role}
      </p>
    </main>
  );
}
