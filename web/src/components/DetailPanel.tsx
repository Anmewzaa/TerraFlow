import { useState } from "react";
import type { GraphNode, GraphExternal } from "../types/graph";
import type { CanvasNode } from "../types/changeset";
import { ModuleForm } from "./catalog/ModuleForm";
import type { ModuleSummary } from "../types/catalog";
import { BLOCK_COLORS, colorValue, resolveParents, hintMap } from "../lib/canvasLayout";
import { nodeIssues, suppliedFields } from "../lib/validate";
import type { Finding } from "../lib/findings";
import { serverIssueTexts } from "../hooks/useValidation";
import { FileTargetFields } from "./FileTargetFields";
import { RawResourceForm } from "./RawResourceForm";
import { isExpr, valueText } from "../lib/valueText";
import { resolveCatalogVersion } from "../lib/importExisting";
import { domainCssVars, getAzureIcon, moduleDomainMap } from "./nodes/domainStyles";
import { ResourceGlyph } from "./nodes/ResourceGlyph";

function isGraphNode(n: GraphNode | GraphExternal): n is GraphNode {
  return "kind" in n && "folder" in n;
}

/* --- Read-only detail panel for existing nodes --- */
interface DetailPanelProps {
  node: GraphNode | GraphExternal;
  onClose: () => void;
  /** Turns this repository resource into an editable node on the canvas. */
  onEdit?: (node: GraphNode) => void;
}

export function DetailPanel({ node, onClose, onEdit }: DetailPanelProps) {
  const [activeTab, setActiveTab] = useState("config");
  const title = isGraphNode(node) ? formatTitle(node.type) : node.kind;
  const subtitle = isGraphNode(node)
    ? `${node.type} / ${node.source ? extractVersion(node.source) ?? "" : ""}`
    : `external / ${node.source}`;
  const domain = isGraphNode(node) ? (moduleDomainMap[node.type] ?? "neutral") : "neutral";
  const color = domainCssVars[domain] ?? domainCssVars.neutral;
  const iconSrc = isGraphNode(node) ? getAzureIcon(node.type) : null;

  return (
    <aside className="tf-detail-panel">
      <PanelHeader
        title={title}
        subtitle={subtitle}
        iconSrc={iconSrc}
        accentColor={color}
          domain={domain}
        onClose={onClose}
      />

      <div className="tf-detail-panel__tabs">
        {["config", "deps", "hcl"].map((tab) => (
          <button
            key={tab}
            className={`tf-detail-panel__tab${activeTab === tab ? " tf-detail-panel__tab--active" : ""}`}
            onClick={() => setActiveTab(tab)}
          >
            {tab === "config" ? "Configuration" : tab === "deps" ? "Dependencies" : "HCL"}
          </button>
        ))}
      </div>

      <div className="tf-detail-panel__body">
        {onEdit && isGraphNode(node) && node.kind === "module" && (
          <button className="tf-btn" style={{ marginBottom: "var(--tf-space-3)" }} onClick={() => onEdit(node)}>
            Edit this resource
          </button>
        )}
        {activeTab === "config" && (
          isGraphNode(node)
            ? <ConfigTab node={node} />
            : <ExternalConfigTab ext={node} />
        )}
        {activeTab === "deps" && (
          <div style={{ color: "var(--tf-color-text-muted)", fontSize: 13 }}>
            Dependencies view coming in Phase 5.
          </div>
        )}
        {activeTab === "hcl" && (
          <div style={{ color: "var(--tf-color-text-muted)", fontSize: 13 }}>
            HCL source view coming in Phase 5.
          </div>
        )}
      </div>
    </aside>
  );
}

/* --- Unified panel for draft nodes (edit form) or existing nodes (info) --- */
interface DraftOrDetailPanelProps {
  editingDraftNode?: CanvasNode;
  selectedExistingNode?: GraphNode | GraphExternal;
  projectId: string | null;
  onClose: () => void;
  onRemoveNode: (id: string) => void;
  onUpdateNode: (id: string, updates: Partial<CanvasNode>) => void;
  onEditExisting?: (node: GraphNode) => void;
  /** Folders of the project, for placing a raw resource. */
  folders?: string[];
  /** All draft nodes and catalog hints: used for the colour picker and relationships. */
  allNodes?: CanvasNode[];
  modules?: ModuleSummary[];
  /** What the server says about the whole canvas (D-065); this panel shows the part about its node. */
  findings?: Finding[];
}

