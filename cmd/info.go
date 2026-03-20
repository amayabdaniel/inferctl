package cmd

import (
	"fmt"

	"github.com/amayabdaniel/inferctl/pkg/models"
	"github.com/amayabdaniel/inferctl/pkg/spec"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show model info, VRAM estimate, and GPU recommendation",
	RunE:  runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

type gpuOption struct {
	name     string
	vram     float64
	costHr   float64
}

var gpuOptions = []gpuOption{
	{"T4", 16, 0.35},
	{"L4", 24, 0.80},
	{"A10G", 24, 1.01},
	{"A100-40GB", 40, 3.40},
	{"A100-80GB", 80, 4.10},
	{"H100", 80, 8.00},
}

func runInfo(cmd *cobra.Command, args []string) error {
	s, err := spec.Load(specFile)
	if err != nil {
		return err
	}

	fmt.Printf("Model: %s\n", s.Name)
	fmt.Printf("  Ollama:      %s\n", s.OllamaModel())
	fmt.Printf("  HuggingFace: %s\n", s.VLLMModel())

	entry, known := models.KnownModels[s.Model]
	if known {
		fmt.Printf("  Parameters:  %s\n", entry.Parameters)
		fmt.Printf("  Est. VRAM:   %.1f GB\n", entry.VRAM_GB)
	} else {
		fmt.Printf("  Parameters:  unknown (not in registry)\n")
		fmt.Printf("  Est. VRAM:   unknown\n")
	}

	if s.Quantization != "" {
		fmt.Printf("  Quantization: %s\n", s.Quantization)
	}
	if s.ContextLength > 0 {
		fmt.Printf("  Context:     %d tokens\n", s.ContextLength)
	}

	fmt.Println()

	if !known {
		fmt.Println("GPU Recommendation: model not in registry, specify resources.gpu manually")
		return nil
	}

	vram := entry.VRAM_GB
	// Context length increases VRAM — rough estimate: +1GB per 4K context above 4096
	if s.ContextLength > 4096 {
		extraCtx := float64(s.ContextLength-4096) / 4096.0
		vram += extraCtx * 1.0
	}

	fmt.Println("GPU Compatibility:")
	recommended := false
	for _, gpu := range gpuOptions {
		status := "  "
		if gpu.vram >= vram*1.15 { // 15% headroom
			status = "OK"
			if !recommended {
				status = "BEST"
				recommended = true
			}
		} else if gpu.vram >= vram {
			status = "TIGHT"
		} else {
			status = "NO"
		}

		fmt.Printf("  %-14s %4.0f GB  $%.2f/hr  [%s]\n", gpu.name, gpu.vram, gpu.costHr, status)
	}

	if !recommended {
		fmt.Println("\n  WARNING: model may not fit on any single GPU. Consider quantization or multi-GPU.")
	}

	// Scaling info
	if s.Scaling.MaxReplicas > 1 {
		fmt.Printf("\nScaling: %d-%d replicas, target %d tokens/sec\n",
			s.Scaling.MinReplicas, s.Scaling.MaxReplicas, s.Scaling.TargetTokensPerSec)
	}

	return nil
}
