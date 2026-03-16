package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amayabdaniel/inferctl/pkg/generate"
	"github.com/amayabdaniel/inferctl/pkg/spec"
	"github.com/spf13/cobra"
)

var (
	outputDir string
	target    string
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate Kubernetes manifests from model.yaml",
	Long:  "Reads model.yaml and generates deployment manifests for the target runtime (vllm, triton).",
	RunE:  runGen,
}

func init() {
	genCmd.Flags().StringVarP(&outputDir, "output", "o", "./k8s", "output directory for generated manifests")
	genCmd.Flags().StringVarP(&target, "target", "t", "vllm", "target runtime (vllm)")
	rootCmd.AddCommand(genCmd)
}

func runGen(cmd *cobra.Command, args []string) error {
	s, err := spec.Load(specFile)
	if err != nil {
		return fmt.Errorf("loading spec: %w", err)
	}

	var manifests string

	switch target {
	case "vllm":
		manifests, err = generate.VLLMManifests(s)
		if err != nil {
			return fmt.Errorf("generating vLLM manifests: %w", err)
		}
	default:
		return fmt.Errorf("unsupported target %q (supported: vllm)", target)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	outFile := filepath.Join(outputDir, fmt.Sprintf("%s-%s.yaml", s.Name, target))
	if err := os.WriteFile(outFile, []byte(manifests), 0644); err != nil {
		return fmt.Errorf("writing manifests: %w", err)
	}
	fmt.Printf("inferctl: generated %s manifests → %s\n", target, outFile)

	// Generate Gateway API routes
	gwManifests, err := generate.GatewayManifests(s)
	if err != nil {
		return fmt.Errorf("generating gateway manifests: %w", err)
	}
	gwFile := filepath.Join(outputDir, fmt.Sprintf("%s-gateway.yaml", s.Name))
	if err := os.WriteFile(gwFile, []byte(gwManifests), 0644); err != nil {
		return fmt.Errorf("writing gateway manifests: %w", err)
	}
	fmt.Printf("inferctl: generated gateway routes → %s\n", gwFile)

	return nil
}
