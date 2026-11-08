const LEGEND_ITEMS = [
  { label: "Network", cssVar: "--tf-cat-network" },
  { label: "Compute", cssVar: "--tf-cat-compute" },
  { label: "Data", cssVar: "--tf-cat-data" },
  { label: "Security", cssVar: "--tf-cat-security" },
  { label: "Neutral", cssVar: "--tf-cat-neutral" },
];

export function CanvasLegend() {
  return (
    <div className="tf-legend">
      {LEGEND_ITEMS.map((item) => (
        <div key={item.label} className="tf-legend__item">
          <span
            className="tf-legend__dot"
            style={{ background: `var(${item.cssVar})` }}
          />
          {item.label}
        </div>
      ))}
    </div>
  );
}
