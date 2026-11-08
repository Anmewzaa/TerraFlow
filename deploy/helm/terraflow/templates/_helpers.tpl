{{- define "terraflow.pgName" -}}{{ .Release.Name }}-postgres{{- end -}}

{{/* One password for the Secret: the value, else the one already in the cluster, else a new one. */}}
{{- define "terraflow.pgPassword" -}}
{{- if .Values.postgres.password -}}
{{- .Values.postgres.password -}}
{{- else -}}
{{- $existing := lookup "v1" "Secret" .Release.Namespace (include "terraflow.pgName" .) -}}
{{- if $existing -}}{{ index $existing.data "password" | b64dec }}{{- else -}}{{ randAlphaNum 24 }}{{- end -}}
{{- end -}}
{{- end -}}
