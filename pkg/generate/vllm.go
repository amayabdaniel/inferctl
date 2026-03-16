package generate

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/amayabdaniel/inferctl/pkg/spec"
)

const vllmDeploymentTmpl = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}-vllm
  labels:
    app.kubernetes.io/name: {{ .Name }}
    app.kubernetes.io/managed-by: inferctl
spec:
  replicas: {{ .MinReplicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .Name }}
        app.kubernetes.io/managed-by: inferctl
    spec:
      containers:
        - name: vllm
          image: vllm/vllm-openai:latest
          args:
            - --model
            - {{ .Model }}
            - --max-model-len
            - "{{ .ContextLength }}"
            - --gpu-memory-utilization
            - "0.90"
{{- if .Quantization }}
            - --quantization
            - {{ .Quantization }}
{{- end }}
          ports:
            - name: http
              containerPort: 8000
          resources:
            limits:
              nvidia.com/gpu: "{{ .GPUCount }}"
{{- if .MemoryMi }}
              memory: {{ .MemoryMi }}Mi
{{- end }}
            requests:
              nvidia.com/gpu: "{{ .GPUCount }}"
{{- if .MemoryMi }}
              memory: {{ .MemoryMi }}Mi
{{- end }}
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 30
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 60
            periodSeconds: 30
      tolerations:
        - key: nvidia.com/gpu
          operator: Exists
          effect: NoSchedule
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-vllm
  labels:
    app.kubernetes.io/name: {{ .Name }}
    app.kubernetes.io/managed-by: inferctl
{{- if .Metrics }}
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8000"
{{- end }}
spec:
  selector:
    app.kubernetes.io/name: {{ .Name }}
  ports:
    - name: http
      port: 8000
      targetPort: http
`

const hpaTmpl = `---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ .Name }}-vllm
  labels:
    app.kubernetes.io/name: {{ .Name }}
    app.kubernetes.io/managed-by: inferctl
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ .Name }}-vllm
  minReplicas: {{ .MinReplicas }}
  maxReplicas: {{ .MaxReplicas }}
  metrics:
    - type: Pods
      pods:
        metric:
          name: vllm_tokens_per_second
        target:
          type: AverageValue
          averageValue: "{{ .TargetTokensPerSec }}"
`

type templateData struct {
	Name              string
	Model             string
	ContextLength     int
	Quantization      string
	GPUCount          int
	MemoryMi          int
	Metrics           bool
	MinReplicas       int
	MaxReplicas       int
	TargetTokensPerSec int
}

func newTemplateData(s *spec.ModelSpec) templateData {
	d := templateData{
		Name:          s.Name,
		Model:         s.VLLMModel(),
		ContextLength: s.ContextLength,
		Quantization:  s.Quantization,
		GPUCount:      s.Resources.GPUCount,
		MemoryMi:      s.Resources.MemoryMi,
		Metrics:       s.Observability.Metrics,
		MinReplicas:   s.Scaling.MinReplicas,
		MaxReplicas:   s.Scaling.MaxReplicas,
		TargetTokensPerSec: s.Scaling.TargetTokensPerSec,
	}

	if d.ContextLength == 0 {
		d.ContextLength = 4096
	}
	if d.GPUCount == 0 {
		d.GPUCount = 1
	}
	if d.MinReplicas == 0 {
		d.MinReplicas = 1
	}
	if d.MaxReplicas == 0 {
		d.MaxReplicas = d.MinReplicas
	}

	return d
}

// VLLMManifests generates Kubernetes YAML for a vLLM deployment from a ModelSpec.
func VLLMManifests(s *spec.ModelSpec) (string, error) {
	data := newTemplateData(s)

	var buf bytes.Buffer

	deployTmpl, err := template.New("deploy").Parse(vllmDeploymentTmpl)
	if err != nil {
		return "", fmt.Errorf("parsing deployment template: %w", err)
	}
	if err := deployTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing deployment template: %w", err)
	}

	if data.MaxReplicas > data.MinReplicas {
		hpa, err := template.New("hpa").Parse(hpaTmpl)
		if err != nil {
			return "", fmt.Errorf("parsing HPA template: %w", err)
		}
		if err := hpa.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("executing HPA template: %w", err)
		}
	}

	return buf.String(), nil
}
