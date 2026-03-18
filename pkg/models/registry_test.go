package models

import "testing"

func TestLookupHuggingFace_Known(t *testing.T) {
	tests := []struct {
		ollama string
		hf     string
	}{
		{"qwen3:8b", "Qwen/Qwen3-8B"},
		{"llama3.3:70b", "meta-llama/Llama-3.3-70B-Instruct"},
		{"deepseek-r1:7b", "deepseek-ai/DeepSeek-R1-Distill-Qwen-7B"},
		{"ministral:8b", "mistralai/Ministral-8B-Instruct-2410"},
		{"nomic-embed-text", "nomic-ai/nomic-embed-text-v1.5"},
	}

	for _, tt := range tests {
		got := LookupHuggingFace(tt.ollama)
		if got != tt.hf {
			t.Errorf("LookupHuggingFace(%q) = %q, want %q", tt.ollama, got, tt.hf)
		}
	}
}

func TestLookupHuggingFace_Unknown(t *testing.T) {
	got := LookupHuggingFace("some-custom-model:latest")
	if got != "some-custom-model:latest" {
		t.Errorf("expected passthrough for unknown model, got %q", got)
	}
}

func TestLookupOllama_Known(t *testing.T) {
	got := LookupOllama("Qwen/Qwen3-8B")
	if got != "qwen3:8b" {
		t.Errorf("expected qwen3:8b, got %q", got)
	}
}

func TestLookupOllama_Unknown(t *testing.T) {
	got := LookupOllama("org/unknown-model")
	if got != "org/unknown-model" {
		t.Errorf("expected passthrough for unknown model, got %q", got)
	}
}

func TestEstimateVRAM(t *testing.T) {
	if v := EstimateVRAM("qwen3:8b"); v != 5.0 {
		t.Errorf("expected 5.0 GB for qwen3:8b, got %f", v)
	}
	if v := EstimateVRAM("llama3.3:70b"); v != 44.0 {
		t.Errorf("expected 44.0 GB for llama3.3:70b, got %f", v)
	}
	if v := EstimateVRAM("nomic-embed-text"); v != 0.5 {
		t.Errorf("expected 0.5 GB for nomic-embed-text, got %f", v)
	}
	if v := EstimateVRAM("unknown"); v != 0 {
		t.Errorf("expected 0 for unknown model, got %f", v)
	}
}

func TestRegistryCompleteness(t *testing.T) {
	// Every entry should have both Ollama and HuggingFace fields
	for key, entry := range KnownModels {
		if entry.Ollama == "" {
			t.Errorf("model %q has empty Ollama name", key)
		}
		if entry.HuggingFace == "" {
			t.Errorf("model %q has empty HuggingFace name", key)
		}
		if entry.VRAM_GB <= 0 {
			t.Errorf("model %q has invalid VRAM estimate: %f", key, entry.VRAM_GB)
		}
	}
}

func TestBidirectionalLookup(t *testing.T) {
	// For every known model, Ollama → HF → Ollama should roundtrip
	for key, entry := range KnownModels {
		hf := LookupHuggingFace(key)
		if hf != entry.HuggingFace {
			t.Errorf("Ollama→HF failed for %q: got %q, want %q", key, hf, entry.HuggingFace)
		}
		back := LookupOllama(hf)
		if back != entry.Ollama {
			t.Errorf("HF→Ollama roundtrip failed for %q: got %q, want %q", key, back, entry.Ollama)
		}
	}
}
