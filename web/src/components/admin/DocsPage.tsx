import { useState, type ReactNode } from "react";
import { ArrowLeft, Check, Copy } from "lucide-react";
import { EXAMPLE_SCHEMA, RAW_RESOURCE_FIELDS, SCHEMA_FIELDS, UNSUPPORTED_KEYWORDS, WIDGETS, type FieldDoc, type HelpTopic } from "./helpContent";

/* The documentation of the admin area, as a page of its own: a list of topics on the left, short
 * sections with their own links on the right, "common" before "everything", and every code sample
 * with a Copy button. The text is kept in step with docs/12-schema-reference.md. */

export const TOPICS: { id: HelpTopic; label: string; summary: string }[] = [
  { id: "modules", label: "Module schemas", summary: "Describe a module once; people get a form." },
  { id: "resources", label: "Raw resources", summary: "Plain provider blocks for one-off needs." },
  { id: "providers", label: "Cloud providers", summary: "Clouds, versions and logos." },
  { id: "generation", label: "Variables and files", summary: "How the generated Terraform is laid out." },
  { id: "projects", label: "Projects", summary: "Owners, repositories and review." },
  { id: "people", label: "People & access", summary: "Sign-ups and GitHub connections." },
];

function Copy_({ text }: { text: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      className="absolute right-2 top-2 flex items-center gap-1 rounded-md border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] px-2 py-1 text-[11px] hover:border-[var(--tf-color-action)]"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          setDone(true);
          setTimeout(() => setDone(false), 1500);
        } catch {
          /* clipboard not available: the text is selectable */
        }
      }}
    >
      {done ? <Check className="size-3" /> : <Copy className="size-3" />} {done ? "Copied" : "Copy"}
    </button>
  );
}

function Code({ children, label }: { children: string; label?: string }) {
  return (
    <figure className="relative m-0 my-3">
      {label && <figcaption className="mb-1 text-xs font-medium text-[var(--tf-color-text-muted)]">{label}</figcaption>}
      <pre className="m-0 max-h-80 overflow-auto rounded-lg border border-[var(--tf-color-border)] bg-[var(--tf-color-canvas)] p-3 pr-16 text-[12px] leading-relaxed">{children}</pre>
      <Copy_ text={children} />
    </figure>
  );
}