export function DraftOrDetailPanel({
  editingDraftNode,
  selectedExistingNode,
  projectId,
  onClose,
  onRemoveNode,
  onUpdateNode,
  onEditExisting,
  folders = [],
  allNodes = [],
  modules = [],
  findings = [],
}: DraftOrDetailPanelProps) {
  const [activeTab, setActiveTab] = useState("config");

  if (editingDraftNode?.kind === "resource") {
    return (
      <aside className="tf-detail-panel">
        <PanelHeader
          title={editingDraftNode.resource_type || "Raw resource"}
          subtitle="raw provider resource"
          iconSrc={null}
          accentColor={domainCssVars.neutral}
          isDraft
          onClose={onClose}
          onRemove={() => onRemoveNode(editingDraftNode.id)}
        />
        <div className="tf-detail-panel__body">
          <RawResourceForm
            key={editingDraftNode.id}
            node={editingDraftNode}
            folders={folders}
            onChange={(u) => onUpdateNode(editingDraftNode.id, u)}
          />
        </div>
      </aside>
    );
  }

  if (editingDraftNode) {
    const hints = hintMap(modules);
    const canHold = !!hints.get(editingDraftNode.module_name)?.container;
    const parentId = resolveParents(allNodes, hints).get(editingDraftNode.id);
    const parentNode = allNodes.find((n) => n.id === parentId);
    const kids = allNodes.filter((n) => resolveParents(allNodes, hints).get(n.id) === editingDraftNode.id);
    const localIssues = nodeIssues(editingDraftNode, parentId, modules.find((m) => m.name === editingDraftNode.module_name), suppliedFields(allNodes, editingDraftNode.id));
    // The server also checks values against the schema, names and containment; show what it adds.
    const issues = [
      ...localIssues,
      ...serverIssueTexts(findings, editingDraftNode.id, localIssues.map((i) => i.text)).map((text) => ({ text })),
    ];
    const label = (n: CanvasNode) => `${formatTitle(n.module_name)} · ${valueText(n.values?.name) || n.instance_name}`;
    const domain = moduleDomainMap[editingDraftNode.module_name] ?? "neutral";
    const color = domainCssVars[domain] ?? domainCssVars.neutral;
    const iconSrc = getAzureIcon(editingDraftNode.module_name, modules.find((m) => m.name === editingDraftNode.module_name)?.icon);
    const title = formatTitle(editingDraftNode.module_name);
    const subtitle = `${editingDraftNode.module_name} / ${editingDraftNode.module_version}`;

    return (
      <aside className="tf-detail-panel">
        <PanelHeader
          title={title}
          subtitle={subtitle}
          iconSrc={iconSrc}
          accentColor={color}
          domain={domain}
          isDraft
          onClose={onClose}
          onRemove={() => onRemoveNode(editingDraftNode.id)}
        />

        <div className="tf-detail-panel__tabs">
          {["config", "deps"].map((tab) => (
            <button
              key={tab}
              className={`tf-detail-panel__tab${activeTab === tab ? " tf-detail-panel__tab--active" : ""}`}
              onClick={() => setActiveTab(tab)}
            >
              {tab === "config" ? "Configuration" : "Relationships"}
            </button>
          ))}
        </div>

        <div className="tf-detail-panel__body">
          {issues.length > 0 && (
            <div className="tf-issues" role="alert">
              <strong>{issues.length} thing{issues.length === 1 ? "" : "s"} left to configure</strong>
              <ul>
                {issues.map((i, k) => <li key={k}>{i.text}</li>)}
              </ul>
            </div>
          )}
          {activeTab === "config" && !editingDraftNode.nest_field && (
            <FileTargetFields
              key={editingDraftNode.id}
              folder={editingDraftNode.target_folder}
              filename={editingDraftNode.filename}
              onChange={(u) => onUpdateNode(editingDraftNode.id, u)}
            />
          )}
          {activeTab === "config" && canHold && (
            <fieldset className="tf-form-group" style={{ marginBottom: "var(--tf-space-4)" }}>
              <legend>Block colour</legend>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }} role="radiogroup" aria-label="Block colour">
                {BLOCK_COLORS.map((c) => {
                  const on = colorValue(editingDraftNode.color) === c.value;
                  return (
                    <button
                      key={c.key}
                      role="radio"
                      aria-checked={on}
                      aria-label={c.label}
                      title={c.label}
                      onClick={() => onUpdateNode(editingDraftNode.id, { color: c.key })}
                      style={{
                        width: 28, height: 28, borderRadius: 999, cursor: "pointer",
                        background: c.value,
                        border: on ? "2px solid var(--tf-color-text)" : "2px solid transparent",
                        outline: on ? "2px solid var(--tf-color-surface)" : "none",
                        outlineOffset: -4,
                        boxShadow: "var(--tf-shadow-rest)",
                      }}
                    />
                  );
                })}
              </div>
            </fieldset>
          )}
          {activeTab === "config" && (
            <ModuleForm
              moduleName={editingDraftNode.module_name}
              moduleVersion={resolveCatalogVersion(editingDraftNode.module_version, modules.find((m) => m.name === editingDraftNode.module_name))}
              projectId={projectId}
              draftNodeId={editingDraftNode.id}
              initialValues={editingDraftNode.values}
              onValuesChange={(values) => onUpdateNode(editingDraftNode.id, { values })}
            />
          )}
          {activeTab === "deps" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-4)", fontSize: 13 }}>
              <fieldset className="tf-form-group">
                <legend>Placed in</legend>
                {parentNode ? label(parentNode) : <span style={{ color: "var(--tf-color-text-muted)" }}>Top level (not inside another resource)</span>}
              </fieldset>
              <fieldset className="tf-form-group">
                <legend>Contains ({kids.length})</legend>
                {kids.length === 0 ? (
                  <span style={{ color: "var(--tf-color-text-muted)" }}>Nothing inside this resource.</span>
                ) : (
                  <ul style={{ margin: 0, paddingLeft: 18, display: "grid", gap: 4 }}>
                    {kids.map((k) => <li key={k.id}>{label(k)}</li>)}
                  </ul>
                )}
              </fieldset>
            </div>
          )}
        </div>
      </aside>
    );
  }

  if (selectedExistingNode) {
    return <DetailPanel node={selectedExistingNode} onClose={onClose} onEdit={onEditExisting} />;
  }

  return null;
}

