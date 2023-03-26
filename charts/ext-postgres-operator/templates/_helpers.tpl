{{/*
Expand the name of the chart.
*/}}
{{- define "chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "chart.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "chart.labels" -}}
helm.sh/chart: {{ include "chart.chart" . }}
{{ include "chart.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "chart.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "chart.selectorLabelsDev" -}}
app.kubernetes.io/name: {{ include "chart.name" . }}-dev
app.kubernetes.io/instance: {{ .Release.Name }}-dev
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "chart.serviceAccountName" -}}
{{- default (include "chart.fullname" .) .Values.serviceAccount.name }}
{{- end }}

{{/*
Generate list of env vars from dics of env vars
*/}}
{{- define "envVarsMap" -}}
{{- $map := . -}}
{{- range $key := $map | keys | sortAlpha -}}
{{- $val := get $map $key }}
- name: {{ $key }}
{{- if or (kindIs "map" $val) (kindIs "slice" $val) }}
{{ $val | toYaml | indent 2 }}
{{- else }}
  value: {{ $val | quote}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Gets a dict of env vars and return list of name,value dicts
*/}}
{{- define "utils.sortedEnvVars" -}}
{{- $dict := . -}}
{{- range $key := $dict | keys | sortAlpha -}}
{{- $val := get $dict $key }}
- name: {{ $key }}
{{- if or (kindIs "map" $val) (kindIs "slice" $val) }}
{{ $val | toYaml | indent 2 }}
{{- else }}
  value: {{ $val | quote}}
{{- end -}}
{{- end -}}
{{- end -}}