function Fields({ rows }: { rows: FieldDoc[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[480px] text-left text-sm">
        <tbody>
          {rows.map((r) => (
            <tr key={r.key} className="border-t border-[var(--tf-color-border)] align-top">
              <td className="w-56 py-2 pr-3 font-mono text-xs">{r.key}</td>
              <td className="py-2 text-[var(--tf-color-text-secondary)]">{r.meaning}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** The keys that matter first (the required ones), then every key behind a disclosure. */
function Reference({ rows }: { rows: FieldDoc[] }) {
  const common = rows.filter((r) => r.required);
  return (
    <>
      <h4 className="mb-1 mt-4 text-sm font-semibold">You always need</h4>
      <Fields rows={common} />
      <details className="mt-4">
        <summary className="cursor-pointer text-sm font-medium">Every key ({rows.length})</summary>
        <div className="mt-2">
          <Fields rows={rows} />
        </div>
      </details>
    </>
  );
}

function Steps({ items }: { items: string[] }) {
  return (
    <ol className="m-0 list-none space-y-2 p-0">
      {items.map((s, i) => (
        <li key={s} className="flex gap-3">
          <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-[var(--tf-color-action)] text-xs font-bold text-[var(--tf-color-on-action)]">{i + 1}</span>
          <span className="pt-0.5 text-sm">{s}</span>
        </li>
      ))}
    </ol>
  );
}

function Quick({ children }: { children: ReactNode }) {
  return (
    <div className="my-4 rounded-xl border border-[var(--tf-color-action)]/40 bg-[var(--tf-glass-strong)] p-4 text-sm">
      <p className="m-0 mb-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-action)]">In one minute</p>
      {children}
    </div>
  );
}

interface Section {
  id: string;
  title: string;
  body: ReactNode;
}

const GENERATED_SAMPLE = `module "rg_main" {
  source = "<the module's location in your repository>"

  location                 = "southeastasia"
  name                     = "rg-aic-dev-app"

  tags = {
    owner                    = "platform"
  }
}`;

function sections(topic: HelpTopic, onUseExample?: () => void): Section[] {
  switch (topic) {
    case "modules":
      return [
        {
          id: "quick",
          title: "Your first module",
          body: (
            <>
              <Quick>
                A module schema is a JSON file that lists a Terraform module's inputs. TerraFlow draws the form from it and writes the module call, so adding a module needs
                no code change.
              </Quick>
              <Steps items={["Press “Add a module” and choose Start from an example: it already passes the check.", "Change the title, the inputs and the x-tf-module block (source, folder, file).", "Press Check. Every problem is listed with where it is; fix and check again.", "Press Import and publish. A published version never changes: publish v2 instead."]} />
              {onUseExample && (
                <button type="button" className="tf-btn mt-4" onClick={onUseExample}>
                  Open the example in the editor
                </button>
              )}
            </>
          ),
        },
        {
          id: "example",
          title: "Schema, form and code",
          body: (
            <>
              <p className="m-0 text-sm text-[var(--tf-color-text-secondary)]">The example schema on the left becomes a form with a Name, a Location, a Management lock and Tags, and, for the values typed in the form, a module call like the one below.</p>
              <Code label="Schema (the example)">{JSON.stringify(EXAMPLE_SCHEMA, null, 2)}</Code>
              <Code label="Module call the generator writes (values are whatever was typed in the form)">{GENERATED_SAMPLE}</Code>
              <p className="m-0 text-xs text-[var(--tf-color-text-muted)]">Attributes are written sorted by name, with tags last. The source line comes from where your module repository is.</p>
            </>
          ),
        },
        { id: "keys", title: "Keys", body: <Reference rows={SCHEMA_FIELDS} /> },
        {
          id: "widgets",
          title: "Widgets",
          body: (
            <ul className="m-0 list-none space-y-1.5 p-0 text-sm">
              {WIDGETS.map((w) => (
                <li key={w.id}>
                  <code className="rounded bg-[var(--tf-glass-strong)] px-1">{w.id}</code> {w.use}
                </li>
              ))}
            </ul>
          ),
        },
        {
          id: "limits",
          title: "What is not supported",
          body: (
            <p className="m-0 text-sm text-[var(--tf-color-text-secondary)]">
              The form cannot draw these JSON Schema keywords, so the check rejects them:{" "}
              {UNSUPPORTED_KEYWORDS.map((k) => (
                <code key={k} className="mr-1 rounded bg-[var(--tf-glass-strong)] px-1">{k}</code>
              ))}
            </p>
          ),
        },
      ];
    case "resources":
      return [
        {
          id: "quick",
          title: "Module or raw resource?",
          body: (
            <>
              <Quick>Prefer a module: it has a schema, so the form checks the values. A raw resource is a plain provider block for the one-off things no module covers.</Quick>
              <Steps items={["Press “Add raw resource”, or pick a common one to start from.", "Name it (lower case) and give it a title people will recognise.", "Type the exact resource type, for example azurerm_resource_group. Its provider is read from it.", "Pick a category, and the provider versions it exists in if it is not in all of them.", "Save. It appears in the palette of every project whose provider and version fit."]} />
            </>
          ),
        },
        { id: "fields", title: "Fields", body: <Reference rows={RAW_RESOURCE_FIELDS} /> },
      ];
    case "providers":
      return [
        {
          id: "quick",
          title: "What a provider is here",
          body: (
            <>
              <Quick>A provider (Azure, AWS, Google…) is registered once with its Terraform source, the versions teams may pick and a logo. TerraFlow never contacts the cloud.</Quick>
              <Steps items={["Press “Add provider”.", "Give an id (the prefix of its resource types, such as aws), a name and the source (hashicorp/aws).", "List the versions to offer, newest first (~> 5.0, ~> 4.0).", "Upload a logo (SVG or PNG, at most 40 KB) and, if needed, the provider block.", "Give its raw resources a version range."]} />
            </>
          ),
        },
        {
          id: "use",
          title: "How projects use it",
          body: (
            <ul className="m-0 list-disc space-y-1.5 pl-5 text-sm">
              <li>A project chooses its providers and a version for each (project Settings, or Advanced when creating it).</li>
              <li>Its palette then shows only the raw resources of those providers whose version range contains the chosen version, and says how many were hidden.</li>
              <li>A new folder gets a <code>providers.tf</code> with required_providers and the provider block written here.</li>
              <li>This is a help for choosing: TerraFlow does not read the provider's own schema, so it cannot promise a resource exists in a version.</li>
            </ul>
          ),
        },
      ];
    case "generation":
      return [
        {
          id: "quick",
          title: "What the settings do",
          body: (
            <>
              <Quick>People keep dragging and dropping. The project's settings decide the defaults, the variables, and the extra files that appear next to the generated code.</Quick>
              <ul className="m-0 list-disc space-y-1.5 pl-5 text-sm">
                <li><strong>Layout</strong>: where a new resource goes by default, each kind in its own folder or everything in one file. A person can still change it on the resource.</li>
                <li><strong>Variables</strong>: declare <code>org = aic</code> once. In any text field type <code>{"{org}"}</code>, and the generated code uses <code>var.org</code>.</li>
                <li><strong>Starter files</strong>: files written to a folder the first time it gets a resource (a backend.tf), and providers.tf from the chosen providers.</li>
              </ul>
            </>
          ),
        },
        {
          id: "variables",
          title: "Variables in detail",
          body: (
            <>
              <p className="m-0 text-sm text-[var(--tf-color-text-secondary)]">Typing {"{org}"} inside a longer text works too. TerraFlow adds the variable to <code>variables.tf</code> and the value to <code>env/&lt;env&gt;.tfvars</code>, and never changes a line that is already there.</p>
              <Code label="In the form">{"rg-{org}-app"}</Code>
              <Code label="In the generated code">{'name = "rg-${var.org}-app"'}</Code>
              <Code label="variables.tf (added if missing)">{'variable "org" {\n  type        = string\n  description = "Organisation"\n}'}</Code>
              <Code label="env/dev.tfvars (added if missing)">{'org = "aic"'}</Code>
              <p className="m-0 text-xs text-[var(--tf-color-text-muted)]">A variable with no value for an environment gets no line there, so Terraform asks for it. Values are plain text in git: do not put secrets in them. When a value you type contains a variable's value (aic), the form offers to use the token; it never replaces it by itself.</p>
            </>
          ),
        },
        {
          id: "layout",
          title: "Layout and folders",
          body: (
            <p className="m-0 text-sm text-[var(--tf-color-text-secondary)]">
              A project's folders are one level deep: <code>&lt;project folder&gt;/&lt;folder&gt;/&lt;file&gt;.tf</code>. For <code>terraform/main.tf</code>, make the project folder the repository root and use the single folder <code>terraform</code>. Files placed directly in the project folder are not part of the contract.
            </p>
          ),
        },
      ];
    case "projects":
      return [
        {
          id: "quick",
          title: "What you see",
          body: (
            <>
              <Quick>Every project with its owner, repository, connection state, members and the changes waiting for review. You see this without opening any project's content.</Quick>
              <ul className="m-0 list-disc space-y-1.5 pl-5 text-sm">
                <li>Rename a project or change its folder.</li>
                <li>Assign people to a project.</li>
                <li>Delete a project from TerraFlow. The repository files are never touched.</li>
              </ul>
            </>
          ),
        },
        {
          id: "roles",
          title: "Roles in a project",
          body: (
            <ul className="m-0 list-disc space-y-1.5 pl-5 text-sm">
              <li><strong>Owner</strong>: everything, including approve, invite, delete, transfer.</li>
              <li><strong>Co-owner</strong>: approve, invite, set roles and change settings.</li>
              <li><strong>Contributor</strong>: build, save and publish, but not approve.</li>
              <li>An admin who is not a member may look, and every such access is written to the audit log as support access.</li>
            </ul>
          ),
        },
      ];
    case "people":
      return [
        {
          id: "quick",
          title: "Sign-ups",
          body: (
            <>
              <Quick>A person who signs in for the first time waits for approval and sees only a waiting page until you approve them. Suspending ends their session at once.</Quick>
              <p className="m-0 text-sm text-[var(--tf-color-text-secondary)]">Approve, reject with a reason, suspend and reactivate in People & access. Everything is recorded in the audit log.</p>
            </>
          ),
        },
        {
          id: "github",
          title: "GitHub connections",
          body: (
            <p className="m-0 text-sm text-[var(--tf-color-text-secondary)]">
              A connection is one installation of the TerraFlow GitHub App on an account or organisation. It holds no password or token. <strong>Check</strong> asks GitHub whether the installation still exists; <strong>Disconnect</strong> stops every project that uses it until the App is installed again.
            </p>
          ),
        },
        {
          id: "admins",
          title: "Making someone an admin",
          body: (
            <p className="m-0 text-sm text-[var(--tf-color-text-secondary)]">
              Admins are named in the server settings (<code>TF_ADMIN_USERS</code>) by <code>github:&lt;id&gt;</code>, an Entra object id or an e-mail. They are never blocked by the approval list.
            </p>
          ),
        },
      ];
  }
}

interface Props {
  topic: HelpTopic;
  onTopic: (t: HelpTopic) => void;
  onBack: () => void;
  onUseExample?: () => void;
}

/** The documentation page: topics on the left, short sections with their own links on the right. */
export function DocsPage({ topic, onTopic, onBack, onUseExample }: Props) {
  const list = sections(topic, topic === "modules" ? onUseExample : undefined);
  const current = TOPICS.find((t) => t.id === topic)!;
  return (
    <div>
      <button onClick={onBack} className="mb-4 flex items-center gap-1.5 text-sm text-[var(--tf-color-text-secondary)] hover:text-[var(--tf-color-text)]">
        <ArrowLeft className="size-4" /> Back
      </button>
      <div className="grid gap-8 xl:grid-cols-[14rem_minmax(0,1fr)_11rem]">
        <nav aria-label="Documentation topics" className="xl:sticky xl:top-0 xl:self-start">
          <p className="m-0 mb-2 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">Documentation</p>
          <ul className="m-0 flex list-none gap-1 overflow-x-auto p-0 xl:block">
            {TOPICS.map((t) => (
              <li key={t.id}>
                <button
                  onClick={() => onTopic(t.id)}
                  aria-current={t.id === topic ? "page" : undefined}
                  className={`w-full whitespace-nowrap rounded-md px-2.5 py-2 text-left text-sm ${t.id === topic ? "bg-[var(--tf-glass-strong)] font-semibold" : "text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-hover)]"}`}
                >
                  {t.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>

        <article className="min-w-0 max-w-3xl">
          <h1 className="m-0 text-3xl font-bold tracking-tight">{current.label}</h1>
          <p className="m-0 mt-1 text-[var(--tf-color-text-secondary)]">{current.summary}</p>
          {list.map((s) => (
            <section key={s.id} id={`doc-${s.id}`} className="scroll-mt-4 border-t border-[var(--tf-color-border)] pt-6 first:mt-6 mt-8">
              <h2 className="m-0 mb-3 text-xl font-semibold">{s.title}</h2>
              {s.body}
            </section>
          ))}
        </article>

        <aside aria-label="On this page" className="hidden xl:sticky xl:top-0 xl:block xl:self-start">
          <p className="m-0 mb-2 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">On this page</p>
          <ul className="m-0 list-none space-y-1 p-0 text-sm">
            {list.map((s) => (
              <li key={s.id}>
                <a href={`#doc-${s.id}`} onClick={(e) => { e.preventDefault(); document.getElementById(`doc-${s.id}`)?.scrollIntoView({ behavior: "smooth" }); }} className="text-[var(--tf-color-text-secondary)] hover:text-[var(--tf-color-text)]">
                  {s.title}
                </a>
              </li>
            ))}
          </ul>
        </aside>
      </div>
    </div>
  );
}
