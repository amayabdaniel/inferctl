package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitize_ValidSpec(t *testing.T) {
	s := &ModelSpec{
		Name:         "support-chat",
		Model:        "qwen3:8b",
		Quantization: "q4_k_m",
		Tools: []ToolSpec{
			{Name: "search", Endpoint: "http://api.local/search"},
		},
	}
	if err := s.Sanitize(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestSanitize_PathTraversalInModel(t *testing.T) {
	s := &ModelSpec{Name: "test-model", Model: "../../../etc/passwd"}
	if err := s.Sanitize(); err == nil {
		t.Fatal("expected error for path traversal in model")
	}
}

func TestSanitize_ShellInjectionInModel(t *testing.T) {
	injections := []string{
		"model; rm -rf /",
		"model | cat /etc/passwd",
		"model$(whoami)",
		"model`id`",
		"model & curl evil.com",
	}
	for _, model := range injections {
		s := &ModelSpec{Name: "test-model", Model: model}
		if err := s.Sanitize(); err == nil {
			t.Errorf("expected error for shell injection in model: %q", model)
		}
	}
}

func TestSanitize_InvalidName(t *testing.T) {
	invalid := []string{
		"UPPERCASE",
		"has spaces",
		"has_underscore",
		"-starts-with-dash",
		"a",                     // too short
		strings.Repeat("x", 65), // too long
	}
	for _, name := range invalid {
		s := &ModelSpec{Name: name, Model: "qwen3:8b"}
		if err := s.Sanitize(); err == nil {
			t.Errorf("expected error for invalid name: %q", name)
		}
	}
}

func TestSanitize_ValidNames(t *testing.T) {
	valid := []string{
		"my-model",
		"support-chat-v2",
		"ab",
		"model-123-test",
	}
	for _, name := range valid {
		s := &ModelSpec{Name: name, Model: "qwen3:8b"}
		if err := s.Sanitize(); err != nil {
			t.Errorf("expected valid name %q, got: %v", name, err)
		}
	}
}

func TestSanitize_InvalidQuantization(t *testing.T) {
	s := &ModelSpec{Name: "test-model", Model: "qwen3:8b", Quantization: "not-a-real-quant"}
	if err := s.Sanitize(); err == nil {
		t.Fatal("expected error for invalid quantization")
	}
}

func TestSanitize_ToolEndpointValidation(t *testing.T) {
	// Valid
	s := &ModelSpec{
		Name:  "test-model",
		Model: "qwen3:8b",
		Tools: []ToolSpec{{Name: "api", Endpoint: "https://api.example.com/v1"}},
	}
	if err := s.Sanitize(); err != nil {
		t.Fatalf("expected valid tool endpoint, got: %v", err)
	}

	// Invalid scheme
	s.Tools[0].Endpoint = "ftp://evil.com"
	if err := s.Sanitize(); err == nil {
		t.Fatal("expected error for non-http tool endpoint")
	}

	// Path traversal in endpoint
	s.Tools[0].Endpoint = "http://api.com/../../../etc/passwd"
	if err := s.Sanitize(); err == nil {
		t.Fatal("expected error for path traversal in tool endpoint")
	}
}

func TestSanitize_ToolShellInjection(t *testing.T) {
	s := &ModelSpec{
		Name:  "test-model",
		Model: "qwen3:8b",
		Tools: []ToolSpec{{Name: "evil;rm -rf /"}},
	}
	if err := s.Sanitize(); err == nil {
		t.Fatal("expected error for shell injection in tool name")
	}
}

func TestSanitize_PromptTemplateTooLong(t *testing.T) {
	s := &ModelSpec{
		Name:           "test-model",
		Model:          "qwen3:8b",
		PromptTemplate: strings.Repeat("x", 10001),
	}
	if err := s.Sanitize(); err == nil {
		t.Fatal("expected error for oversized prompt template")
	}
}

func TestSanitize_ModelRefTooLong(t *testing.T) {
	s := &ModelSpec{Name: "test-model", Model: strings.Repeat("x", 257)}
	if err := s.Sanitize(); err == nil {
		t.Fatal("expected error for oversized model reference")
	}
}

// Criticality validation: empty (omit-the-field) and the three CRD
// values pass; anything else — including case variants and typos —
// fails with a message that names the offending value.
func TestSanitize_CriticalityValidation(t *testing.T) {
	for _, ok := range []string{"", "Critical", "Standard", "Sheddable"} {
		t.Run("accepts_"+ok, func(t *testing.T) {
			s := &ModelSpec{Name: "test", Model: "qwen3:8b", Security: SecuritySpec{Criticality: ok}}
			if err := s.Sanitize(); err != nil {
				t.Errorf("valid criticality %q rejected: %v", ok, err)
			}
		})
	}
	for _, bad := range []string{"standard", "critical", "sheddable", "Stanadard", "high", "low"} {
		t.Run("rejects_"+bad, func(t *testing.T) {
			s := &ModelSpec{Name: "test", Model: "qwen3:8b", Security: SecuritySpec{Criticality: bad}}
			err := s.Sanitize()
			if err == nil {
				t.Fatalf("invalid criticality %q accepted", bad)
			}
			if !strings.Contains(err.Error(), bad) {
				t.Errorf("error must name the offending value %q; got: %v", bad, err)
			}
		})
	}
}

// AllowedOrigins was declared, validated, tested, and never consumed —
// nothing in the gateway generator emitted a CORS policy, so an operator
// setting allowed_origins got a spec that validated cleanly and
// restricted nothing. Removed in this repo. This test pins the removal:
// a spec that carries allowed_origins now fails LOUDLY at parse (via
// strict decoding in Load), not silently at manifest generation.
func TestLoad_AllowedOriginsIsRemovedAndFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.yaml")
	body := `
name: test
model: qwen3:8b
security:
  allowed_origins:
    - https://app.example.com
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for removed allowed_origins field, got nil")
	}
	if !strings.Contains(err.Error(), "allowed_origins") {
		t.Errorf("error must name the removed field so an operator can find their stale config; got: %v", err)
	}
}
