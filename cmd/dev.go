package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/amayabdaniel/inferctl/pkg/spec"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Run model locally via Ollama",
	Long:  "Reads model.yaml and starts the model using the local Ollama runtime.",
	RunE:  runDev,
}

func init() {
	rootCmd.AddCommand(devCmd)
}

func runDev(cmd *cobra.Command, args []string) error {
	s, err := spec.Load(specFile)
	if err != nil {
		return fmt.Errorf("loading spec: %w", err)
	}

	model := s.OllamaModel()
	fmt.Printf("inferctl: starting %s via Ollama...\n", model)

	// Pull the model if not already available
	pull := exec.Command("ollama", "pull", model)
	pull.Stdout = os.Stdout
	pull.Stderr = os.Stderr
	if err := pull.Run(); err != nil {
		return fmt.Errorf("pulling model %s: %w", model, err)
	}

	// Run the model
	run := exec.Command("ollama", "run", model)
	run.Stdin = os.Stdin
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr

	fmt.Printf("inferctl: model %s ready. Type to chat.\n", s.Name)
	return run.Run()
}
