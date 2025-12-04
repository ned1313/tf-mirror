{{/*
Expand the name of the chart.
*/}}
{{- define "terraform-mirror.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "terraform-mirror.fullname" -}}
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
{{- define "terraform-mirror.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "terraform-mirror.labels" -}}
helm.sh/chart: {{ include "terraform-mirror.chart" . }}
{{ include "terraform-mirror.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "terraform-mirror.selectorLabels" -}}
app.kubernetes.io/name: {{ include "terraform-mirror.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "terraform-mirror.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "terraform-mirror.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use
*/}}
{{- define "terraform-mirror.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "terraform-mirror.fullname" . }}
{{- end }}
{{- end }}

{{/*
Get S3 endpoint for MinIO or external S3
*/}}
{{- define "terraform-mirror.s3Endpoint" -}}
{{- if .Values.minio.enabled }}
{{- printf "http://%s-minio:9000" .Release.Name }}
{{- else }}
{{- .Values.config.storage.s3.endpoint }}
{{- end }}
{{- end }}

{{/*
Get S3 access key
*/}}
{{- define "terraform-mirror.s3AccessKey" -}}
{{- if .Values.minio.enabled }}
{{- .Values.minio.rootUser }}
{{- else }}
{{- .Values.secrets.s3AccessKey }}
{{- end }}
{{- end }}

{{/*
Get S3 secret key
*/}}
{{- define "terraform-mirror.s3SecretKey" -}}
{{- if .Values.minio.enabled }}
{{- .Values.minio.rootPassword }}
{{- else }}
{{- .Values.secrets.s3SecretKey }}
{{- end }}
{{- end }}
