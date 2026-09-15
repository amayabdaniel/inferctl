package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/amayabdaniel/inferctl/pkg/models"
	"github.com/amayabdaniel/inferctl/pkg/spec"
	"github.com/spf13/cobra"
)

var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Predict model performance on each GPU without deploying",
	Long:  "Estimates VRAM, tokens/sec, TTFT, max concurrent requests, and cost efficiency for each GPU type.",
	RunE:  runSimulate,
}

func init() {
	rootCmd.AddCommand(simulateCmd)
}

func runSimulate(cmd *cobra.Command, args []string) error {
	s, err := spec.Load(specFile)
	if err != nil {
		return err
	}

	params := models.LookupModelParams(s.Model)
	if params == 0 {
		return fmt.Errorf("model %q not in registry — specify a known model or use inferctl info instead", s.Model)
	}

	ctxLen := s.ContextLength
	if ctxLen == 0 {
		ctxLen = 4096
	}

	input := models.SimulationInput{
		ParametersBillions: params,
		ContextLength:      ctxLen,
		Quantization:       s.Quantization,
	}

	fmt.Printf("Simulation: %s (%s, %.0fB params, %s quant, %d ctx)\n\n",
		s.Name, s.Model, params, quantLabel(s.Quantization), ctxLen)

	// Every number in the table below is an ESTIMATE from a named
	// heuristic — not a measurement. Reproducibility conditions are
	// spelled out here so a reader who pastes this into a capacity
	// plan can see what was assumed, and challenge any assumption
	// that does not match their deployment.
	fmt.Println("Assumptions embedded in these estimates:")
	fmt.Printf("  * Weights: params × bytes/param (fp16 = 2 B, q4 = 0.5 B, fp8 = 1 B).\n")
	fmt.Printf("  * KV cache: %.1f GB per B-param per 4K context (linear in context, ignores layer count and GQA).\n", models.KVCacheGBPerBParamsPer4K)
	fmt.Printf("  * Activation: %.0f%% of weight memory.\n", models.ActivationFractionOfWeights*100)
	fmt.Printf("  * Tok/sec: roofline (memory-bandwidth / weights), single-stream decode, capped at %.0f tok/s.\n", models.MaxDecodeTokensPerSec)
	fmt.Printf("  * TTFT: %.0f-token prompt, compute-bound prefill, floored at %.0f ms.\n", models.AssumedPromptTokens, models.TTFTFloorMs)
	fmt.Printf("  * Concurrent: VRAM-free ÷ KV-per-request, clamped [1, %d]. No batching engine modelled.\n", models.MaxConcurrentReported)
	fmt.Println("  NOT modelled: batching gains (real vLLM decode is 3-8× higher with proper batches), attention-KV read overhead at long context, LoRA adapters, engine differences beyond a vLLM-class assumption.")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	// Column headers name their conditions so a screenshot of the table
	// alone still says what it's measuring.
	fmt.Fprintf(w, "GPU\tFits?\tVRAM Used\tVRAM Free\tTok/s (@1×%.0f)\tTTFT (%.0f-tok prompt)\tConcurrent\tTok/$\tVerdict\n",
		models.AssumedPromptTokens, models.AssumedPromptTokens)
	fmt.Fprintln(w, "---\t-----\t---------\t---------\t--------------\t---------------------\t----------\t-----\t-------")

	gpuOrder := []string{"T4", "L4", "A10G", "A100-40GB", "A100-80GB", "H100"}
	for _, name := range gpuOrder {
		gpu := models.KnownGPUs[name]
		r := models.Simulate(input, gpu)

		fits := "NO"
		if r.Fits {
			fits = "YES"
		}

		vramUsed := fmt.Sprintf("%.1fGB", r.VRAMUsedGB)
		vramFree := fmt.Sprintf("%.1fGB", r.VRAMFreeGB)
		tokSec := fmt.Sprintf("%.0f", r.EstTokensPerSec)
		ttft := fmt.Sprintf("%.0fms", r.EstTTFTMs)
		concurrent := fmt.Sprintf("%d", r.EstConcurrent)
		tokDollar := fmt.Sprintf("%.0f", r.TokensPerDollar)

		if !r.Fits {
			vramFree = "--"
			tokSec = "--"
			ttft = "--"
			concurrent = "--"
			tokDollar = "--"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			name, fits, vramUsed, vramFree, tokSec, ttft, concurrent, tokDollar, r.Recommendation)
	}
	w.Flush()

	// Print warnings for best fit
	fmt.Println()
	for _, name := range gpuOrder {
		gpu := models.KnownGPUs[name]
		r := models.Simulate(input, gpu)
		if r.Fits && len(r.Warnings) > 0 {
			fmt.Printf("Warnings for %s:\n", name)
			for _, warn := range r.Warnings {
				fmt.Printf("  - %s\n", warn)
			}
		}
	}

	return nil
}

func quantLabel(q string) string {
	if q == "" {
		return "FP16"
	}
	return q
}
