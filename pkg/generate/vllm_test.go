package generate

import (
	"strings"
	"testing"

	"github.com/amayabdaniel/inferctl/pkg/spec"
)

func TestVLLMManifests_BasicDeployment(t *testing.T) {
	s := &spec.ModelSpec{
		Name:          "support-chat",
		Model:         "Qwen/Qwen3-8B",
		ContextLength: 8192,
		Resources:     spec.ResourceSpec{GPUCount: 1, MemoryMi: 16384},
		Observability: spec.ObservabilitySpec{Metrics: true},
	}

	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "name: support-chat-vllm")
	assertContains(t, out, "Qwen/Qwen3-8B")
	assertContains(t, out, `nvidia.com/gpu: "1"`)
	assertContains(t, out, "memory: 16384Mi")
	assertContains(t, out, "prometheus.io/scrape")
	assertContains(t, out, "--max-model-len")
	assertContains(t, out, `"8192"`)
	assertContains(t, out, "managed-by: inferctl")
}

func TestVLLMManifests_WithAutoscaling(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "agent",
		Model: "meta-llama/Llama-3.3-70B",
		Scaling: spec.ScalingSpec{
			MinReplicas:        2,
			MaxReplicas:        8,
			TargetTokensPerSec: 1000,
		},
		Resources: spec.ResourceSpec{GPUCount: 2},
	}

	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "kind: HorizontalPodAutoscaler")
	assertContains(t, out, "minReplicas: 2")
	assertContains(t, out, "maxReplicas: 8")
	assertContains(t, out, "vllm_tokens_per_second")
	assertContains(t, out, `replicas: 2`)
}

func TestVLLMManifests_NoHPAWhenNoScaling(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "simple",
		Model: "qwen3:8b",
	}

	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out, "HorizontalPodAutoscaler") {
		t.Error("should not generate HPA when max_replicas == min_replicas")
	}
}

func TestVLLMManifests_WithQuantization(t *testing.T) {
	s := &spec.ModelSpec{
		Name:         "quantized",
		Model:        "Qwen/Qwen3-8B",
		Quantization: "awq",
	}

	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "--quantization")
	assertContains(t, out, "awq")
}

func TestVLLMManifests_Defaults(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "minimal",
		Model: "tiny-model",
	}

	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, `"4096"`)       // default context length
	assertContains(t, out, `nvidia.com/gpu: "1"`) // default 1 GPU
	assertContains(t, out, "replicas: 1")         // default 1 replica
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}
