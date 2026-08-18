{{/*
Expand the name of the chart.
*/}}
{{- define "ghep.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "ghep.fullname" -}}
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
{{- define "ghep.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ghep.labels" -}}
helm.sh/chart: {{ include "ghep.chart" . }}
{{ include "ghep.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ghep.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ghep.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "ghep.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ghep.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
The container port, derived from config.serverAddr.
*/}}
{{- define "ghep.containerPort" -}}
{{- $addr := .Values.config.serverAddr | default "0.0.0.0:8080" }}
{{- $parts := splitList ":" $addr }}
{{- index $parts (sub (len $parts) 1) }}
{{- end }}

{{/*
Validate required values.
*/}}
{{- define "ghep.validateValues" -}}
{{- if not .Values.githubOrg }}
{{- fail "githubOrg is required" }}
{{- end }}
{{- if not .Values.existingSecret.name }}
{{- fail "existingSecret.name is required — create a Secret with database and app credentials (see values.yaml)" }}
{{- end }}
{{- if not .Values.database.host }}
{{- fail "database.host is required" }}
{{- end }}
{{- if not .Values.teamsConfig.name }}
{{- fail "teamsConfig.name is required — create a ConfigMap containing your teams.yaml" }}
{{- end }}
{{- if ne (int .Values.replicaCount) 1 }}
{{- fail "replicaCount must be 1 — Ghep has no leader election outside nais and would send duplicate digests" }}
{{- end }}
{{- end }}
