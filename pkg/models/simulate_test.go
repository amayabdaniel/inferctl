package models

import (
	"math"
	"strings"
	"testing"
)

func TestSimulate_SmallModelOnLargeGPU(t *testing.T) {
	input := SimulationInput{
		ParametersBillions: 8,
		ContextLength:      4096,
		Quantization:       "q4_k_m",
	}

	result := Simulate(input, KnownGPUs["A100-80GB"])

	if !result.Fits {
		t.Error("8B q4 should fit on A100-80GB")
	}
	if result.VRAMUtilPercent > 30 {
		t.Errorf("expected low util for small model on big GPU, got %.1f%%", result.VRAMUtilPercent)
	}
	if result.Recommendation == "" {
		t.Error("expected recommendation")
	}
	// The <30% util branch of generateRecommendation MUST name a cheaper
	// GPU — that's the whole point of running the simulator for a small
	// model on a big card. `t.Logf` here previously let a Recommendation
	// of "" or one that pointed at a MORE expensive GPU pass silently.
	if !strings.Contains(strings.ToLower(result.Recommendation), "cheaper") {
		t.Errorf("recommendation for a small model on a large GPU must call out that a cheaper GPU exists; got: %q", result.Recommendation)
	}
}

func TestSimulate_LargeModelDoesntFit(t *testing.T) {
	input := SimulationInput{
		ParametersBillions: 70,
		ContextLength:      8192,
		Quantization:       "", // FP16
	}

	result := Simulate(input, KnownGPUs["T4"])

	if result.Fits {
		t.Error("70B FP16 should NOT fit on T4")
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for model that doesn't fit")
	}
}

func TestSimulate_QuantizationReducesVRAM(t *testing.T) {
	inputFP16 := SimulationInput{ParametersBillions: 8, ContextLength: 4096, Quantization: ""}
	inputQ4 := SimulationInput{ParametersBillions: 8, ContextLength: 4096, Quantization: "q4_k_m"}

	gpu := KnownGPUs["L4"]
	resultFP16 := Simulate(inputFP16, gpu)
	resultQ4 := Simulate(inputQ4, gpu)

	if resultQ4.VRAMUsedGB >= resultFP16.VRAMUsedGB {
		t.Errorf("q4 should use less VRAM than FP16: q4=%.1fGB, fp16=%.1fGB",
			resultQ4.VRAMUsedGB, resultFP16.VRAMUsedGB)
	}
}

func TestSimulate_HigherBandwidthFasterTokens(t *testing.T) {
	input := SimulationInput{ParametersBillions: 8, ContextLength: 4096, Quantization: "q4_k_m"}

	resultT4 := Simulate(input, KnownGPUs["T4"])
	resultH100 := Simulate(input, KnownGPUs["H100"])

	if resultH100.EstTokensPerSec <= resultT4.EstTokensPerSec {
		t.Errorf("H100 should be faster than T4: H100=%.1f tok/s, T4=%.1f tok/s",
			resultH100.EstTokensPerSec, resultT4.EstTokensPerSec)
	}
}

func TestSimulate_LongerContextMoreVRAM(t *testing.T) {
	input4K := SimulationInput{ParametersBillions: 8, ContextLength: 4096, Quantization: "q4_k_m"}
	input32K := SimulationInput{ParametersBillions: 8, ContextLength: 32768, Quantization: "q4_k_m"}

	gpu := KnownGPUs["L4"]
	result4K := Simulate(input4K, gpu)
	result32K := Simulate(input32K, gpu)

	if result32K.VRAMUsedGB <= result4K.VRAMUsedGB {
		t.Errorf("32K context should use more VRAM: 4K=%.1fGB, 32K=%.1fGB",
			result4K.VRAMUsedGB, result32K.VRAMUsedGB)
	}
}

func TestSimulate_ConcurrentRequests(t *testing.T) {
	input := SimulationInput{ParametersBillions: 8, ContextLength: 4096, Quantization: "q4_k_m"}

	resultSmall := Simulate(input, KnownGPUs["L4"])
	resultBig := Simulate(input, KnownGPUs["A100-80GB"])

	if resultBig.EstConcurrent <= resultSmall.EstConcurrent {
		t.Errorf("bigger GPU should support more concurrent requests: L4=%d, A100=%d",
			resultSmall.EstConcurrent, resultBig.EstConcurrent)
	}
}

func TestSimulate_TokensPerDollar(t *testing.T) {
	input := SimulationInput{ParametersBillions: 8, ContextLength: 4096, Quantization: "q4_k_m"}

	result := Simulate(input, KnownGPUs["T4"])

	if result.TokensPerDollar <= 0 {
		t.Error("expected positive tokens per dollar")
	}
}

