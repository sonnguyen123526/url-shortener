{{/* Chart name, overridable with nameOverride. */}}
{{- define "url-shortener.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified name used for resource names. Kubernetes names are limited to 63 chars.
If the release name already contains the chart name, don't repeat it.
*/}}
{{- define "url-shortener.fullname" -}}
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

{{- define "url-shortener.redisName" -}}
{{- printf "%s-redis" (include "url-shortener.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "url-shortener.redisSecretName" -}}
{{- default (include "url-shortener.redisName" .) .Values.redis.auth.existingSecret }}
{{- end }}

{{/* Labels on every resource. */}}
{{- define "url-shortener.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: {{ include "url-shortener.name" . }}
{{- end }}

{{/* Selector labels. These must never change after install, so keep them minimal. */}}
{{- define "url-shortener.selectorLabels" -}}
app.kubernetes.io/name: {{ include "url-shortener.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: app
{{- end }}

{{- define "url-shortener.redisSelectorLabels" -}}
app.kubernetes.io/name: {{ include "url-shortener.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: redis
{{- end }}
