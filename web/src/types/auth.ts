export interface User {
  oid: string;
  email: string;
  name: string;
  roles: string[];
  group_ids?: string[];
}

/** A made-up user on the local sign-in page (auth disabled). */
export interface DevUser {
  oid: string;
  email: string;
  name: string;
  role: "devops" | "developer";
  note: string;
}
