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

const moduleIconMap: Record<string, string> = {
  "resource-group": "/azure-icons/resource-group.svg",
  "resource_group": "/azure-icons/resource-group.svg",
  "virtual-network": "/azure-icons/virtual-network.svg",
  "virtual_network": "/azure-icons/virtual-network.svg",
  "network-security-group": "/azure-icons/network-security-group.svg",
  "network_security_group": "/azure-icons/network-security-group.svg",
  "key-vault": "/azure-icons/key-vault.svg",
  "key_vault": "/azure-icons/key-vault.svg",
  "storage-account": "/azure-icons/storage-account.svg",
  "storage_account": "/azure-icons/storage-account.svg",
  "subnet": "/azure-icons/subnet.svg",
  "managed-identity": "/azure-icons/managed-identity.svg",
  "managed_identity": "/azure-icons/managed-identity.svg",
  "private-endpoint": "/azure-icons/private-endpoint.svg",
  "private_endpoint": "/azure-icons/private-endpoint.svg",
  "private-dns-zone": "/azure-icons/private-dns-zone.svg",
  "private_dns_zone": "/azure-icons/private-dns-zone.svg",
};

export function getAzureIcon(moduleOrType: string): string | null {
  return moduleIconMap[moduleOrType] ?? null;
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
};
