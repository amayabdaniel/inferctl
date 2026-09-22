package cmd

import (
	"fmt"

	"github.com/amayabdaniel/inferctl/pkg/models"
	"github.com/amayabdaniel/inferctl/pkg/spec"
	"github.com/spf13/cobra"
)

var hoursPerDay float64

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Estimate monthly cloud cost for a model deployment",
	RunE:  runCost,
}

func init() {
	costCmd.Flags().Float64Var(&hoursPerDay, "hours-per-day", 24, "expected GPU hours per day (default: 24 = always on)")
	rootCmd.AddCommand(costCmd)
}

func runCost(cmd *cobra.Command, args []string) error {
	s, err := spec.Load(specFile)
	if err != nil {
		return err
	}

	entry, known := models.KnownModels[s.Model]

	fmt.Printf("Cost Estimate: %s\n", s.Name)
	fmt.Printf("  Model: %s\n", s.Model)
	if known {
		fmt.Printf("  Parameters: %s\n", entry.Parameters)
		fmt.Printf("  Est. VRAM: %.1f GB\n", entry.VRAM_GB)
	}
	fmt.Println()

	minReplicas := s.Scaling.MinReplicas
	if minReplicas == 0 {
		minReplicas = 1
	}
	maxReplicas := s.Scaling.MaxReplicas
	if maxReplicas == 0 {
		maxReplicas = minReplicas
	}

	gpuCount := s.Resources.GPUCount
	if gpuCount == 0 {
		gpuCount = 1
	}

	fmt.Printf("  Replicas: %d min, %d max\n", minReplicas, maxReplicas)
	fmt.Printf("  GPUs per replica: %d\n", gpuCount)
	fmt.Printf("  Hours/day: %.0f\n", hoursPerDay)
	fmt.Println()

	fmt.Println("Monthly Cost Estimates:")
	fmt.Printf("  %-14s  %-10s  %-14s  %-14s\n", "GPU", "$/hr", "Min replicas", "Max replicas")
	fmt.Printf("  %-14s  %-10s  %-14s  %-14s\n", "---", "----", "------------", "------------")

	for _, gpu := range gpuOptions {
		if known && gpu.vram < entry.VRAM_GB {
			continue // skip GPUs that can't fit the model
		}
		minCost := MonthlyGPUCost(gpu.costHr, gpuCount, hoursPerDay, minReplicas)
		maxCost := MonthlyGPUCost(gpu.costHr, gpuCount, hoursPerDay, maxReplicas)
		fmt.Printf("  %-14s  $%-9.2f  $%-13.0f  $%-13.0f\n",
			gpu.name, gpu.costHr, minCost, maxCost)
	}

	fmt.Println()

	// Spot pricing estimate — ILLUSTRATIVE. See IllustrativeSpotDiscount
	// in cost_math.go for the range of real-world variance; the 65%
	// number is a defensible midpoint for L4/A10-class GPUs but the
	// actual discount you'll see depends on region, GPU class, and time
	// of day. Do not quote these numbers as commitments.
	fmt.Println("With Spot/Preemptible (~65% discount — ILLUSTRATIVE, verify against provider's spot history):")
	for _, gpu := range gpuOptions {
		if known && gpu.vram < entry.VRAM_GB {
			continue
		}
		spotRate := SpotHourly(gpu.costHr)
		minCost := MonthlyGPUCost(spotRate, gpuCount, hoursPerDay, minReplicas)
		fmt.Printf("  %-14s  $%-9.2f  $%-13.0f/mo\n",
			gpu.name, spotRate, minCost)
	}

	fmt.Println()

	// Cost comparison with per-minute API pricing
	fmt.Println("Break-even vs per-minute API pricing:")
	minutesPerMonth := hoursPerDay * AvgDaysPerMonth * 60
	apiPrices := []struct {
		name   string
		perMin float64
	}{
		{"Bland.ai", 0.09},
		{"Retell.ai", 0.07},
		{"Vapi.ai", 0.05},
	}

	cheapestGPU := gpuOptions[0]
	if known {
		for _, gpu := range gpuOptions {
			if gpu.vram >= entry.VRAM_GB {
				cheapestGPU = gpu
				break
			}
		}
	}

	selfHostCost := MonthlyGPUCost(cheapestGPU.costHr, gpuCount, hoursPerDay, minReplicas)

	for _, api := range apiPrices {
		breakEvenMin := BreakEvenMinutes(selfHostCost, api.perMin)
		fmt.Printf("  vs %s ($%.2f/min): self-host is cheaper above %.0f min/mo (you have %.0f min available)\n",
			api.name, api.perMin, breakEvenMin, minutesPerMonth)
	}

	return nil
}
