{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "modelmesh.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "modelmesh.labels" -}}
helm.sh/chart: {{ include "modelmesh.chart" . }}
{{ include "modelmesh.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/instance: modelmesh-controller
app.kubernetes.io/managed-by: modelmesh-controller
app.kubernetes.io/name: modelmesh-controller
{{- end }}

{{/*
Selector labels
*/}}
{{- define "modelmesh.selectorLabels" -}}
control-plane: modelmesh-controller
{{- end }}

{{- define "webhook.caBundle" -}}
{{- if .Values.modelmesh.certmanager.enabled }}
  Cg==
{{- else }}
  {{- $namespace := .Release.Namespace -}}
  {{- $caSecret := lookup "v1" "Secret" $namespace .Values.modelmesh.config.selfsignedSecretName -}}
  {{- if $caSecret -}}
    {{- index $caSecret.data "ca.crt" | b64dec -}}
  {{- end -}}
{{- end -}}
{{- end -}}


