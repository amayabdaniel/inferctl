package generate

import (
	"strings"
	"testing"

	"github.com/amayabdaniel/inferctl/pkg/spec"
)

func TestGatewayManifests_BasicRoute(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "support-chat",
		Model: "Qwen/Qwen3-8B",
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "kind: HTTPRoute")
	assertContains(t, out, "name: support-chat-route")
	assertContains(t, out, "kind: InferenceModel")
	assertContains(t, out, "modelName: Qwen/Qwen3-8B")
	assertContains(t, out, "name: support-chat-vllm")
	assertContains(t, out, "weight: 100")
	assertContains(t, out, "managed-by: inferctl")
	assertContains(t, out, "x-model")
}

func TestGatewayManifests_WithSecurity(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "secure-agent",
		Model: "llama3.3:70b",
		Security: spec.SecuritySpec{
			PromptInjectionProtection: true,
			PIIRedaction:              true,
		},
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "kind: NetworkPolicy")
	assertContains(t, out, "name: secure-agent-inference-isolation")
	assertContains(t, out, "component: gateway")
	assertContains(t, out, "criticality: Standard")
}

// Default emits the NetworkPolicy. inferctl generates manifests, it
// does not apply them — so a restrictive default is visible in the YAML
// the operator reviews before applying, and easily removed. The
// previous behaviour (no NetworkPolicy unless the operator turned on
// EITHER PromptInjectionProtection OR PIIRedaction, two unrelated
// LLM-safety features) was invisible-permissive: the absence of a
// policy in a diff is not something a reviewer notices.
func TestGatewayManifests_DefaultEmitsNetworkPolicy(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "open-model",
		Model: "qwen3:8b",
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "kind: NetworkPolicy") {
		t.Error("default (no security block) must emit a NetworkPolicy; a restrictive-visible default beats a permissive-invisible one")
	}
}

func TestGatewayManifests_ExplicitOptOut(t *testing.T) {
	off := false
	s := &spec.ModelSpec{
		Name:  "public-model",
		Model: "qwen3:8b",
		Security: spec.SecuritySpec{
			NetworkIsolation: &off,
		},
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out, "NetworkPolicy") {
		t.Error("explicit network_isolation: false must skip NetworkPolicy emission")
	}
}

// This is the load-bearing test. It pins the DECOUPLING — the previous
// implementation gated Isolated on `PromptInjectionProtection ||
// PIIRedaction`, so an operator who wanted prompt-injection protection
// got a NetworkPolicy they didn't ask for, and an operator who wanted
// network isolation off but PII redaction on could not express it. The
// two are unrelated. After this change, LLM-safety flags do not drive
// isolation and isolation does not drive LLM-safety flags.
func TestGatewayManifests_LLMFlagsDoNotDriveIsolation(t *testing.T) {
	off := false
	s := &spec.ModelSpec{
		Name:  "safety-only-model",
		Model: "llama3.3:70b",
		Security: spec.SecuritySpec{
			PromptInjectionProtection: true,
			PIIRedaction:              true,
			NetworkIsolation:          &off,
		},
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out, "NetworkPolicy") {
		t.Errorf("LLM-safety flags being on MUST NOT force NetworkPolicy when network_isolation is explicitly off; got NetworkPolicy in output")
	}
}

func TestGatewayManifests_InferenceModelPointsToVLLMService(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "my-model",
		Model: "test/model",
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "name: my-model-vllm")
	assertContains(t, out, "port: 8000")
}
