import { Box, Database, Network, Server, ShieldCheck } from "lucide-react";

const BY_DOMAIN = { network: Network, compute: Server, data: Database, security: ShieldCheck, neutral: Box } as const;

/**
 * What a resource shows when no icon is registered for it (see lib/azure-icons.json): one glyph per
 * domain, so a missing icon still tells the kind of resource instead of a blank or a broken image.
 */
export function ResourceGlyph({ domain, className = "size-5" }: { domain?: string; className?: string }) {
  const Icon = BY_DOMAIN[(domain as keyof typeof BY_DOMAIN) ?? "neutral"] ?? Box;
  return <Icon className={className} aria-hidden />;
}
