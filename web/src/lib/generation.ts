/* A project's choices for the HCL TerraFlow writes (D-057): where new resources go by default,
 * project variables ({org} becomes var.org) and starter files. Mirrors server/internal/hcl/settings.go;
 * the server validates, this file only helps the screens. */

export interface ProjectVariable {
  name: string;
  description?: string;
  /** Value per environment (dev, prod...). */
  values?: Record<string, string>;
}

export interface StarterFile {
  filename: string;
  content: string;
}

export interface GenerationSettings {
  layout: {
    mode?: "catalog" | "single";
    folder?: string;
    file?: string;
    /** File name by catalog name, in catalog mode (resource-group -> rg.tf). */
    files?: Record<string, string>;
    /** Where an environment's folders are, with {stack} and {env}; empty is "{stack}" (8.9). */
    pattern?: string;
    /** Where an environment's values file is; empty is "{dir}/env/{env}.tfvars", or "none". */
    values?: string;
  };
  /** The project's environments, in order; empty means dev, staging, prod. */
  environments?: string[];
  variables?: ProjectVariable[];
  starters?: StarterFile[];
  /** Cloud providers the project uses, each at a version constraint (D-058). */
  providers?: { id: string; version?: string }[];
}

export const EMPTY_SETTINGS: GenerationSettings = { layout: {} };

export const DEFAULT_SINGLE_FOLDER = "terraform";
export const DEFAULT_SINGLE_FILE = "main.tf";

/** Where a new resource goes by default: the project's layout, else the module's own choice. */
export function targetFor(
  settings: GenerationSettings,
  catalogName: string,
  moduleFolder: string,
  moduleFile?: string,
): { folder: string; filename?: string } {
  const l = settings.layout ?? {};
  if (l.mode === "single") return { folder: l.folder || DEFAULT_SINGLE_FOLDER, filename: l.file || DEFAULT_SINGLE_FILE };
  const filename = l.files?.[catalogName] || moduleFile;
  return filename ? { folder: moduleFolder, filename } : { folder: moduleFolder };
}

const TOKEN = /\{([a-z][a-z0-9_]*)\}/g;

/** The {name} tokens in a text that are declared project variables. */
export function tokensIn(text: string, settings: GenerationSettings): string[] {
  const declared = new Set((settings.variables ?? []).map((v) => v.name));
  return [...text.matchAll(TOKEN)].map((m) => m[1]).filter((n) => declared.has(n));
}

/**
 * If a typed value contains the value of a project variable (aic in rg-aic-dev), offer to replace
 * it with the token. Never automatic: a short value can match inside another word.
 */
export function suggestToken(
  text: string,
  settings: GenerationSettings,
): { name: string; value: string; replaced: string } | null {
  if (!text || tokensIn(text, settings).length > 0) return null;
  let best: { name: string; value: string } | null = null;
  for (const v of settings.variables ?? []) {
    for (const value of new Set(Object.values(v.values ?? {}))) {
      if (value.length < 2 || !text.includes(value)) continue;
      if (!best || value.length > best.value.length) best = { name: v.name, value };
    }
  }
  return best && { ...best, replaced: text.replace(best.value, `{${best.name}}`) };
}

const ENV_NAME = /^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$/;
export const isEnvName = (s: string) => ENV_NAME.test(s);
export const VAR_NAME = /^[a-z][a-z0-9_]{0,40}$/;

/** The project's environments: the list it declares (8.9), plus any that has a value; dev first when no list. */
export function environmentsOf(settings: GenerationSettings): string[] {
  if (settings.environments?.length) {
    const extra = new Set<string>();
    for (const v of settings.variables ?? []) for (const e of Object.keys(v.values ?? {})) if (!settings.environments.includes(e)) extra.add(e);
    return [...settings.environments, ...[...extra].sort()];
  }
  const set = new Set<string>(["dev"]);
  for (const v of settings.variables ?? []) for (const e of Object.keys(v.values ?? {})) set.add(e);
  return [...set].sort((a, b) => (a === "dev" ? -1 : b === "dev" ? 1 : a.localeCompare(b)));
}

/** Starter files a team commonly wants; the owner edits the placeholders. */
export const STARTER_PRESETS: { label: string; file: StarterFile }[] = [
  {
    label: "backend.tf (azurerm state)",
    file: {
      filename: "backend.tf",
      content: 'terraform {\n  backend "azurerm" {\n    resource_group_name  = "REPLACE_ME"\n    storage_account_name = "REPLACE_ME"\n    container_name       = "tfstate"\n    key                  = "{folder}.tfstate"\n  }\n}\n',
    },
  },
];
