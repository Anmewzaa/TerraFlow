/** The TerraFlow mark: a terra block with a flow running through it. Takes its colour from the text colour. */
export function Logo({ size = 24, className }: { size?: number; className?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      role="img"
      aria-label="TerraFlow"
    >
      <path d="M12 2.5l8.5 4.75v9.5L12 21.5l-8.5-4.75v-9.5z" />
      <path d="M7.5 10c2.5-2 4.5 2 9 0" />
      <path d="M7.5 14c2.5-2 4.5 2 9 0" />
    </svg>
  );
}
