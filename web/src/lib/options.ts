/* Choices for an input (a dropdown) are defined in the module schema, never in UI code:
 *   x-tf-ui.options       a list of "value" or {value, label}
 *   x-tf-ui.options_from  the name of a shared list (below; the server only checks the name)
 *   x-tf-ui.allow_custom  also offer "Other..." so a value that is not listed can be typed
 * A plain `enum` stays a dropdown as before. As a convenience an input named `location` with no
 * choices of its own gets the Azure regions; `widget: "text"` turns that off. */
import type { JSONSchema } from "../types/catalog";

export interface Choice {
  value: string;
  label: string;
}

export interface ChoiceSet {
  choices: Choice[];
  /** A value that is not in the list may be typed. */
  allowCustom: boolean;
}

const region = (value: string, label: string): Choice => ({ value, label: `${label} (${value})` });

/** Azure regions, by geography. Typical for `location` of azurerm resources. */
export const AZURE_REGIONS: Choice[] = [
  region("southeastasia", "Southeast Asia"), region("eastasia", "East Asia"),
  region("japaneast", "Japan East"), region("japanwest", "Japan West"),
  region("koreacentral", "Korea Central"), region("koreasouth", "Korea South"),
  region("centralindia", "Central India"), region("southindia", "South India"), region("westindia", "West India"),
  region("australiaeast", "Australia East"), region("australiasoutheast", "Australia Southeast"),
  region("eastus", "East US"), region("eastus2", "East US 2"), region("centralus", "Central US"),
  region("northcentralus", "North Central US"), region("southcentralus", "South Central US"),
  region("westus", "West US"), region("westus2", "West US 2"), region("westus3", "West US 3"),
  region("canadacentral", "Canada Central"), region("canadaeast", "Canada East"),
  region("brazilsouth", "Brazil South"),
  region("northeurope", "North Europe"), region("westeurope", "West Europe"),
  region("uksouth", "UK South"), region("ukwest", "UK West"),
  region("francecentral", "France Central"), region("germanywestcentral", "Germany West Central"),
  region("switzerlandnorth", "Switzerland North"), region("swedencentral", "Sweden Central"),
  region("norwayeast", "Norway East"), region("italynorth", "Italy North"), region("polandcentral", "Poland Central"),
  region("uaenorth", "UAE North"), region("qatarcentral", "Qatar Central"), region("israelcentral", "Israel Central"),
  region("southafricanorth", "South Africa North"),
];

export const OPTION_PRESETS: Record<string, Choice[]> = { "azure-regions": AZURE_REGIONS };

const asChoice = (c: unknown): Choice | null => {
  if (typeof c === "string") return c ? { value: c, label: c } : null;
  if (c && typeof c === "object" && typeof (c as { value?: unknown }).value === "string") {
    const o = c as { value: string; label?: unknown };
    return { value: o.value, label: typeof o.label === "string" && o.label ? o.label : o.value };
  }
  return null;
};

/** The choices an input offers, or null when it is a free input. */
export function choicesOf(name: string, schema: JSONSchema): ChoiceSet | null {
  const ui = schema["x-tf-ui"] ?? {};
  if (ui.widget && ui.widget !== "select") return null; // another widget was asked for
  if (Array.isArray(ui.options)) {
    const choices = ui.options.map(asChoice).filter((c): c is Choice => !!c);
    return choices.length ? { choices, allowCustom: ui.allow_custom === true } : null;
  }
  if (ui.options_from) {
    const choices = OPTION_PRESETS[ui.options_from];
    return choices ? { choices, allowCustom: ui.allow_custom !== false } : null;
  }
  const type = Array.isArray(schema.type) ? schema.type[0] : schema.type;
  if (schema.enum) return null; // the enum dropdown handles it
  if (name === "location" && (type === "string" || type === undefined) && !ui.widget) {
    return { choices: AZURE_REGIONS, allowCustom: true };
  }
  return null;
}
