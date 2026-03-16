package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidSpec(t *testing.T) {
	yaml := `
name: support-chat
model: qwen3:8b
context_length: 8192
quantization: q4_k_m
prompt_template: "You are a helpful assistant."
observability:
  metrics: true
  tracing: true
scaling:
  min_replicas: 1
  max_replicas: 4
  target_tokens_per_second: 500
resources:
  gpu: nvidia-l4
  gpu_count: 1
  memory_mi: 16384
  cpu_cores: 4
security:
  prompt_injection_protection: true
  pii_redaction: true
`
	path := writeTempFile(t, yaml)
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Name != "support-chat" {
		t.Errorf("expected name support-chat, got %s", spec.Name)
	}
	if spec.Model != "qwen3:8b" {
		t.Errorf("expected model qwen3:8b, got %s", spec.Model)
	}
	if spec.ContextLength != 8192 {
		t.Errorf("expected context_length 8192, got %d", spec.ContextLength)
	}
	if spec.Scaling.MaxReplicas != 4 {
		t.Errorf("expected max_replicas 4, got %d", spec.Scaling.MaxReplicas)
	}
	if !spec.Security.PromptInjectionProtection {
		t.Error("expected prompt_injection_protection true")
	}
	if !spec.Observability.Metrics {
		t.Error("expected observability.metrics true")
	}
	if spec.Resources.GPU != "nvidia-l4" {
		t.Errorf("expected gpu nvidia-l4, got %s", spec.Resources.GPU)
	}
}

func TestLoad_MissingName(t *testing.T) {
	yaml := `model: qwen3:8b`
	path := writeTempFile(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoad_MissingModel(t *testing.T) {
	yaml := `name: test`
	path := writeTempFile(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestLoad_InvalidScaling(t *testing.T) {
	yaml := `
name: test
model: qwen3:8b
scaling:
  min_replicas: 5
  max_replicas: 2
`
	path := writeTempFile(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for max_replicas < min_replicas")
	}
}

func TestLoad_MinimalSpec(t *testing.T) {
	yaml := `
name: simple
model: llama3.3:8b
`
	path := writeTempFile(t, yaml)
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Name != "simple" {
		t.Errorf("expected name simple, got %s", spec.Name)
	}
}

func TestLoad_WithTools(t *testing.T) {
	yaml := `
name: agent
model: qwen3:8b
tools:
  - name: search
    endpoint: http://search-api/v1/query
  - name: calculator
    schema: '{"type": "object"}'
`
	path := writeTempFile(t, yaml)
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(spec.Tools))
	}
	if spec.Tools[0].Name != "search" {
		t.Errorf("expected tool name search, got %s", spec.Tools[0].Name)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/model.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "model.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
