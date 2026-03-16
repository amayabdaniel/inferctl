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

func TestGatewayManifests_NoNetworkPolicyWithoutSecurity(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "open-model",
		Model: "qwen3:8b",
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out, "NetworkPolicy") {
		t.Error("should not generate NetworkPolicy when no security settings")
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
