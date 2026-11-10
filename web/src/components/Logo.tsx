import { useId } from "react";

/**
 * The TerraFlow mark: a globe (terra) with water flowing across it, and one small node leaving it,
 * the branch the app stops at. Coloured on its own; use `mono` for one colour that follows the text.
 */
export function Logo({ size = 24, className, mono = false }: { size?: number; className?: string; mono?: boolean }) {
  const id = useId().replace(/:/g, "");
  const wave = (dy: number) => `M2 ${38 + dy}c10-9 18 9 30 0s16-7 30 0`;
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" className={className} role="img" aria-label="TerraFlow">
      {mono ? (
        <>
          <circle cx="30" cy="34" r="23.5" fill="none" stroke="currentColor" strokeWidth="3" />
          <path d="M9 38c7-6 14 6 21 0s12-5 21 0" fill="none" stroke="currentColor" strokeWidth="3.4" strokeLinecap="round" />
          <circle cx="52" cy="12" r="5.5" fill="currentColor" />
        </>
      ) : (
        <>
          <defs>
            <linearGradient id={`g${id}`} x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stopColor="#67E8F9" />
              <stop offset="1" stopColor="#0891B2" />
            </linearGradient>
            <clipPath id={`c${id}`}>
              <circle cx="30" cy="34" r="25" />
            </clipPath>
          </defs>
          <circle cx="30" cy="34" r="25" fill={`url(#g${id})`} />
          <g clipPath={`url(#c${id})`} fill="none" stroke="#fff" strokeLinecap="round" strokeWidth="3.6">
            <path d={wave(0)} />
            <path d={wave(11)} strokeOpacity=".55" />
            <path d={wave(-11)} strokeOpacity=".55" />
          </g>
          <circle cx="52" cy="12" r="5.5" fill="#fff" stroke="#0891B2" strokeWidth="1.2" />
          <circle cx="52" cy="12" r="2.2" fill="#0891B2" />
        </>
      )}
    </svg>
  );
}
