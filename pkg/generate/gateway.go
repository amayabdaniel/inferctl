package generate

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/amayabdaniel/inferctl/pkg/spec"
)

const inferenceRouteTmpl = `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ .Name }}-route
  labels:
    app.kubernetes.io/name: {{ .Name }}
    app.kubernetes.io/managed-by: inferctl
spec:
  parentRefs:
    - name: inference-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /v1
          headers:
            - name: x-model
              value: {{ .Name }}
      backendRefs:
        - name: {{ .Name }}-vllm
          port: 8000
---
apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: InferenceModel
metadata:
  name: {{ .Name }}
  labels:
    app.kubernetes.io/name: {{ .Name }}
    app.kubernetes.io/managed-by: inferctl
spec:
  modelName: {{ .Model }}
  targetModels:
    - name: {{ .Name }}-vllm
      weight: 100
{{- if .CostBudget }}
  criticality: Standard
{{- end }}
`

const networkPolicyTmpl = `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .Name }}-inference-isolation
  labels:
    app.kubernetes.io/name: {{ .Name }}
    app.kubernetes.io/managed-by: inferctl
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: {{ .Name }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/component: gateway
      ports:
        - port: 8000
          protocol: TCP
  egress:
    - to: []
      ports:
        - port: 53
          protocol: UDP
        - port: 443
          protocol: TCP
`

type gatewayData struct {
	Name       string
	Model      string
	CostBudget bool
	Isolated   bool
}

// GatewayManifests generates Gateway API Inference Extension routes and optional
// network isolation policies from a ModelSpec.
func GatewayManifests(s *spec.ModelSpec) (string, error) {
	data := gatewayData{
		Name:  s.Name,
		Model: s.VLLMModel(),
		// CostBudget stays coupled to the LLM-safety flags for now — the
		// coupling is a separate finding and requires a policy decision
		// (a cost budget implies a numeric ceiling that has to come from
		// somewhere, unlike NetworkPolicy which can default restrictive
		// visibly).
		CostBudget: s.Security.PromptInjectionProtection || s.Security.PIIRedaction,
		// Network isolation derives from an explicit security field.
		// Default (nil pointer) is true so a spec with no security block
		// still gets a NetworkPolicy in its generated YAML — restrictive
		// default, visibly present in the diff, explicit opt-out via
		// `security.network_isolation: false`.
		Isolated: s.Security.EmitNetworkPolicy(),
	}

	var buf bytes.Buffer

	routeTmpl, err := template.New("route").Parse(inferenceRouteTmpl)
	if err != nil {
		return "", fmt.Errorf("parsing route template: %w", err)
	}
	if err := routeTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing route template: %w", err)
	}

	if data.Isolated {
		npTmpl, err := template.New("netpol").Parse(networkPolicyTmpl)
		if err != nil {
			return "", fmt.Errorf("parsing network policy template: %w", err)
		}
		if err := npTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("executing network policy template: %w", err)
		}
	}

	return buf.String(), nil
}
