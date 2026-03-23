package spec

import (
	"fmt"
	"os"

	"github.com/amayabdaniel/inferctl/pkg/models"
	"gopkg.in/yaml.v3"
)

// ModelSpec is the single source of truth for a model deployment.
// Same file runs on Ollama locally and generates K8s manifests for vLLM.
type ModelSpec struct {
	Name           string          `yaml:"name"`
	Model          string          `yaml:"model"`
	ContextLength  int             `yaml:"context_length,omitempty"`
	Quantization   string          `yaml:"quantization,omitempty"`
	PromptTemplate string          `yaml:"prompt_template,omitempty"`
	Tools          []ToolSpec      `yaml:"tools,omitempty"`
	Observability  ObservabilitySpec `yaml:"observability,omitempty"`
	Scaling        ScalingSpec     `yaml:"scaling,omitempty"`
	Resources      ResourceSpec    `yaml:"resources,omitempty"`
	Security       SecuritySpec    `yaml:"security,omitempty"`
}

type ToolSpec struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Schema   string `yaml:"schema,omitempty"`
}

type ObservabilitySpec struct {
	Metrics bool `yaml:"metrics,omitempty"`
	Tracing bool `yaml:"tracing,omitempty"`
}

type ScalingSpec struct {
	MinReplicas          int `yaml:"min_replicas,omitempty"`
	MaxReplicas          int `yaml:"max_replicas,omitempty"`
	TargetTokensPerSec   int `yaml:"target_tokens_per_second,omitempty"`
}

type ResourceSpec struct {
	GPU       string `yaml:"gpu,omitempty"`
	GPUCount  int    `yaml:"gpu_count,omitempty"`
	MemoryMi  int    `yaml:"memory_mi,omitempty"`
	CPUCores  int    `yaml:"cpu_cores,omitempty"`
}

type SecuritySpec struct {
	PromptInjectionProtection bool     `yaml:"prompt_injection_protection,omitempty"`
	PIIRedaction              bool     `yaml:"pii_redaction,omitempty"`
	AllowedOrigins            []string `yaml:"allowed_origins,omitempty"`
}

func Load(path string) (*ModelSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}

	var spec ModelSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
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
