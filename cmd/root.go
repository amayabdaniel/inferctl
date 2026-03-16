package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var specFile string

var rootCmd = &cobra.Command{
	Use:   "inferctl",
	Short: "Local-to-cloud AI deployment bridge",
	Long:  "One manifest from local Ollama to Kubernetes vLLM. Deploy the same model.yaml everywhere.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&specFile, "file", "f", "model.yaml", "path to model spec file")
}
