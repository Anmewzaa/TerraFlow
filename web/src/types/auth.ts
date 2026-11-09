export interface User {
  oid: string;
  email: string;
  name: string;
  roles: string[];
  group_ids?: string[];
  /** active, pending, rejected or suspended; absent with the local sign-in. */
  status?: "active" | "pending" | "rejected" | "suspended";
}

/** A made-up user on the local sign-in page (auth disabled). */
export interface DevUser {
  oid: string;
  email: string;
  name: string;
  role: "devops" | "developer";
  note: string;
}
