import { cn } from "@/lib/utils";

/** Placeholder rows for a panel that is loading, in place of the word "Loading…". */
export function PanelLoading({ rows = 3, label = "Loading", className }: { rows?: number; label?: string; className?: string }) {
  return (
    <div role="status" aria-label={label} className={cn("tf-skeleton-stack", className)}>
      {Array.from({ length: rows }, (_, i) => (
        <span key={i} className="tf-skel tf-skeleton-card" style={{ animationDelay: `${i * 90}ms`, opacity: 1 - i * 0.12 }} />
      ))}
    </div>
  );
}

/** A line of placeholder text. */
export function SkeletonLine({ width = "60%" }: { width?: string }) {
  return <span className="tf-skel" style={{ width }} aria-hidden />;
}
