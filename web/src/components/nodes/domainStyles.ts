import iconManifest from "../../design-system/azure-icons.json";

export const domainCssVars: Record<string, string> = {
  network: "var(--tf-cat-network)",
  compute: "var(--tf-cat-compute)",
  data: "var(--tf-cat-data)",
  security: "var(--tf-cat-security)",
  neutral: "var(--tf-cat-neutral)",
};

export const domainBgOpacity: Record<string, string> = {
  network: "rgba(0,114,178,0.1)",
  compute: "rgba(0,158,115,0.1)",
  data: "rgba(213,94,0,0.1)",
  security: "rgba(204,121,167,0.1)",
  neutral: "rgba(107,117,133,0.08)",
};

const manifest = iconManifest as Record<string, string>;
const kebab = (s: string) =>
  s.replace(/([a-z0-9])([A-Z])/g, "$1-$2").replace(/[_\s]+/g, "-").toLowerCase();

/**
 * The icon of a resource: from the manifest by module name or resource type, else by the icon name
 * the module's schema gives (x-tf-module.icon, e.g. "RouteTable"), else a path or small SVG data URI
 * written in the schema itself. null when there is none; the caller draws a neutral glyph.
 */
export function getAzureIcon(moduleOrType: string, schemaIcon?: string): string | null {
  const file = manifest[kebab(moduleOrType)] ?? (schemaIcon ? manifest[kebab(schemaIcon)] : undefined);
  if (file && !file.startsWith("_")) return `/azure-icons/${file}`;
  if (schemaIcon && (/^\/azure-icons\/[\w.-]+\.svg$/.test(schemaIcon) || /^data:image\/svg\+xml[;,]/.test(schemaIcon)) && schemaIcon.length < 20000) {
    return schemaIcon;
  }
  return null;
}

export const moduleDomainMap: Record<string, string> = {
  "resource-group": "neutral",
  "resource_group": "neutral",
  "virtual-network": "network",
  "virtual_network": "network",
  "network-security-group": "network",
  "network_security_group": "network",
  "key-vault": "security",
  "key_vault": "security",
  "storage-account": "data",
  "storage_account": "data",
  "subnet": "network",
  "managed-identity": "security",
  "managed_identity": "security",
  "private-endpoint": "network",
  "private_endpoint": "network",
  "private-dns-zone": "network",
  "private_dns_zone": "network",
  "linux-virtual-machine": "compute",
  "linux_virtual_machine": "compute",
};
