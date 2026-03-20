package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInfoCommand_KnownModel(t *testing.T) {
	yaml := `
name: test-model
model: qwen3:8b
context_length: 8192
quantization: q4_k_m
scaling:
  min_replicas: 1
  max_replicas: 4
  target_tokens_per_second: 500
`
	path := writeTempSpec(t, yaml)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	specFile = path
	err := runInfo(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assertOutputContains(t, output, "Qwen/Qwen3-8B")
	assertOutputContains(t, output, "8B")
	assertOutputContains(t, output, "5.0 GB")
	assertOutputContains(t, output, "BEST")
	assertOutputContains(t, output, "1-4 replicas")
}

func TestInfoCommand_UnknownModel(t *testing.T) {
	yaml := `
name: custom-model
model: org/custom-fine-tune:latest
`
	path := writeTempSpec(t, yaml)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	specFile = path
	err := runInfo(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assertOutputContains(t, output, "unknown")
	assertOutputContains(t, output, "not in registry")
}

func TestInfoCommand_LargeModel(t *testing.T) {
	yaml := `
name: big-model
model: llama3.3:70b
context_length: 32768
`
	path := writeTempSpec(t, yaml)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	specFile = path
	err := runInfo(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assertOutputContains(t, output, "44.0 GB")
	// T4 and L4 should be NO for 70B
	assertOutputContains(t, output, "NO")
}

func writeTempSpec(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "model.yaml")
	os.WriteFile(path, []byte(content), 0644)
	return path
}

func assertOutputContains(t *testing.T, output, needle string) {
	t.Helper()
	if !bytes.Contains([]byte(output), []byte(needle)) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, output)
	}
}
