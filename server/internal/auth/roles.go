package auth

type RoleConfig struct {
	DevOpsGroupID    string
	DeveloperGroupID string
}

func MapRoles(cfg RoleConfig, groupIDs []string) []string {
	if cfg.DevOpsGroupID == "" && cfg.DeveloperGroupID == "" {
		return []string{"developer"}
	}
	var roles []string
	for _, gid := range groupIDs {
		switch gid {
		case cfg.DevOpsGroupID:
			roles = append(roles, "devops")
		case cfg.DeveloperGroupID:
			roles = append(roles, "developer")
		}
	}
	return roles
}
