import type React from "react";
import { cn } from "@/lib/utils";

interface DarkGradientBgProps {
  children?: React.ReactNode;
  className?: string;
}

const STREAKS = [
  "linear-gradient(90deg, transparent 0%, #000 20%, transparent 36%, #000 55%, rgba(0,0,0,.13) 67%, #000 78%, transparent 97%)",
  "linear-gradient(90deg, transparent 11%, #000 25%, rgba(0,0,0,.55) 41%, rgba(0,0,0,.13) 67%, #000 78%, transparent 97%)",
  "linear-gradient(90deg, transparent 9%, #000 20%, rgba(0,0,0,.55) 28%, rgba(0,0,0,.42) 40%, #000 48%, rgba(0,0,0,.27) 54%, rgba(0,0,0,.13) 78%, #000 88%, transparent 97%)",
  "linear-gradient(90deg, transparent 0%, #000 17%, rgba(0,0,0,.55) 26%, #000 35%, transparent 47%, rgba(0,0,0,.13) 69%, #000 79%, transparent 97%)",
];

/** Themed backdrop (dark: black; light: pale cyan): soft corner glow, skewed cyan streaks and a faint dot grid. */
export function DarkGradientBg({ children, className }: DarkGradientBgProps) {
  return (
    <div className={cn("relative isolate w-full overflow-hidden bg-[var(--tf-home-bg)]", className)}>
      <div className="pointer-events-none absolute inset-0 -z-10" aria-hidden>
        <div
          className="absolute inset-0"
          style={{
            background: "radial-gradient(100% 100% at 0% 0%, var(--tf-home-glow) 0%, var(--tf-home-bg) 100%)",
            maskImage: "radial-gradient(125% 100% at 0% 0%, #000 0%, rgba(0,0,0,.22) 88%, transparent 100%)",
            WebkitMaskImage: "radial-gradient(125% 100% at 0% 0%, #000 0%, rgba(0,0,0,.22) 88%, transparent 100%)",
          }}
        >
          {STREAKS.map((mask, i) => (
            <div
              key={i}
              className="absolute inset-0"
              style={{
                opacity: "var(--tf-streak-op)",
                background: "linear-gradient(rgb(0,207,255) 0%, rgba(0,207,255,0) 100%)",
                maskImage: mask,
                WebkitMaskImage: mask,
                transform: "skewX(45deg)",
              }}
            />
          ))}
        </div>
        <div
          className="absolute inset-0"
          style={{
            opacity: "var(--tf-dot-op)",
            backgroundImage: "radial-gradient(circle at 1px 1px, var(--tf-dot) 1px, transparent 0)",
            backgroundSize: "20px 20px",
          }}
        />
      </div>
      <div className="relative z-10 min-h-full">{children}</div>
    </div>
  );
}
