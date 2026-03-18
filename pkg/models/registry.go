package models

// Registry maps between Ollama model names and HuggingFace model IDs.
// This is the bridge that lets model.yaml work on both runtimes.

// Entry represents a known model with both Ollama and HuggingFace identifiers.
type Entry struct {
	Ollama      string
	HuggingFace string
	Parameters  string
	VRAM_GB     float64
}

// KnownModels maps Ollama-style names to their HuggingFace equivalents.
var KnownModels = map[string]Entry{
	// Qwen 3
	"qwen3:8b":       {Ollama: "qwen3:8b", HuggingFace: "Qwen/Qwen3-8B", Parameters: "8B", VRAM_GB: 5.0},
	"qwen3:14b":      {Ollama: "qwen3:14b", HuggingFace: "Qwen/Qwen3-14B", Parameters: "14B", VRAM_GB: 9.0},
	"qwen3:32b":      {Ollama: "qwen3:32b", HuggingFace: "Qwen/Qwen3-32B", Parameters: "32B", VRAM_GB: 20.0},
	"qwen3:72b":      {Ollama: "qwen3:72b", HuggingFace: "Qwen/Qwen3-72B", Parameters: "72B", VRAM_GB: 44.0},

	// Llama 3.3
	"llama3.3:8b":    {Ollama: "llama3.3:8b", HuggingFace: "meta-llama/Llama-3.3-8B-Instruct", Parameters: "8B", VRAM_GB: 5.0},
	"llama3.3:70b":   {Ollama: "llama3.3:70b", HuggingFace: "meta-llama/Llama-3.3-70B-Instruct", Parameters: "70B", VRAM_GB: 44.0},

	// DeepSeek
	"deepseek-r1:7b": {Ollama: "deepseek-r1:7b", HuggingFace: "deepseek-ai/DeepSeek-R1-Distill-Qwen-7B", Parameters: "7B", VRAM_GB: 5.0},
	"deepseek-r1:14b":{Ollama: "deepseek-r1:14b", HuggingFace: "deepseek-ai/DeepSeek-R1-Distill-Qwen-14B", Parameters: "14B", VRAM_GB: 9.0},
	"deepseek-r1:70b":{Ollama: "deepseek-r1:70b", HuggingFace: "deepseek-ai/DeepSeek-R1-Distill-Llama-70B", Parameters: "70B", VRAM_GB: 44.0},

	// Mistral / Ministral
	"ministral:8b":   {Ollama: "ministral:8b", HuggingFace: "mistralai/Ministral-8B-Instruct-2410", Parameters: "8B", VRAM_GB: 5.0},
	"mistral:7b":     {Ollama: "mistral:7b", HuggingFace: "mistralai/Mistral-7B-Instruct-v0.3", Parameters: "7B", VRAM_GB: 5.0},

	// Phi
	"phi4:14b":       {Ollama: "phi4:14b", HuggingFace: "microsoft/phi-4", Parameters: "14B", VRAM_GB: 9.0},

	// Code models
	"deepseek-coder-v2:16b": {Ollama: "deepseek-coder-v2:16b", HuggingFace: "deepseek-ai/DeepSeek-Coder-V2-Lite-Instruct", Parameters: "16B", VRAM_GB: 10.0},
	"qwen2.5-coder:7b":      {Ollama: "qwen2.5-coder:7b", HuggingFace: "Qwen/Qwen2.5-Coder-7B-Instruct", Parameters: "7B", VRAM_GB: 5.0},

	// Embeddings
	"nomic-embed-text": {Ollama: "nomic-embed-text", HuggingFace: "nomic-ai/nomic-embed-text-v1.5", Parameters: "137M", VRAM_GB: 0.5},
}

// LookupHuggingFace converts an Ollama model name to a HuggingFace ID.
// Returns the input unchanged if not found in the registry.
func LookupHuggingFace(ollamaName string) string {
	if entry, ok := KnownModels[ollamaName]; ok {
		return entry.HuggingFace
	}
	return ollamaName
}

// LookupOllama converts a HuggingFace model ID to an Ollama name.
// Returns the input unchanged if not found in the registry.
func LookupOllama(hfName string) string {
	for _, entry := range KnownModels {
		if entry.HuggingFace == hfName {
			return entry.Ollama
		}
	}
	return hfName
}

// EstimateVRAM returns the estimated VRAM usage in GB for a model.
// Returns 0 if the model is not in the registry.
func EstimateVRAM(modelName string) float64 {
	if entry, ok := KnownModels[modelName]; ok {
		return entry.VRAM_GB
	}
	return 0
}