/* --- Shared panel header --- */
function PanelHeader({
  title,
  subtitle,
  iconSrc,
  accentColor,
  domain,
  isDraft,
  onClose,
  onRemove,
}: {
  title: string;
  subtitle: string;
  iconSrc: string | null;
  accentColor: string;
  domain?: string;
  isDraft?: boolean;
  onClose: () => void;
  onRemove?: () => void;
}) {
  return (
    <div className="tf-detail-panel__header">
      <div style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-2)", flex: 1, minWidth: 0 }}>
        <div style={{
          width: 32, height: 32, borderRadius: "var(--tf-radius-control)",
          background: `color-mix(in srgb, ${accentColor} 15%, transparent)`,
          display: "flex", alignItems: "center", justifyContent: "center",
          flexShrink: 0,
        }}>
          {iconSrc ? (
            <img src={iconSrc} alt="" width="18" height="18" />
          ) : (
            <span style={{ color: accentColor, display: "flex" }}>
              <ResourceGlyph domain={domain} className="size-4" />
            </span>
          )}
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontWeight: 600, fontSize: 14, display: "flex", alignItems: "center", gap: "var(--tf-space-2)" }}>
            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{title}</span>
            {isDraft && <span className="tf-node__draft-badge">draft</span>}
          </div>
          <div style={{
            fontSize: 11,
            color: "var(--tf-color-text-muted)",
            fontFamily: "var(--tf-font-mono)",
            overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
          }}>
            {subtitle}
          </div>
        </div>
      </div>
      <div style={{ display: "flex", gap: "var(--tf-space-1)", flexShrink: 0 }}>
        {onRemove && (
          <button
            className="tf-btn tf-btn--ghost"
            style={{ height: 24, padding: "0 6px", minHeight: "auto", color: "var(--tf-color-error)" }}
            title="Remove draft"
            onClick={onRemove}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <polyline points="3 6 5 6 21 6" />
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
            </svg>
          </button>
        )}
        <button
          className="tf-btn tf-btn--ghost"
          style={{ height: 24, padding: "0 6px", minHeight: "auto" }}
          title="Close"
          onClick={onClose}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>
    </div>
  );
}

/* --- Shared styles matching SchemaForm --- */
const readonlyInputStyle: React.CSSProperties = {
  width: "100%",
  padding: "var(--tf-space-2) var(--tf-space-3)",
  fontSize: "var(--tf-text-small)",
  border: "1px solid var(--tf-color-border)",
  borderRadius: "var(--tf-radius-control)",
  background: "var(--tf-color-surface-subtle)",
  overflowWrap: "anywhere" as const,
  color: "var(--tf-color-text)",
  boxSizing: "border-box" as const,
  fontFamily: "var(--tf-font-mono)",
};

const labelStyle: React.CSSProperties = {
  display: "block",
  fontWeight: 500,
  fontSize: "var(--tf-text-small)",
  marginBottom: "var(--tf-space-1)",
  color: "var(--tf-color-text)",
};

function ReadonlyFieldset({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <fieldset className="tf-form-group">
      <legend>{label}</legend>
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-4)" }}>
        {children}
      </div>
    </fieldset>
  );
}

function ReadonlyField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label style={labelStyle}>{label}</label>
      {children}
    </div>
  );
}

