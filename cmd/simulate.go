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

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "GPU\tFits?\tVRAM Used\tVRAM Free\tTok/sec\tTTFT\tConcurrent\tTok/$\tVerdict")
	fmt.Fprintln(w, "---\t-----\t---------\t---------\t-------\t----\t----------\t-----\t-------")

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
