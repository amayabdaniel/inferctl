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

// Regression pin: the vLLM image reference must NOT be `:latest`. A
// floating tag in a MANIFEST GENERATOR is one violation per user of the
// tool — not one violation total. See the header comment in vllm.go and
// research-factory ADR 0008 for the class.
func TestVLLMManifests_ImageIsPinned(t *testing.T) {
	s := &spec.ModelSpec{Name: "x", Model: "org/x"}
	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "vllm/vllm-openai:latest") {
		t.Error("template must NOT emit :latest — that's the ADR-0008 class this generator was leaking")
	}
	// Positive: some concrete version must appear so the pin is real,
	// not merely "not latest".
	if !strings.Contains(out, "vllm/vllm-openai:v") {
		t.Errorf("expected a pinned v-tagged vllm image, got:\n%s", out)
	}
}

// FMEA-style effect check for the second bug: cpu_cores in model.yaml
// was being silently dropped from the generated Deployment. Validation
// passed, the user thought it was honoured, and the container shipped
// with no CPU request or limit. Test the emitted manifest, not the
// intermediate templateData, because the whole failure lived between
// spec-load and template-render.
func TestVLLMManifests_CPUCoresIsEmitted(t *testing.T) {
	s := &spec.ModelSpec{
		Name:      "with-cpu",
		Model:     "org/x",
		Resources: spec.ResourceSpec{GPUCount: 1, MemoryMi: 16384, CPUCores: 4},
	}
	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatal(err)
	}
	// The template inlines cpu under BOTH requests and limits — verify
	// both, or a future refactor could drop one silently.
	if strings.Count(out, `cpu: "4"`) < 2 {
		t.Errorf("expected cpu limit AND request both set to \"4\", got:\n%s", out)
	}
}

// Backwards-compat pin: with cpu_cores UNSET, no cpu line should be
// emitted (the previous behaviour). Catches an "always emit cpu: 0"
// regression that would over-constrain deployments that don't set it.
func TestVLLMManifests_CPUCoresOmittedWhenUnset(t *testing.T) {
	s := &spec.ModelSpec{
		Name:      "no-cpu",
		Model:     "org/x",
		Resources: spec.ResourceSpec{GPUCount: 1, MemoryMi: 16384},
	}
	out, err := VLLMManifests(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "cpu:") {
		t.Errorf("cpu MUST NOT appear when CPUCores unset (backwards compat); got:\n%s", out)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}
