package models

import (
	"fmt"
	"math"
)

// GPUSpec describes a GPU's capabilities for performance prediction.
type GPUSpec struct {
	Name            string
	VRAM_GB         float64
	MemBandwidthGBs float64 // memory bandwidth in GB/s
	FP16_TFLOPS     float64
	CostPerHour     float64
}

// KnownGPUs contains specs for common inference GPUs.
var KnownGPUs = map[string]GPUSpec{
	"T4":        {Name: "T4", VRAM_GB: 16, MemBandwidthGBs: 320, FP16_TFLOPS: 65, CostPerHour: 0.35},
	"L4":        {Name: "L4", VRAM_GB: 24, MemBandwidthGBs: 300, FP16_TFLOPS: 121, CostPerHour: 0.80},
	"A10G":      {Name: "A10G", VRAM_GB: 24, MemBandwidthGBs: 600, FP16_TFLOPS: 125, CostPerHour: 1.01},
	"A100-40GB": {Name: "A100-40GB", VRAM_GB: 40, MemBandwidthGBs: 1555, FP16_TFLOPS: 312, CostPerHour: 3.40},
	"A100-80GB": {Name: "A100-80GB", VRAM_GB: 80, MemBandwidthGBs: 2039, FP16_TFLOPS: 312, CostPerHour: 4.10},
	"H100":      {Name: "H100", VRAM_GB: 80, MemBandwidthGBs: 3350, FP16_TFLOPS: 990, CostPerHour: 8.00},
}

// SimulationResult predicts performance characteristics.
type SimulationResult struct {
	GPU              string
	Fits             bool
	VRAMUsedGB       float64
	VRAMFreeGB       float64
	VRAMUtilPercent  float64
	EstTokensPerSec  float64 // generation speed
	EstTTFTMs        float64 // time to first token in ms
	EstConcurrent    int     // max concurrent requests before degradation
	TokensPerDollar  float64
	Recommendation   string
	Warnings         []string
}

// SimulationInput contains model parameters for prediction.
type SimulationInput struct {
	ParametersBillions float64
	ContextLength      int
	Quantization       string
	BatchSize          int
}

