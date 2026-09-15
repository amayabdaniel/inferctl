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

// The Simulate numbers are ESTIMATES with named assumptions, not
// measurements. Extracting each heuristic into a named constant + a
// small pure function means the CLI can print those assumptions
// alongside the numbers, and unit tests can pin each heuristic in
// isolation. If any of these constants change, the corresponding
// assertion in simulate_test.go and the printed banner in
// cmd/simulate.go must change too — the test set is the audit trail.
//
// What is NOT modelled today (a reviewer looking at the output should
// know these limits, so the CLI banner names them):
//   * Batch size — decode throughput assumes a single request stream.
//     Real vLLM/TGI with proper batching sees 3-8x higher tok/s.
//   * Attention-KV read overhead for long context in the decode
//     roofline (the formula uses model weight size only).
//   * Engine efficiency — an assumed vLLM-class PagedAttention
//     implementation, capped at MaxDecodeTokensPerSec to keep an
//     over-optimistic bandwidth number from suggesting throughput
//     no real engine reaches on this hardware.
//   * LoRA / adapter memory.
//   * Prompt length beyond AssumedPromptTokens for TTFT.
const (
	// KVCacheGBPerBParamsPer4K is the KV-cache-size heuristic:
	// approximately 0.5 GB per billion parameters, per 4K context
	// tokens. The real formula is `2 * layers * hidden_dim * context
	// * 2 (K+V) * bytes` — this ignores layer count and hidden dim
	// and interpolates linearly with context. For 7-14B dense
	// transformers at fp16 it is within ~30%; MoE models and models
	// with grouped-query attention will be off further.
	KVCacheGBPerBParamsPer4K = 0.5

	// ActivationFractionOfWeights is the fraction of the loaded
	// weights added for intermediate activation memory. 10% is a
	// rough figure covering activations + workspace; real engines
	// vary from ~5% (aggressive) to ~15% (conservative).
	ActivationFractionOfWeights = 0.10

	// MaxDecodeTokensPerSec caps the reported tokens/second so the
	// roofline number (bandwidth ÷ weights) can't imply throughput a
	// real engine cannot reach — realistic single-stream vLLM decode
	// tops out well before hardware bandwidth peak because attention
	// reads and pipeline overhead eat into the ceiling.
	MaxDecodeTokensPerSec = 200.0

	// AssumedPromptTokens is the prompt length assumed for the
	// prefill/TTFT calculation. Real prompts vary from 10s to 10k+
	// tokens; TTFT scales roughly linearly with prompt length in the
	// compute-bound regime.
	AssumedPromptTokens = 256.0

	// TTFTFloorMs prevents the estimate from printing sub-10ms
	// numbers for tiny models on H100 — real single-request TTFT
	// has kernel-launch + tokenizer overhead below this floor.
	TTFTFloorMs = 10.0

	// MaxConcurrentReported caps the concurrency estimate; beyond
	// this the estimate is unreliable (real ceilings are set by
	// batching engines, not raw KV-cache math).
	MaxConcurrentReported = 64
)

// VRAMBreakdown splits VRAM usage into its three heuristic components
// so tests and the CLI banner can name each. Sum is what's actually
// consumed (VRAMUsedGB).
type VRAMBreakdown struct {
	WeightsGB    float64
	KVCacheGB    float64
	ActivationGB float64
}

// EstimateVRAMBreakdown applies the three heuristics from the constants
// above. Pure function; unit-testable without a GPU or a Simulate call.
// (Named "Breakdown" to avoid collision with the older
// EstimateVRAM(modelName) helper in registry.go, which returns a single
// scalar look-up-by-model — a different job.)
func EstimateVRAMBreakdown(paramsB float64, contextLen int, quantization string) VRAMBreakdown {
	weights := paramsB * bytesForQuantization(quantization)
	kv := paramsB * KVCacheGBPerBParamsPer4K * (float64(contextLen) / 4096.0)
	activation := weights * ActivationFractionOfWeights
	return VRAMBreakdown{WeightsGB: weights, KVCacheGB: kv, ActivationGB: activation}
}

// EstimateDecodeTokensPerSec applies the roofline model
// `bandwidth / weights` and caps at MaxDecodeTokensPerSec. Returns the
// rounded value the CLI prints.
func EstimateDecodeTokensPerSec(gpu GPUSpec, weightsGB float64) float64 {
	if weightsGB <= 0 {
		return 0
	}
	tps := gpu.MemBandwidthGBs * 1e9 / (weightsGB * 1e9)
	if tps > MaxDecodeTokensPerSec {
		tps = MaxDecodeTokensPerSec
	}
	return math.Round(tps*10) / 10
}

// EstimateTTFTMs computes the compute-bound prefill time for the
// AssumedPromptTokens length, floored at TTFTFloorMs.
func EstimateTTFTMs(paramsB float64, gpu GPUSpec) float64 {
	prefillOps := AssumedPromptTokens * 2 * paramsB * 1e9
	if gpu.FP16_TFLOPS <= 0 {
		return TTFTFloorMs
	}
	ttft := math.Round(prefillOps / (gpu.FP16_TFLOPS * 1e12) * 1000)
	if ttft < TTFTFloorMs {
		return TTFTFloorMs
	}
	return ttft
}

// EstimateConcurrent divides remaining VRAM by KV-per-request and
// clamps to [1, MaxConcurrentReported]. Returns 0 only when
// kvPerRequest is non-positive (unmodelled shape).
func EstimateConcurrent(vramFreeGB, kvPerRequestGB float64) int {
	if kvPerRequestGB <= 0 {
		return 0
	}
	c := int(vramFreeGB / kvPerRequestGB)
	if c < 1 {
		return 1
	}
	if c > MaxConcurrentReported {
		return MaxConcurrentReported
	}
	return c
}

// Simulate predicts performance of a model on a GPU without running it.
// Uses roofline model: LLM inference is memory-bandwidth-bound during generation.
// TTFT is compute-bound (prefill). Token generation is bandwidth-bound (decode).
// See the constants above for every assumption embedded in these numbers.
func Simulate(input SimulationInput, gpu GPUSpec) SimulationResult {
	result := SimulationResult{GPU: gpu.Name}

	// Step 1: VRAM breakdown.
	vram := EstimateVRAMBreakdown(input.ParametersBillions, input.ContextLength, input.Quantization)
	result.VRAMUsedGB = vram.WeightsGB + vram.KVCacheGB + vram.ActivationGB
	result.VRAMFreeGB = gpu.VRAM_GB - result.VRAMUsedGB
	result.VRAMUtilPercent = (result.VRAMUsedGB / gpu.VRAM_GB) * 100
	result.Fits = result.VRAMFreeGB > 0

	if !result.Fits {
		result.Recommendation = fmt.Sprintf("Model needs %.1fGB but %s has %.0fGB. Use quantization or a larger GPU.", result.VRAMUsedGB, gpu.Name, gpu.VRAM_GB)
		result.Warnings = append(result.Warnings, "Model does not fit in GPU memory")
		return result
	}

	// Step 2-4: throughput + latency + concurrency estimates.
	result.EstTokensPerSec = EstimateDecodeTokensPerSec(gpu, vram.WeightsGB)
	result.EstTTFTMs = EstimateTTFTMs(input.ParametersBillions, gpu)
	result.EstConcurrent = EstimateConcurrent(result.VRAMFreeGB, vram.KVCacheGB)

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
