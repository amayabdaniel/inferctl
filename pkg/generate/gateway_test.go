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

func TestGatewayManifests_EmitsNetworkPolicyByDefault(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "secure-agent",
		Model: "llama3.3:70b",
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "kind: NetworkPolicy")
	assertContains(t, out, "name: secure-agent-inference-isolation")
	assertContains(t, out, "component: gateway")
	// NetworkIsolation defaults to true; see TestGatewayManifests_
	// DefaultEmitsNetworkPolicy below for the stronger invariant that
	// an absent security block still emits the policy.
}

// A default spec (no security block at all) MUST NOT emit a
// `criticality:` line — inferctl should not pick a scheduling priority
// on the operator's behalf. Omitting defers to the InferenceModel CRD's
// own default, which is the authority.
func TestGatewayManifests_DefaultOmitsCriticality(t *testing.T) {
	s := &spec.ModelSpec{
		Name:  "no-criticality",
		Model: "qwen3:8b",
	}

	out, err := GatewayManifests(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "criticality:") {
		t.Errorf("default (no criticality set) must not emit a criticality line; got:\n%s", out)
	}
}

// Each of the three valid values, when set explicitly, MUST reach the
// generated manifest verbatim. Table-driven so a fourth CRD value
// (should the InferenceModel schema ever grow one) surfaces here.
func TestGatewayManifests_ExplicitCriticalityValuesEmit(t *testing.T) {
	for _, v := range []string{"Critical", "Standard", "Sheddable"} {
		t.Run(v, func(t *testing.T) {
			s := &spec.ModelSpec{
				Name:  "explicit-crit",
				Model: "qwen3:8b",
				Security: spec.SecuritySpec{
					Criticality: v,
				},
			}
			out, err := GatewayManifests(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertContains(t, out, "criticality: "+v)
		})
	}
}

// Default emits the NetworkPolicy. inferctl generates manifests, it
// does not apply them — so a restrictive default is visible in the YAML
// the operator reviews before applying, and easily removed.
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
