import { useState } from "react";

export const FILENAME_RE = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}\.tf$/;
export const FOLDER_RE = /^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$/;

interface Props {
  folder: string;
  /** Empty/undefined means the generator default (main.tf). */
  filename?: string;
  onChange: (update: { target_folder?: string; filename?: string }) => void;
}

/** Where the generated code goes: folder and file name, both editable. */
export function FileTargetFields({ folder, filename, onChange }: Props) {
  const [f, setF] = useState(folder);
  const [n, setN] = useState(filename ?? "");

  const folderOk = FOLDER_RE.test(f);
  const nameOk = n === "" || FILENAME_RE.test(n);

  return (
    <fieldset className="tf-form-group" style={{ marginBottom: "var(--tf-space-4)" }}>
      <legend>Output file</legend>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--tf-space-3)" }}>
        <div>
          <label htmlFor="tf-out-folder" style={{ display: "block", fontWeight: 600, fontSize: 13, marginBottom: 4 }}>
            Folder
          </label>
          <input
            id="tf-out-folder"
            className="tf-input"
            value={f}
            aria-invalid={!folderOk}
            style={folderOk ? undefined : { borderColor: "var(--tf-color-error)" }}
            onChange={(e) => {
              setF(e.target.value);
              if (FOLDER_RE.test(e.target.value)) onChange({ target_folder: e.target.value });
            }}
          />
        </div>
        <div>
          <label htmlFor="tf-out-file" style={{ display: "block", fontWeight: 600, fontSize: 13, marginBottom: 4 }}>
            File name
          </label>
          <input
            id="tf-out-file"
            className="tf-input"
            value={n}
            placeholder="main.tf"
            aria-invalid={!nameOk}
            style={nameOk ? undefined : { borderColor: "var(--tf-color-error)" }}
            onChange={(e) => {
              setN(e.target.value);
              if (e.target.value === "" || FILENAME_RE.test(e.target.value)) onChange({ filename: e.target.value });
            }}
          />
        </div>
      </div>
      <p
        style={{
          margin: "var(--tf-space-2) 0 0",
          fontSize: 12,
          color: folderOk && nameOk ? "var(--tf-color-text-muted)" : "var(--tf-color-error)",
        }}
      >
        {folderOk && nameOk ? (
          <>
            Written to <code>{folder}/{filename || "main.tf"}</code>. Resources that share a file name end up in the
            same file.
          </>
        ) : !folderOk ? (
          "Folder: letters, digits, - and _ only."
        ) : (
          "File name: letters, digits, - _ . and it must end with .tf"
        )}
      </p>
    </fieldset>
  );
}
