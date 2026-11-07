package hcl

// Domain is the Azure grouping used for a node's colour and icon on the canvas.
// Only four domains carry a hue; everything else is neutral and is told apart by
// its icon + label (see docs/05-design-system.md).
type Domain string

const (
	DomainNetwork  Domain = "network"
	DomainCompute  Domain = "compute"
	DomainData     Domain = "data"
	DomainSecurity Domain = "security"
	DomainNeutral  Domain = "neutral" // resource groups, integration, external
)

// Domains returns the domains in canvas/legend order.
func Domains() []Domain {
	return []Domain{DomainNetwork, DomainCompute, DomainData, DomainSecurity, DomainNeutral}
}

// DomainForModule maps a module name (the catalog key, e.g. "virtual-network")
// to its domain. Unknown modules are neutral. This is a seed for Phase 1; the
// real mapping is driven by catalog metadata.
func DomainForModule(module string) Domain {
	switch module {
	case "virtual-network", "network-security-group", "private-dns-zone", "private-endpoint", "subnet":
		return DomainNetwork
	case "kubernetes", "app-service", "virtual-machine":
		return DomainCompute
	case "storage-account", "sql", "cosmos", "postgresql":
		return DomainData
	case "key-vault", "managed-identity":
		return DomainSecurity
	default:
		return DomainNeutral
	}
}
