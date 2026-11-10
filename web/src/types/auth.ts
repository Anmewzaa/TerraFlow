export interface User {
  oid: string;
  email: string;
  name: string;
  roles: string[];
  group_ids?: string[];
  /** "pending" while an admin has not approved this person yet. */
  status?: "pending";
}

/** One sign-in button of the login page (GET /auth/providers). */
export interface LoginProvider {
  id: string;
  label: string;
  href: string;
}

/** A made-up user on the local sign-in page (auth disabled). */
export interface DevUser {
  oid: string;
  email: string;
  name: string;
  role: "devops" | "developer";
  note: string;
}
