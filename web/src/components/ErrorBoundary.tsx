import { Component, type ErrorInfo, type ReactNode } from "react";

interface State {
  error: Error | null;
}

/** A render error shows a message here instead of leaving a blank page. */
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("UI crashed", error, info.componentStack);
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div role="alert" style={{ maxWidth: 560, margin: "15vh auto", padding: 24, fontFamily: "system-ui, sans-serif" }}>
        <h1 style={{ fontSize: 20, marginBottom: 8 }}>Something went wrong</h1>
        <p style={{ marginBottom: 12 }}>The page could not be drawn. Your draft is saved on the server.</p>
        <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, opacity: 0.8 }}>{this.state.error.message}</pre>
        <p style={{ marginTop: 16, display: "flex", gap: 8 }}>
          <button onClick={() => window.location.reload()}>Reload</button>
          <button
            onClick={() => {
              window.location.hash = "";
              window.location.reload();
            }}
          >
            Back to projects
          </button>
        </p>
      </div>
    );
  }
}