// Simulate predicts performance of a model on a GPU without running it.
// Uses roofline model: LLM inference is memory-bandwidth-bound during generation.
// TTFT is compute-bound (prefill). Token generation is bandwidth-bound (decode).
func Simulate(input SimulationInput, gpu GPUSpec) SimulationResult {
	result := SimulationResult{GPU: gpu.Name}

	// Step 1: Estimate VRAM usage
	bytesPerParam := bytesForQuantization(input.Quantization)
	modelSizeGB := input.ParametersBillions * bytesPerParam

	// KV cache estimate: 2 * layers * hidden_dim * context * 2 (K+V) * bytes
	// Rough heuristic: ~0.5GB per billion params per 4K context
	kvCacheGB := input.ParametersBillions * 0.5 * (float64(input.ContextLength) / 4096.0)

	// Activation memory: ~10% of model size
	activationGB := modelSizeGB * 0.10

	result.VRAMUsedGB = modelSizeGB + kvCacheGB + activationGB
	result.VRAMFreeGB = gpu.VRAM_GB - result.VRAMUsedGB
	result.VRAMUtilPercent = (result.VRAMUsedGB / gpu.VRAM_GB) * 100
	result.Fits = result.VRAMFreeGB > 0

	if !result.Fits {
		result.Recommendation = fmt.Sprintf("Model needs %.1fGB but %s has %.0fGB. Use quantization or a larger GPU.", result.VRAMUsedGB, gpu.Name, gpu.VRAM_GB)
		result.Warnings = append(result.Warnings, "Model does not fit in GPU memory")
		return result
	}

	// Step 2: Estimate token generation speed (decode phase)
	// LLM decode is memory-bandwidth-bound: tokens/sec ≈ bandwidth / model_size
	modelSizeBytes := modelSizeGB * 1e9
	result.EstTokensPerSec = gpu.MemBandwidthGBs * 1e9 / modelSizeBytes
	// Cap at reasonable max (hardware can't exceed certain rates)
	if result.EstTokensPerSec > 200 {
		result.EstTokensPerSec = 200
	}
	result.EstTokensPerSec = math.Round(result.EstTokensPerSec*10) / 10

	// Step 3: Estimate TTFT (prefill phase)
	// Prefill is compute-bound: TTFT ≈ (prompt_tokens * 2 * params) / FLOPS
	promptTokens := 256.0 // assume average prompt
	prefillOps := promptTokens * 2 * input.ParametersBillions * 1e9
	ttftSeconds := prefillOps / (gpu.FP16_TFLOPS * 1e12)
	result.EstTTFTMs = math.Round(ttftSeconds * 1000)
	if result.EstTTFTMs < 10 {
		result.EstTTFTMs = 10 // floor
	}

	// Step 4: Estimate max concurrent requests
	// Each concurrent request needs KV cache memory
	kvPerRequest := kvCacheGB // already scaled by context length
	if kvPerRequest > 0 {
		result.EstConcurrent = int(result.VRAMFreeGB / kvPerRequest)
		if result.EstConcurrent < 1 {
			result.EstConcurrent = 1
		}
		if result.EstConcurrent > 64 {
			result.EstConcurrent = 64
		}
	}

	// Step 5: Calculate tokens per dollar
	tokensPerHour := result.EstTokensPerSec * 3600
	if gpu.CostPerHour > 0 {
		result.TokensPerDollar = math.Round(tokensPerHour / gpu.CostPerHour)
	}

	// Step 6: Generate recommendation
	result.Recommendation = generateRecommendation(result, gpu)

	// Warnings
	if result.VRAMUtilPercent > 90 {
		result.Warnings = append(result.Warnings, "VRAM utilization >90% — may cause OOM under load")
	}
	if result.EstTTFTMs > 2000 {
		result.Warnings = append(result.Warnings, "TTFT >2s — users will notice latency")
	}
	if result.EstConcurrent <= 1 {
		result.Warnings = append(result.Warnings, "Only 1 concurrent request fits — consider a larger GPU for production")
	}

	return result
}

func bytesForQuantization(q string) float64 {
	switch q {
	case "q4_0", "q4_1", "q4_k_m", "q4_k_s", "gptq", "awq":
		return 0.5 // 4-bit = 0.5 bytes per param
	case "q5_0", "q5_1", "q5_k_m", "q5_k_s":
		return 0.625
	case "q6_k":
		return 0.75
	case "q8_0", "fp8":
		return 1.0
	case "":
		return 2.0 // FP16 default
	default:
		return 2.0
	}
}

func generateRecommendation(r SimulationResult, gpu GPUSpec) string {
	if !r.Fits {
		return "Does not fit"
	}
	if r.VRAMUtilPercent < 30 {
		return fmt.Sprintf("Overpaying — %s is too large for this model. Use a cheaper GPU.", gpu.Name)
	}
	if r.VRAMUtilPercent > 90 {
		return "Tight fit — works but may OOM under concurrent load."
	}
	if r.EstTokensPerSec > 50 && r.EstTTFTMs < 500 {
		return "Excellent performance expected."
	}
	if r.EstTokensPerSec > 20 {
		return "Good performance expected."
	}
	return "Usable but may feel slow for interactive use."
}

// LookupModelParams returns the parameter count for a known model.
func LookupModelParams(modelName string) float64 {
	paramMap := map[string]float64{
		"qwen3:8b": 8, "qwen3:14b": 14, "qwen3:32b": 32, "qwen3:72b": 72,
		"llama3.3:8b": 8, "llama3.3:70b": 70,
		"deepseek-r1:7b": 7, "deepseek-r1:14b": 14, "deepseek-r1:70b": 70,
		"ministral:8b": 8, "mistral:7b": 7,
		"phi4:14b": 14,
		"deepseek-coder-v2:16b": 16, "qwen2.5-coder:7b": 7,
		"nomic-embed-text": 0.137,
	}
	if p, ok := paramMap[modelName]; ok {
		return p
	}
	return 0
}
