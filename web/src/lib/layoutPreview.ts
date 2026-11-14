/** Display only: the server decides what a layout is (8.9, Resolver). This shows the paths a pattern would give. */

export const DEFAULT_FOLDER_PATTERN = "{stack}";
export const DEFAULT_VALUES_PATTERN = "{dir}/env/{env}.tfvars";

export const ENV_PRESETS: { label: string; envs: string[] }[] = [
  { label: "dev, prod", envs: ["dev", "prod"] },
  { label: "dev, test, prod", envs: ["dev", "test", "prod"] },
  { label: "sbx, int, dev, test, stage, prod", envs: ["sbx", "int", "dev", "test", "stage", "prod"] },
];

export const LAYOUT_PRESETS: { label: string; text: string; pattern: string; values: string }[] = [
  { label: "vnet/dev", text: "A folder per stack, one folder per environment inside it", pattern: "{stack}/{env}", values: "{dir}/terraform.tfvars" },
  { label: "dev/vnet", text: "A folder per environment, one folder per stack inside it", pattern: "{env}/{stack}", values: "{dir}/terraform.tfvars" },
  { label: "dev", text: "One folder per environment, all its resources together", pattern: "{env}", values: "{dir}/terraform.tfvars" },
  { label: "vnet + env/dev.tfvars", text: "A folder per stack, a values file per environment (the default)", pattern: DEFAULT_FOLDER_PATTERN, values: DEFAULT_VALUES_PATTERN },
  { label: "one folder", text: "Everything in the project folder, a values file per environment", pattern: ".", values: "{dir}/{env}.tfvars" },
];

/** The folder a stack's files go to, or "" for the project folder. Without {stack} every stack shares the folder. */
export function folderFor(pattern: string | undefined, stack: string, env: string): string {
  const p = pattern || DEFAULT_FOLDER_PATTERN;
  if (p === ".") return "";
  return p.replace("{stack}", stack).replace("{env}", env);
}

export function valuesFor(pattern: string | undefined, values: string | undefined, stack: string, env: string): string | null {
  const v = values || DEFAULT_VALUES_PATTERN;
  if (v === "none") return null;
  const dir = folderFor(pattern, stack, env);
  const path = v.replace("{env}", env);
  return dir ? path.replace("{dir}", dir) : path.replace("{dir}/", "");
}

export function samplePaths(pattern: string | undefined, values: string | undefined, env: string): string[] {
  const join = (d: string, f: string) => (d ? `${d}/${f}` : f);
  const out = ["vnet", "storage"].map((s) => join(folderFor(pattern, s, env), s === "vnet" ? "vnet.tf" : "storage.tf"));
  const v = valuesFor(pattern, values, "vnet", env);
  if (v) out.push(v);
  return [...new Set(out)];
}
