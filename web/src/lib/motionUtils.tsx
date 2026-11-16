/* Small motion helpers the glyph background needs. The component was written against
 * @animate-ui/react/utils; these are the same three things on top of `motion` alone, so the app
 * does not take a second animation package for them. */
import * as React from "react";
import { MotionConfig, useReducedMotion } from "motion/react";

/** Makes every motion child follow the person's "reduce motion" setting. */
export function ReducedMotionConfig({ children }: { children: React.ReactNode }) {
  return <MotionConfig reducedMotion="user">{children}</MotionConfig>;
}

/** True when the person asked the system for less motion. */
export function useShouldReduceMotion(): boolean {
  return useReducedMotion() ?? false;
}

const subscribe = (cb: () => void) => {
  document.addEventListener("visibilitychange", cb);
  return () => document.removeEventListener("visibilitychange", cb);
};

/** False while the tab is in the background, so nothing animates where nobody looks. */
export function usePageVisible(): boolean {
  return React.useSyncExternalStore(
    subscribe,
    () => document.visibilityState !== "hidden",
    () => true,
  );
}
