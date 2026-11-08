import { useModuleSchema } from "../../hooks/useModuleSchema";
import { useDraft } from "../../hooks/useDraft";
import { SchemaForm } from "./SchemaForm";

interface Props {
  moduleName: string;
  moduleVersion: string;
  projectId: string | null;
}

export function ModuleForm({ moduleName, moduleVersion, projectId }: Props) {
  const { module, loading, error } = useModuleSchema(moduleName, moduleVersion);
  const { values, save, loaded } = useDraft(projectId, module?.id ?? null);

  if (loading) {
    return (
      <div style={{ padding: "var(--tf-space-4)", color: "var(--tf-color-text-muted)" }}>
        Loading schema…
      </div>
    );
  }

  if (error || !module) {
    return (
      <div style={{ padding: "var(--tf-space-4)", color: "var(--tf-color-bad)" }}>
        {error?.message ?? "Module not found"}
      </div>
    );
  }

  if (!loaded) {
    return (
      <div style={{ padding: "var(--tf-space-4)", color: "var(--tf-color-text-muted)" }}>
        Loading draft…
      </div>
    );
  }

  return (
    <div style={{ padding: "var(--tf-space-4)", overflow: "auto" }}>
      <div style={{ marginBottom: "var(--tf-space-3)" }}>
        <h2 style={{ margin: 0, fontSize: "var(--tf-text-body)", fontWeight: 600 }}>
          {module.title}
        </h2>
        <div style={{ fontSize: "var(--tf-text-small)", color: "var(--tf-color-text-muted)" }}>
          {module.name} / {module.version}
          {module.target_folder && (
            <span style={{ marginLeft: "var(--tf-space-2)" }}>
              → {module.target_folder}/
            </span>
          )}
        </div>
      </div>
      <SchemaForm schema={module.schema} values={values} onChange={save} />
    </div>
  );
}