function ReadonlyValue({ value }: { value: unknown }) {
  if (value && typeof value === "object" && "$unknown" in (value as Record<string, unknown>)) {
    return <div style={{ ...readonlyInputStyle, color: "var(--tf-color-text-muted)", fontStyle: "italic" }}>unknown</div>;
  }
  if (typeof value === "string") {
    return <div style={readonlyInputStyle}>{value}</div>;
  }
  if (typeof value === "boolean") {
    return (
      <label style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-2)", fontSize: "var(--tf-text-small)" }}>
        <input type="checkbox" checked={value} disabled style={{ pointerEvents: "none" }} />
        {String(value)}
      </label>
    );
  }
  if (typeof value === "number") {
    return <div style={readonlyInputStyle}>{String(value)}</div>;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return <div style={{ ...readonlyInputStyle, color: "var(--tf-color-text-muted)", fontStyle: "italic" }}>(empty)</div>;
    }
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {value.map((item, i) => (
          <div key={i} style={readonlyInputStyle}>{typeof item === "object" ? JSON.stringify(item) : String(item)}</div>
        ))}
      </div>
    );
  }
  if (value && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0) {
      return <div style={{ ...readonlyInputStyle, color: "var(--tf-color-text-muted)", fontStyle: "italic" }}>(empty)</div>;
    }
    // One grid for every row, so the keys share a column and the values line up.
    return (
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "minmax(96px, max-content) minmax(0, 1fr)",
          border: "1px solid var(--tf-color-border)",
          borderRadius: "var(--tf-radius-s)",
          overflow: "hidden",
          fontSize: "var(--tf-text-small)",
          fontFamily: "var(--tf-font-mono)",
        }}
      >
        {entries.map(([k, v], i) => {
          const last = i === entries.length - 1;
          const line = last ? "none" : "1px solid var(--tf-color-border)";
          return (
            <div key={k} style={{ display: "contents" }}>
              <span
                style={{
                  padding: "6px 12px",
                  background: "var(--tf-color-surface-subtle)",
                  color: "var(--tf-color-text-secondary)",
                  borderBottom: line,
                  borderRight: "1px solid var(--tf-color-border)",
                  wordBreak: "break-word",
                }}
              >
                {k}
              </span>
              <span style={{ padding: "6px 12px", borderBottom: line, wordBreak: "break-all" }}>
                {isExpr(v) ? v.$expr : typeof v === "object" ? JSON.stringify(v) : String(v ?? "")}
              </span>
            </div>
          );
        })}
      </div>
    );
  }
  return <div style={{ ...readonlyInputStyle, color: "var(--tf-color-text-muted)", fontStyle: "italic" }}>unknown</div>;
}

/* --- Existing node config tab --- */
function ConfigTab({ node }: { node: GraphNode }) {
  const entries = Object.entries(node.inputs);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-4)" }}>
      <ReadonlyFieldset label="Basics">
        <ReadonlyField label="Name">
          {node.nameState === "resolved" ? (
            <div style={readonlyInputStyle}>{node.name}</div>
          ) : (
            <div style={{ ...readonlyInputStyle, color: "var(--tf-color-text-muted)", fontStyle: "italic" }}>unknown</div>
          )}
        </ReadonlyField>
        <ReadonlyField label="Folder">
          <div style={readonlyInputStyle}>{node.folder}/</div>
        </ReadonlyField>
        {node.source && (
          <ReadonlyField label="Source">
            <div style={{ ...readonlyInputStyle, fontSize: 11, wordBreak: "break-all" }}>{node.source}</div>
          </ReadonlyField>
        )}
      </ReadonlyFieldset>

      {entries.length > 0 && (
        <ReadonlyFieldset label="Inputs">
          {entries.map(([key, val]) => (
            <ReadonlyField key={key} label={formatLabel(key)}>
              <ReadonlyValue value={val} />
            </ReadonlyField>
          ))}
        </ReadonlyFieldset>
      )}
    </div>
  );
}

function ExternalConfigTab({ ext }: { ext: GraphExternal }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-4)" }}>
      <ReadonlyFieldset label="Basics">
        <ReadonlyField label="Kind">
          <div style={readonlyInputStyle}>{ext.kind}</div>
        </ReadonlyField>
        <ReadonlyField label="Name">
          <div style={readonlyInputStyle}>{ext.name}</div>
        </ReadonlyField>
        <ReadonlyField label="Source">
          <div style={readonlyInputStyle}>{ext.source}</div>
        </ReadonlyField>
      </ReadonlyFieldset>
    </div>
  );
}

function formatLabel(key: string): string {
  return key.replace(/[-_]/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function formatTitle(type: string): string {
  return type.replace(/[-_]/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function extractVersion(source: string): string | null {
  const m = source.match(/\?ref=([^&]+)/);
  return m ? m[1] : null;
}