func TestSimulate_AllGPUs(t *testing.T) {
	input := SimulationInput{ParametersBillions: 8, ContextLength: 4096, Quantization: "q4_k_m"}

	for name, gpu := range KnownGPUs {
		result := Simulate(input, gpu)
		if !result.Fits {
			t.Errorf("8B q4 should fit on %s", name)
		}
		if result.EstTokensPerSec <= 0 {
			t.Errorf("expected positive tokens/sec on %s", name)
		}
		if result.EstTTFTMs <= 0 {
			t.Errorf("expected positive TTFT on %s", name)
		}
	}
}

func TestLookupModelParams(t *testing.T) {
	if p := LookupModelParams("qwen3:8b"); p != 8 {
		t.Errorf("expected 8B for qwen3:8b, got %f", p)
	}
	if p := LookupModelParams("llama3.3:70b"); p != 70 {
		t.Errorf("expected 70B for llama3.3:70b, got %f", p)
	}
	if p := LookupModelParams("unknown"); p != 0 {
		t.Errorf("expected 0 for unknown, got %f", p)
	}
}

// --- Assumption pins ------------------------------------------------------
//
// Each named constant in simulate.go controls what number ends up in a
// user's capacity plan. If any of them drifts, the CLI banner that
// prints assumptions must drift with it. These tests do TWO things:
//   1) pin the constant so drift is visible in `git diff`,
//   2) prove that changing the input in an expected way moves the
//      output in an expected magnitude — so a swapped operator (`+`
//      for `*`, `2` for `0.5`) would fail one of these.

func TestSimulate_KVCacheHeuristicPin(t *testing.T) {
	if KVCacheGBPerBParamsPer4K != 0.5 {
		t.Errorf("KV cache constant changed to %v; update the CLI banner in cmd/simulate.go alongside", KVCacheGBPerBParamsPer4K)
	}
	// Sanity: 10B params × 4K ctx → 5 GB (10 × 0.5 × 1.0).
	// 10B params × 8K ctx → 10 GB (linear scaling in context).
	b4k := EstimateVRAMBreakdown(10, 4096, "")
	b8k := EstimateVRAMBreakdown(10, 8192, "")
	if diff := b8k.KVCacheGB - 2*b4k.KVCacheGB; math.Abs(diff) > 1e-6 {
		t.Errorf("KV cache must scale linearly in context length; got 4k=%v 8k=%v", b4k.KVCacheGB, b8k.KVCacheGB)
	}
}

func TestSimulate_ActivationFractionPin(t *testing.T) {
	if ActivationFractionOfWeights != 0.10 {
		t.Errorf("Activation fraction changed to %v; update the CLI banner", ActivationFractionOfWeights)
	}
	// 8B fp16 → 16 GB weights → 1.6 GB activation.
	b := EstimateVRAMBreakdown(8, 4096, "")
	if math.Abs(b.WeightsGB-16) > 1e-6 {
		t.Errorf("expected 16 GB weights (8 × 2 bytes), got %v", b.WeightsGB)
	}
	if math.Abs(b.ActivationGB-1.6) > 1e-6 {
		t.Errorf("expected 1.6 GB activation (10%% of 16), got %v", b.ActivationGB)
	}
}

func TestSimulate_TokensPerSecCapPin(t *testing.T) {
	if MaxDecodeTokensPerSec != 200.0 {
		t.Errorf("Decode tok/s cap changed to %v; update the CLI banner", MaxDecodeTokensPerSec)
	}
	// H100's 3350 GB/s ÷ tiny 1 GB weights = 3350 tok/s uncapped → must
	// clamp to 200. If a future refactor drops the cap, this catches it.
	tps := EstimateDecodeTokensPerSec(KnownGPUs["H100"], 1.0)
	if tps > MaxDecodeTokensPerSec {
		t.Errorf("cap must clamp roofline output; got %v", tps)
	}
	if tps != MaxDecodeTokensPerSec {
		t.Errorf("with a bandwidth ceiling far above cap, output must equal the cap exactly; got %v", tps)
	}
}

func TestSimulate_TokensPerSecRooflineBelowCap(t *testing.T) {
	// 14B fp16 → 28 GB weights on T4 (320 GB/s) → 320 / 28 = 11.43 tok/s
	// Below the cap; must NOT be clamped.
	tps := EstimateDecodeTokensPerSec(KnownGPUs["T4"], 28.0)
	want := math.Round(320.0/28.0*10) / 10 // 11.4
	if math.Abs(tps-want) > 1e-6 {
		t.Errorf("roofline formula must be bandwidth/weights below cap; got %v want %v", tps, want)
	}
}

