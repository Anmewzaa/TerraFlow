export interface Project {
  id: string;
  name: string;
  slug: string;
  description?: string;
  iac_path: string;
  source_type: string;
  created_at: string;
  updated_at: string;
}
