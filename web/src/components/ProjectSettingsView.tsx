import { useEffect, useMemo, useState } from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { getGeneration, putGeneration, SettingsError } from "../hooks/useGeneration";
import { canApproveRole, useTeam } from "../hooks/useGitHub";
import { useProviders } from "../hooks/useProviders";
import { ProviderLogo } from "./admin/ProvidersPanel";
import {
  DEFAULT_SINGLE_FILE,
  DEFAULT_SINGLE_FOLDER,
  EMPTY_SETTINGS,
  environmentsOf,
  isEnvName,
  STARTER_PRESETS,
  VAR_NAME,
  type GenerationSettings,
  type ProjectVariable,
} from "../lib/generation";
import type { ModuleSummary } from "../types/catalog";

const field =
  "mt-1 block w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none transition focus:border-[var(--tf-color-action)] focus:ring-2 focus:ring-[var(--tf-color-action)]/25 disabled:opacity-60";

function Section({ id, title, hint, children }: { id?: string; title: string; hint: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-4 rounded-xl border border-[var(--tf-color-border)] p-5">
      <h2 className="m-0 text-base font-semibold">{title}</h2>
      <p className="m-0 mb-4 mt-1 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">{hint}</p>
      {children}
    </section>
  );
}

interface Props {
  projectId: string;
  modules: ModuleSummary[];
  isAdmin: boolean;
  onSaved: () => void;
  /** Shown inside the project page: no scrolling, padding or page title of its own. */
  embedded?: boolean;
}

/**
 * Project settings for generated HCL (D-057): where new resources go by default, project
 * variables ({org} becomes var.org in the generated code) and starter files. Owners and
 * co-owners change them; everyone in the project can read them.
 */