func TestSimulate_AssumedPromptTokensPin(t *testing.T) {
	if AssumedPromptTokens != 256.0 {
		t.Errorf("Prompt-tokens assumption changed to %v; update the CLI banner", AssumedPromptTokens)
	}
	// 8B model on A100-80GB (312 TFLOPS): 256 × 2 × 8e9 / 312e12 = 13.1ms
	ttft := EstimateTTFTMs(8, KnownGPUs["A100-80GB"])
	want := math.Round(256 * 2 * 8e9 / (312e12) * 1000)
	if math.Abs(ttft-want) > 0.001 && ttft != TTFTFloorMs {
		t.Errorf("TTFT must equal prefill/FLOPS or hit floor; got %v want %v (or floor %v)", ttft, want, TTFTFloorMs)
	}
}

func TestSimulate_TTFTFloorPin(t *testing.T) {
	if TTFTFloorMs != 10.0 {
		t.Errorf("TTFT floor changed to %v; update the CLI banner", TTFTFloorMs)
	}
	// Tiny model on H100: prefill is sub-1ms; must clamp to floor.
	// 0.5B × 256 × 2 / 990e12 → ~250 μs raw → floors to 10ms.
	ttft := EstimateTTFTMs(0.5, KnownGPUs["H100"])
	if ttft != TTFTFloorMs {
		t.Errorf("expected floor %v, got %v", TTFTFloorMs, ttft)
	}
}

func TestSimulate_MaxConcurrentPin(t *testing.T) {
	if MaxConcurrentReported != 64 {
		t.Errorf("Concurrent cap changed to %v; update the CLI banner", MaxConcurrentReported)
	}
	// vram_free 128GB, kv 0.1GB/req → raw = 1280, must clamp to 64.
	if c := EstimateConcurrent(128, 0.1); c != MaxConcurrentReported {
		t.Errorf("expected cap %v, got %v", MaxConcurrentReported, c)
	}
	// Floor at 1: tight fit that would round to zero must not report 0.
	if c := EstimateConcurrent(0.3, 1.0); c != 1 {
		t.Errorf("under-fit must floor at 1, got %v", c)
	}
	// Zero KV: return 0 sentinel (caller handles).
	if c := EstimateConcurrent(80, 0); c != 0 {
		t.Errorf("kv=0 must return 0 sentinel, got %v", c)
	}
}

// Cross-check: doubling parameters at fixed context doubles WEIGHTS
// but does NOT double KV cache the same way (KV scales as
// params * context, so linear in params too). This test catches an
// operator swap where either WeightsGB or KVCacheGB stopped scaling
// with params.
func TestSimulate_VRAMScalesWithParams(t *testing.T) {
	b8 := EstimateVRAMBreakdown(8, 4096, "")   // fp16
	b16 := EstimateVRAMBreakdown(16, 4096, "") // fp16
	if math.Abs(b16.WeightsGB-2*b8.WeightsGB) > 1e-6 {
		t.Errorf("weights must scale linearly with params; got 8B=%v 16B=%v", b8.WeightsGB, b16.WeightsGB)
	}
	if math.Abs(b16.KVCacheGB-2*b8.KVCacheGB) > 1e-6 {
		t.Errorf("KV cache must scale linearly with params at fixed context; got 8B=%v 16B=%v", b8.KVCacheGB, b16.KVCacheGB)
	}
}

// Cross-check: quantization affects weights ONLY, not KV cache. If a
// future refactor accidentally applies bytesPerParam to the KV
// heuristic (which is often what a naive extension would do), this
// catches it.
func TestSimulate_QuantizationAffectsWeightsNotKV(t *testing.T) {
	fp16 := EstimateVRAMBreakdown(8, 4096, "")     // 2 bytes/param
	q4 := EstimateVRAMBreakdown(8, 4096, "q4_k_m") // 0.5 bytes/param
	if q4.WeightsGB >= fp16.WeightsGB {
		t.Error("q4 weights must be smaller than fp16 weights")
	}
	if math.Abs(q4.KVCacheGB-fp16.KVCacheGB) > 1e-6 {
		t.Errorf("quantization must NOT change KV cache heuristic; got fp16=%v q4=%v", fp16.KVCacheGB, q4.KVCacheGB)
	}
}

func TestBytesForQuantization(t *testing.T) {
	if b := bytesForQuantization("q4_k_m"); b != 0.5 {
		t.Errorf("expected 0.5 for q4, got %f", b)
	}
	if b := bytesForQuantization(""); b != 2.0 {
		t.Errorf("expected 2.0 for FP16 default, got %f", b)
	}
	if b := bytesForQuantization("fp8"); b != 1.0 {
		t.Errorf("expected 1.0 for fp8, got %f", b)
	}
}
