import type { User } from "../types/auth";

interface UserMenuProps {
  user: User;
}

export function UserMenu({ user }: UserMenuProps) {
  const role = user.roles?.[0] ?? "user";
  return (
    <div style={{
      display: "flex", alignItems: "center", gap: "var(--tf-space-2)",
      fontSize: "var(--tf-text-small)", color: "var(--tf-color-text-secondary)",
    }}>
      <span>{user.name}</span>
      <span style={{
        padding: "2px 8px", borderRadius: "var(--tf-radius-s)",
        background: role === "devops" ? "var(--tf-color-accent)" : "var(--tf-color-border)",
        color: role === "devops" ? "#fff" : "var(--tf-color-text)",
        fontSize: "var(--tf-text-xsmall)", fontWeight: 600, textTransform: "uppercase",
      }}>
        {role}
      </span>
      <a href="/auth/logout" style={{
        color: "var(--tf-color-text-muted)", textDecoration: "none",
      }}>Logout</a>
    </div>
  );
}
