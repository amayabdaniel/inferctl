package spec

import (
	"bytes"
	"fmt"
	"os"

	"github.com/amayabdaniel/inferctl/pkg/models"
	"gopkg.in/yaml.v3"
)

// ModelSpec is the single source of truth for a model deployment.
// Same file runs on Ollama locally and generates K8s manifests for vLLM.
type ModelSpec struct {
	Name           string            `yaml:"name"`
	Model          string            `yaml:"model"`
	ContextLength  int               `yaml:"context_length,omitempty"`
	Quantization   string            `yaml:"quantization,omitempty"`
	PromptTemplate string            `yaml:"prompt_template,omitempty"`
	Tools          []ToolSpec        `yaml:"tools,omitempty"`
	Observability  ObservabilitySpec `yaml:"observability,omitempty"`
	Scaling        ScalingSpec       `yaml:"scaling,omitempty"`
	Resources      ResourceSpec      `yaml:"resources,omitempty"`
	Security       SecuritySpec      `yaml:"security,omitempty"`
}

type ToolSpec struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Schema   string `yaml:"schema,omitempty"`
}

// ObservabilitySpec previously carried a Tracing bool alongside Metrics,
// documented in examples/model.yaml and README's quick-start as if the
// generated vLLM Deployment honored it — but the vllm template only
// reads Observability.Metrics (wiring --metrics on the vllm command),
// and no generated manifest ever emitted tracing config. Removed rather
// than kept as scaffolding; the honest re-introduction is to add
// Tracing back the same commit that wires an OTEL/Jaeger exporter into
// the generated Deployment.
type ObservabilitySpec struct {
	Metrics bool `yaml:"metrics,omitempty"`
}

type ScalingSpec struct {
	MinReplicas        int `yaml:"min_replicas,omitempty"`
	MaxReplicas        int `yaml:"max_replicas,omitempty"`
	TargetTokensPerSec int `yaml:"target_tokens_per_second,omitempty"`
}

type ResourceSpec struct {
	GPU      string `yaml:"gpu,omitempty"`
	GPUCount int    `yaml:"gpu_count,omitempty"`
	MemoryMi int    `yaml:"memory_mi,omitempty"`
	CPUCores int    `yaml:"cpu_cores,omitempty"`
}

// SecuritySpec previously carried PromptInjectionProtection and
// PIIRedaction bools, documented in examples/model.yaml and tested
// through round-trip fixtures. inferctl generates manifests for
// Gateway API + InferenceModel, and NEITHER CRD has a field where
// those flags would land — they are proxy-layer controls, enforced by
// modelgate at request time, not by the CRDs this tool emits. Two
// prior commits (8bb7612 decoupling them from NetworkIsolation,
// 1a13ed1 decoupling them from Criticality) removed WRONG wirings and
// left the fields in place, with gateway tests that assert "LLM flags
// do not drive X" — which was accurate but concealed that the fields
// drive NOTHING at all. Removed rather than kept as scaffolding so
// the schema stops promising controls inferctl can't deliver; the
// honest home for them is already modelgate's InferencePolicySpec,
// linked from the README. Same shape as the ObservabilitySpec Tracing
// removal in this commit and the modelgate Budgets removal in
// fb0e07f.
type SecuritySpec struct {
	// Criticality maps to the InferenceModel CRD's scheduling-priority
	// classification. Empty (absent in YAML) means "do not emit the
	// field", deferring to the CRD's own default — inferctl should not
	// pick a scheduling priority on the operator's behalf because none
	// of the three values is safe as a default (Critical can starve
	// other workloads, Sheddable can evict this one under pressure,
	// Standard is a guess in the middle). Legal values validated by
	// Sanitize: "" (omit), "Critical", "Standard", "Sheddable".
	Criticality string `yaml:"criticality,omitempty"`

	// NetworkIsolation controls whether GatewayManifests emits a
	// NetworkPolicy that restricts ingress to the app-gateway pod and
	// egress to DNS + HTTPS. Pointer so a nil (absent-in-YAML) value
	// defaults to true — inferctl generates manifests, it does not
	// apply them, so a restrictive default appears in the YAML the
	// operator reviews before applying, and is easily deletable. An
	// invisible absence of a NetworkPolicy is not something anyone
	// notices in a diff review. An explicit `network_isolation: false`
	// opts out.
	NetworkIsolation *bool `yaml:"network_isolation,omitempty"`
}

// EmitNetworkPolicy reports whether GatewayManifests should include the
// NetworkPolicy for this spec. Nil (unset) means the default of true.
func (s SecuritySpec) EmitNetworkPolicy() bool {
	if s.NetworkIsolation == nil {
		return true
	}
	return *s.NetworkIsolation
}

func Load(path string) (*ModelSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}

	// Strict decoding: any unknown key in the YAML fails at parse rather
	// than being silently dropped. A typo like `securty:` or a stale
	// field like `allowed_origins:` (which the tool once carried and
	// never consumed) must not travel unnoticed to a generated manifest.
	var spec ModelSpec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return nil, fmt.Errorf("parsing spec file: %w", err)
	}

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}

	if err := spec.Sanitize(); err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	return &spec, nil
}

func (s *ModelSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	if s.Model == "" {
		return fmt.Errorf("model is required")
	}
	if s.ContextLength < 0 {
		return fmt.Errorf("context_length must be non-negative")
	}
	if s.Scaling.MinReplicas < 0 {
		return fmt.Errorf("min_replicas must be non-negative")
	}
	if s.Scaling.MaxReplicas > 0 && s.Scaling.MaxReplicas < s.Scaling.MinReplicas {
		return fmt.Errorf("max_replicas must be >= min_replicas")
	}
	return nil
}

// OllamaModel returns the model identifier formatted for Ollama.
// If quantization is specified, appends it (e.g., "qwen3:8b" stays as-is if no override).
func (s *ModelSpec) OllamaModel() string {
	return s.Model
}

// VLLMModel returns the HuggingFace model identifier for vLLM.
// Converts Ollama-style names to HF format (e.g., "qwen3:8b" → "Qwen/Qwen3-8B").
func (s *ModelSpec) VLLMModel() string {
	return models.LookupHuggingFace(s.Model)
}
