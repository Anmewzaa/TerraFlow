import { useCallback, useEffect, useState, type ReactNode } from "react";
import { Sparkles } from "lucide-react";
import { GlyphBackground } from "@/components/ui/backgrounds-glyph";

const KEY = "tf-canvas-glyphs";

/** Whether the glyph background is on (a per-person convenience; on unless the person turned it off). */
export function useGlyphPref(): [boolean, () => void] {
  const [on, setOn] = useState(() => {
    try {
      return localStorage.getItem(KEY) !== "off";
    } catch {
      return true;
    }
  });
  const toggle = useCallback(() => {
    setOn((v) => {
      try {
        localStorage.setItem(KEY, v ? "off" : "on");
      } catch {
        /* storage unavailable */
      }
      return !v;
    });
  }, []);
  return [on, toggle];
}

interface Colors {
  glyph: string;
  ring: string;
  haze: number;
}

/** The theme's colours, read from the page so light and dark both follow the design tokens. */
function readColors(): Colors {
  const root = getComputedStyle(document.documentElement);
  const attr = document.documentElement.getAttribute("data-theme");
  const dark = attr ? attr === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
  const ring = root.getPropertyValue("--tf-color-action").trim();
  return { glyph: dark ? "#cbd5e1" : "#334155", ring: ring || (dark ? "#67e8f9" : "#0e7490"), haze: dark ? 0.14 : 0.2 };
}

function useThemeColors(): Colors {
  const [colors, setColors] = useState<Colors>(readColors);
  useEffect(() => {
    const update = () => setColors(readColors());
    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    media.addEventListener("change", update);
    return () => {
      observer.disconnect();
      media.removeEventListener("change", update);
    };
  }, []);
  return colors;
}

/**
 * The drag-and-drop canvas on a field of drifting glyphs. A hover leaves a soft trail and a click sends a
 * ring through them. It follows light and dark, stops when the canvas is off screen or the tab is in the
 * background, and stays still for a person who asked the system for less motion.
 */
export function CanvasGlyphs({ enabled, soft, children }: { enabled: boolean; soft?: boolean; children: ReactNode }) {
  const colors = useThemeColors();
  if (!enabled) return <div className="size-full bg-[var(--tf-color-canvas)]">{children}</div>;
  return (
    <GlyphBackground className={`tf-glyph-canvas${soft ? " tf-glyph-soft" : ""} size-full bg-[var(--tf-color-canvas)] text-[var(--tf-color-text)]`} glowColor={colors.glyph} shapeColor={colors.ring} haze={soft ? colors.haze * 0.6 : colors.haze} fontSize={11}>
      {children}
    </GlyphBackground>
  );
}

/**
 * The page behind a full-screen view (changesets, HCL preview, compare, settings): the glyph field fills the
 * area and the view sits on top of it. A small switch in the corner turns the effect off.
 */
export function PageBackdrop({ children }: { children: ReactNode }) {
  const [on, toggle] = useGlyphPref();
  return (
    <div className="relative flex min-h-0 min-w-0 flex-1">
      <CanvasGlyphs enabled={on}>
        <div className="relative z-10 flex size-full min-h-0 min-w-0">{children}</div>
      </CanvasGlyphs>
      <button
        type="button"
        onClick={toggle}
        aria-pressed={on}
        title={on ? "Turn the background effect off" : "Turn the background effect on"}
        className="tf-btn tf-btn--ghost absolute bottom-3 right-3 z-20"
        style={{ height: 28, padding: "0 10px", fontSize: 12, background: "var(--tf-color-surface)", border: "1px solid var(--tf-color-border)" }}
      >
        <Sparkles className="size-3.5" aria-hidden /> {on ? "Effect on" : "Effect off"}
      </button>
    </div>
  );
}
