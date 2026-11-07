// TerraFlow brand identity. Single source for naming, palette anchors and
// principles. Keep in sync with brand-tokens.css; do not restate raw hex in
// feature code — import from here or use the CSS tokens.
export const brandIdentity = {
  name: "TerraFlow",
  system: "Harbor",
  palette: "Sapphire & Gold",
  version: "0.1.0",
  tagline: "Compose infrastructure. Keep the repo in control.",
  role:
    "A visual composer that turns an approved module catalog into reviewed " +
    "Terraform branches on Azure DevOps. It never touches Azure and holds no state.",
  anchors: {
    sapphire: "#163F8C", // primary action
    sapphireDeep: "#0A1F4D", // ink / deep navy
    gold: "#C68A1C", // brand mark + active indicator only (fill/large, not thin marks)
    cloud: "#F4F6FA", // light canvas
    night: "#0D1526", // dark canvas (deep navy)
  },
  type: {
    ui: { family: "Inter", license: "SIL OFL 1.1", weights: [400, 500, 600, 700] },
    code: {
      family: "JetBrains Mono",
      license: "SIL OFL 1.1",
      use: "resource names, HCL, module sources, diffs — everything that is an identifier",
    },
    note: "Self-hosted and pinned. No webfont CDN at runtime (air-gapped).",
  },
  icons: {
    set: "Official Azure architecture icons, self-hosted in web/public/azure-icons/",
    rule: "Used unmodified. Domain is shown by a coloured accent bar, never by recolouring the icon.",
  },
  // Resource-domain colors are semantic, not decorative. Only four domains carry
  // a hue (Okabe-Ito based, validated colorblind-safe in both themes); the rest
  // use the neutral and are told apart by their icon + label. Color is a
  // secondary cue — every node also has an icon and a text label.
  domains: {
    network: { color: "#0072B2", of: "vnet, subnet, nsg, private dns, private endpoint" },
    compute: { color: "#009E73", of: "aks, app service, vm" },
    data: { color: "#D55E00", of: "storage, sql, cosmos, postgres" },
    security: { color: "#CC79A7", of: "key vault, managed identity" },
    neutral: { color: "#6B7585", of: "resource group (container frame), integration, external/hub data-source" },
  },
  principles: [
    "Dense and scannable before pretty — an operator reads 80 nodes, not 5.",
    "Honest unknown states — never render a guess as a fact. Unresolved = marked unresolved.",
    "The identifier is sacred — resource names and HCL are monospace, selectable, never truncated silently.",
    "Restraint over decoration — deep sapphire, one gold accent, pure status colors, tight radii. A bank control plane, not a landing page.",
    "A corporate-bank blue, not Azure blue — deeper and more saturated, so it does not read as a stock Microsoft template.",
    "Gold is an accent, not a second button color — primary actions are all sapphire; gold marks identity and the active item only.",
    "Palette validated, not eyeballed — domain hues pass a colorblind-safety and contrast check (dataviz skill).",
    "Status stays teal/green/amber/red — never the brand blue — so a status color only ever means status.",
    "Same geometry in light and dark — only semantic colors change.",
    "Read-only looks read-only — derived/locked values are visibly inert.",
  ],
} as const;
