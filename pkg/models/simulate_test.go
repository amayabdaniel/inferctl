package models

import (
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
	// Should recommend a cheaper GPU
	if result.VRAMUtilPercent < 30 {
		t.Logf("Recommendation: %s", result.Recommendation)
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
