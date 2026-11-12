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
The namespace the operator watches: watchNamespace, or the release namespace when its
rights are limited to that namespace.
*/}}
{{- define "chart.watchNamespace" -}}
{{- if .Values.rbac.clusterWide }}
{{- .Values.watchNamespace | default "" }}
{{- else }}
{{- if and .Values.watchNamespace (ne .Values.watchNamespace .Release.Namespace) }}
{{- fail "rbac.clusterWide=false grants rights only in the release namespace: leave watchNamespace empty or set it to the release namespace" }}
{{- end }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
The operator's leader election lease, named as in cmd/main.go.
*/}}
{{- define "chart.leaseName" -}}
{{- if .Values.env.POSTGRES_INSTANCE }}
{{- printf "lock-%s.db.movetokube.com" .Values.env.POSTGRES_INSTANCE }}
{{- else }}
{{- print "lock.db.movetokube.com" }}
{{- end }}
{{- end }}