export function ProjectSettingsView({ projectId, modules, isAdmin, onSaved, embedded }: Props) {
  const { team } = useTeam(projectId);
  const { providers: registry } = useProviders();
  const canEdit = isAdmin || canApproveRole(team?.my_role);
  const [draft, setDraft] = useState<GenerationSettings | null>(null);
  const [saved, setSaved] = useState("");
  const [problems, setProblems] = useState<string[]>([]);
  const [msg, setMsg] = useState<{ tone: "ok" | "error"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let off = false;
    getGeneration(projectId)
      .then((s) => {
        if (off) return;
        setDraft(s);
        setSaved(JSON.stringify(s));
      })
      .catch((e) => !off && setMsg({ tone: "error", text: e instanceof Error ? e.message : "Cannot load the settings" }));
    return () => {
      off = true;
    };
  }, [projectId]);

  const dirty = draft !== null && JSON.stringify(draft) !== saved;
  const envs = useMemo(() => environmentsOf(draft ?? EMPTY_SETTINGS), [draft]);

  if (!draft) {
    return <div className="p-8 text-sm text-[var(--tf-color-text-muted)]">{msg ? msg.text : "Loading…"}</div>;
  }

  const set = (patch: Partial<GenerationSettings>) => setDraft({ ...draft, ...patch });
  const layout = draft.layout;
  const vars = draft.variables ?? [];
  const starters = draft.starters ?? [];
  const setVar = (i: number, v: ProjectVariable) => set({ variables: vars.map((x, j) => (j === i ? v : x)) });

  const save = async () => {
    setBusy(true);
    setMsg(null);
    setProblems([]);
    try {
      // An environment with an empty value has no value: drop it rather than write `x = ""`.
      const clean: GenerationSettings = {
        ...draft,
        variables: (draft.variables ?? []).map((v) => ({
          ...v,
          values: Object.fromEntries(Object.entries(v.values ?? {}).filter(([, x]) => x !== "")),
        })),
      };
      const next = await putGeneration(projectId, clean);
      setDraft(next);
      setSaved(JSON.stringify(next));
      setMsg({ tone: "ok", text: "Saved. New resources and the next HCL preview use these settings." });
      onSaved();
    } catch (e) {
      if (e instanceof SettingsError) setProblems(e.problems.length ? e.problems : [e.message]);
      setMsg({ tone: "error", text: e instanceof Error ? e.message : "Could not save" });
    } finally {
      setBusy(false);
    }
  };

  const addEnv = () => {
    const name = window.prompt("Environment name (for example prod, uat)")?.trim();
    if (!name) return;
    if (!isEnvName(name)) return setMsg({ tone: "error", text: `“${name}” is not a valid environment name (letters, digits, - and _).` });
    // An environment exists by having a value: give the first variable an empty slot so it shows up.
    if (vars.length > 0 && !envs.includes(name)) setVar(0, { ...vars[0], values: { ...vars[0].values, [name]: "" } });
    else setMsg({ tone: "ok", text: `Add a variable first, then set its value for ${name}.` });
  };

  return (
    <div className={embedded ? "" : "flex-1 overflow-y-auto p-6 md:p-8"}>
      <div className={embedded ? "space-y-6" : "mx-auto max-w-4xl space-y-6"}>
        <header className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="m-0 text-xl font-semibold">Settings</h2>
            <p className="m-0 mt-1 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">
              How the Terraform for this project is written. People keep dragging and dropping as before; these settings decide the defaults
              and the extra files.
            </p>
          </div>
          {canEdit ? (
            <button className="tf-btn" disabled={busy || !dirty} onClick={save}>
              {busy && <Loader2 className="size-4 animate-spin" />} Save settings
            </button>
          ) : (
            <span className="tf-badge" data-tone="neutral">
              Read only: owners and co-owners can change these
            </span>
          )}
        </header>

        {msg && (
          <p
            role={msg.tone === "error" ? "alert" : "status"}
            className="m-0 rounded-lg border px-3 py-2 text-sm"
            style={{ borderColor: msg.tone === "error" ? "var(--tf-color-error)" : "var(--tf-color-success)", color: msg.tone === "error" ? "var(--tf-color-error)" : "var(--tf-color-success)" }}
          >
            {msg.text}
          </p>
        )}
        {problems.length > 1 && (
          <ul className="m-0 list-disc pl-6 text-sm text-[var(--tf-color-error)]">
            {problems.map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ul>
        )}

        <Section
          id="providers"
          title="Cloud providers"
          hint="The providers this project builds on, and the version of each. The raw resources offered are the ones that fit; a new folder gets a providers.tf with required_providers. Leave it empty to see every resource."
        >
          {registry.length === 0 ? (
            <p className="m-0 text-sm text-[var(--tf-color-text-muted)]">No providers are registered yet. An admin adds them under Administration, Cloud providers.</p>
          ) : (
            <ul className="m-0 grid list-none gap-3 p-0 sm:grid-cols-2">
              {registry.map((p) => {
                const chosen = (draft.providers ?? []).find((x) => x.id === p.id);
                const setChosen = (next: { id: string; version?: string } | null) =>
                  set({ providers: [...(draft.providers ?? []).filter((x) => x.id !== p.id), ...(next ? [next] : [])] });
                return (
                  <li key={p.id} className={`rounded-xl border p-3 ${chosen ? "border-[var(--tf-color-action)] bg-[var(--tf-glass-strong)]" : "border-[var(--tf-color-border)]"}`}>
                    <label className="flex cursor-pointer items-center gap-3">
                      <input type="checkbox" disabled={!canEdit} checked={!!chosen} onChange={(e) => setChosen(e.target.checked ? { id: p.id, version: p.versions[0] } : null)} />
                      <ProviderLogo provider={p} size={28} />
                      <span className="min-w-0">
                        <span className="block text-sm font-semibold">{p.name}</span>
                        <span className="block font-mono text-xs text-[var(--tf-color-text-muted)]">{p.source}</span>
                      </span>
                    </label>
                    {chosen && (
                      <div className="mt-2">
                        <input
                          disabled={!canEdit}
                          aria-label={`Version of ${p.name}`}
                          list={`versions-${p.id}`}
                          className={`${field} mt-0 font-mono text-xs`}
                          placeholder="~> 4.0"
                          value={chosen.version ?? ""}
                          onChange={(e) => setChosen({ id: p.id, version: e.target.value.trim() })}
                        />
                        <datalist id={`versions-${p.id}`}>
                          {p.versions.map((v) => (
                            <option key={v} value={v} />
                          ))}
                        </datalist>
                      </div>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </Section>

        <Section
          id="layout"
          title="Where new resources go"
          hint="A project's folders are one level deep: <project folder>/<folder>/<file>.tf. This is the default for a resource when it is dropped on the canvas; a person can still change it on the resource."
        >
          <fieldset disabled={!canEdit} className="m-0 grid gap-3 border-0 p-0 sm:grid-cols-2">
            {(
              [
                ["catalog", "Each kind in its own folder", "Every module uses the folder and file its schema asks for (common/rg.tf, network/vnet.tf). The usual layout for a team."],
                ["single", "Everything in one place", "All new resources go to one folder and one file, for example terraform/main.tf. Good for a small project."],
              ] as const
            ).map(([mode, title, text]) => {
              const on = (layout.mode ?? "catalog") === mode;
              return (
                <label key={mode} className={`flex cursor-pointer gap-3 rounded-xl border p-3 ${on ? "border-[var(--tf-color-action)] bg-[var(--tf-glass-strong)]" : "border-[var(--tf-color-border)]"}`}>
                  <input type="radio" name="layout-mode" checked={on} onChange={() => set({ layout: { ...layout, mode } })} className="mt-1" />
                  <span>
                    <span className="block text-sm font-semibold">{title}</span>
                    <span className="text-xs text-[var(--tf-color-text-secondary)]">{text}</span>
                  </span>
                </label>
              );
            })}
          </fieldset>

          {(layout.mode ?? "catalog") === "single" ? (
            <div className="mt-4 grid max-w-md gap-3 sm:grid-cols-2">
              <label className="text-sm font-medium">
                Folder
                <input disabled={!canEdit} className={`${field} font-mono`} value={layout.folder ?? ""} placeholder={DEFAULT_SINGLE_FOLDER} onChange={(e) => set({ layout: { ...layout, folder: e.target.value.trim() } })} />
              </label>
              <label className="text-sm font-medium">
                File
                <input disabled={!canEdit} className={`${field} font-mono`} value={layout.file ?? ""} placeholder={DEFAULT_SINGLE_FILE} onChange={(e) => set({ layout: { ...layout, file: e.target.value.trim() } })} />
              </label>
            </div>
          ) : (
            modules.length > 0 && (
              <details className="mt-4">
                <summary className="cursor-pointer text-sm font-medium">File name for each kind of resource (optional)</summary>
                <p className="m-0 mt-2 text-xs text-[var(--tf-color-text-muted)]">For example resource-group → rg.tf. Empty keeps the module's own file name.</p>
                <ul className="m-0 mt-2 grid list-none gap-2 p-0 sm:grid-cols-2">
                  {modules.map((m) => (
                    <li key={m.name} className="flex items-center gap-2">
                      <span className="w-44 shrink-0 truncate text-xs" title={m.name}>
                        {m.title || m.name}
                      </span>
                      <input
                        disabled={!canEdit}
                        aria-label={`File for ${m.name}`}
                        className={`${field} mt-0 py-1.5 font-mono text-xs`}
                        placeholder="default"
                        value={layout.files?.[m.name] ?? ""}
                        onChange={(e) => {
                          const files = { ...layout.files };
                          if (e.target.value.trim()) files[m.name] = e.target.value.trim();
                          else delete files[m.name];
                          set({ layout: { ...layout, files } });
                        }}
                      />
                    </li>
                  ))}
                </ul>
              </details>
            )
          )}
        </Section>

        <Section
          id="variables"
          title="Variables"
          hint="Declare a value once (org = aic). In any text field type {org} and the generated code uses var.org; TerraFlow adds the variable to variables.tf and the value to env/<env>.tfvars, without changing lines that are already there."
        >
          {vars.length === 0 ? (
            <p className="m-0 mb-3 text-sm text-[var(--tf-color-text-muted)]">No variables yet. Values are written as typed, as before.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[560px] text-left text-sm">
                <thead className="text-xs uppercase text-[var(--tf-color-text-muted)]">
                  <tr>
                    <th className="py-1.5 pr-2">Name</th>
                    <th className="pr-2">Description</th>
                    {envs.map((e) => (
                      <th key={e} className="pr-2">
                        {e}
                      </th>
                    ))}
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {vars.map((v, i) => (
                    <tr key={i} className="border-t border-[var(--tf-color-border)] align-top">
                      <td className="py-2 pr-2">
                        <input disabled={!canEdit} aria-label="Variable name" className={`${field} mt-0 w-32 font-mono text-xs ${v.name && !VAR_NAME.test(v.name) ? "border-[var(--tf-color-error)]" : ""}`} value={v.name} placeholder="org" onChange={(e) => setVar(i, { ...v, name: e.target.value })} />
                      </td>
                      <td className="pr-2">
                        <input disabled={!canEdit} aria-label="Description" className={`${field} mt-0 text-xs`} value={v.description ?? ""} placeholder="What it is" onChange={(e) => setVar(i, { ...v, description: e.target.value })} />
                      </td>
                      {envs.map((e) => (
                        <td key={e} className="pr-2">
                          <input
                            disabled={!canEdit}
                            aria-label={`${v.name || "variable"} in ${e}`}
                            className={`${field} mt-0 w-32 font-mono text-xs`}
                            value={v.values?.[e] ?? ""}
                            onChange={(ev) => {
                              const values = { ...v.values };
                              if (ev.target.value === "") delete values[e];
                              else values[e] = ev.target.value;
                              setVar(i, { ...v, values });
                            }}
                          />
                        </td>
                      ))}
                      <td className="py-2 text-right">
                        {canEdit && (
                          <button aria-label={`Remove ${v.name || "variable"}`} className="rounded-md p-1.5 text-[var(--tf-color-text-muted)] hover:bg-[var(--tf-color-error-surface)] hover:text-[var(--tf-color-error)]" onClick={() => set({ variables: vars.filter((_, j) => j !== i) })}>
                            <Trash2 className="size-4" />
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {canEdit && (
            <div className="mt-3 flex flex-wrap gap-2">
              <button className="tf-btn tf-btn--ghost" onClick={() => set({ variables: [...vars, { name: "", values: {} }] })}>
                <Plus className="size-4" /> Add variable
              </button>
              <button className="tf-btn tf-btn--ghost" onClick={addEnv}>
                <Plus className="size-4" /> Add environment
              </button>
            </div>
          )}
          <p className="m-0 mt-3 text-xs text-[var(--tf-color-text-muted)]">
            A variable with no value for an environment gets no line in that environment's file, so Terraform asks for it there. Values are plain text in git: do not put secrets here.
          </p>
        </Section>

        <Section
          id="starters"
          title="Starter files"
          hint="Files written to a folder the first time it gets a resource, only if the folder does not have them yet. {folder} in the text becomes the folder name (useful for a state key)."
        >
          {starters.length === 0 && <p className="m-0 mb-3 text-sm text-[var(--tf-color-text-muted)]">None. Folders get only the resources people add.</p>}
          <ul className="m-0 list-none space-y-3 p-0">
            {starters.map((f, i) => (
              <li key={i} className="rounded-lg border border-[var(--tf-color-border)] p-3">
                <div className="mb-2 flex items-center justify-between gap-2">
                  <input disabled={!canEdit} aria-label="Starter file name" className={`${field} mt-0 max-w-[14rem] font-mono text-xs`} value={f.filename} onChange={(e) => set({ starters: starters.map((x, j) => (j === i ? { ...x, filename: e.target.value.trim() } : x)) })} />
                  {canEdit && (
                    <button aria-label={`Remove ${f.filename}`} className="rounded-md p-1.5 text-[var(--tf-color-text-muted)] hover:bg-[var(--tf-color-error-surface)] hover:text-[var(--tf-color-error)]" onClick={() => set({ starters: starters.filter((_, j) => j !== i) })}>
                      <Trash2 className="size-4" />
                    </button>
                  )}
                </div>
                <textarea disabled={!canEdit} aria-label={`Content of ${f.filename}`} rows={7} spellCheck={false} className={`${field} mt-0 font-mono text-xs`} value={f.content} onChange={(e) => set({ starters: starters.map((x, j) => (j === i ? { ...x, content: e.target.value } : x)) })} />
              </li>
            ))}
          </ul>
          {canEdit && (
            <div className="mt-3 flex flex-wrap gap-2">
              {STARTER_PRESETS.filter((p) => !starters.some((s) => s.filename === p.file.filename)).map((p) => (
                <button key={p.label} className="tf-btn tf-btn--ghost" onClick={() => set({ starters: [...starters, p.file] })}>
                  <Plus className="size-4" /> {p.label}
                </button>
              ))}
              <button className="tf-btn tf-btn--ghost" onClick={() => set({ starters: [...starters, { filename: "versions.tf", content: "" }] })}>
                <Plus className="size-4" /> Empty file
              </button>
            </div>
          )}
        </Section>
      </div>
    </div>
  );
}
