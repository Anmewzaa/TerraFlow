export interface User {
  oid: string;
  email: string;
  name: string;
  roles: string[];
  group_ids?: string[];
}
