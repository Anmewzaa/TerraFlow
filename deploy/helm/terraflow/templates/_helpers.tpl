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

{{/* The key that signs session cookies: the value, else the one already in the cluster, else a new one (so sessions survive upgrades). */}}
{{- define "terraflow.sessionKey" -}}
{{- if .Values.sessionKey -}}
{{- .Values.sessionKey -}}
{{- else -}}
{{- $existing := lookup "v1" "Secret" .Release.Namespace (printf "%s-app" .Release.Name) -}}
{{- if and $existing (index $existing.data "session-key") -}}{{ index $existing.data "session-key" | b64dec }}{{- else -}}{{ randAlphaNum 48 }}{{- end -}}
{{- end -}}
{{- end -}}
