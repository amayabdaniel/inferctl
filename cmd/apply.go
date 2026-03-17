package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/amayabdaniel/inferctl/pkg/generate"
	"github.com/amayabdaniel/inferctl/pkg/spec"
	"github.com/spf13/cobra"
)

var (
	kubeContext string
	namespace   string
	dryRun      bool
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Generate and apply manifests to a Kubernetes cluster",
	Long:  "Reads model.yaml, generates vLLM + Gateway API manifests, and applies them via kubectl.",
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().StringVar(&kubeContext, "context", "", "kubectl context to use")
	applyCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print manifests without applying")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	s, err := spec.Load(specFile)
	if err != nil {
		return fmt.Errorf("loading spec: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "inferctl-apply-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate vLLM manifests
	vllmManifests, err := generate.VLLMManifests(s)
	if err != nil {
		return fmt.Errorf("generating vLLM manifests: %w", err)
	}
	vllmFile := filepath.Join(tmpDir, "vllm.yaml")
	if err := os.WriteFile(vllmFile, []byte(vllmManifests), 0644); err != nil {
		return err
	}

	// Generate Gateway API manifests
	gwManifests, err := generate.GatewayManifests(s)
	if err != nil {
		return fmt.Errorf("generating gateway manifests: %w", err)
	}
	gwFile := filepath.Join(tmpDir, "gateway.yaml")
	if err := os.WriteFile(gwFile, []byte(gwManifests), 0644); err != nil {
		return err
	}

	if dryRun {
		fmt.Println("--- vLLM manifests ---")
		fmt.Println(vllmManifests)
		fmt.Println("--- Gateway API manifests ---")
		fmt.Println(gwManifests)
		return nil
	}

	// Apply via kubectl
	for _, file := range []string{vllmFile, gwFile} {
		kubectlArgs := []string{"apply", "-f", file, "-n", namespace}
		if kubeContext != "" {
			kubectlArgs = append(kubectlArgs, "--context", kubeContext)
		}

		fmt.Printf("inferctl: kubectl apply -f %s -n %s\n", filepath.Base(file), namespace)
		kubectl := exec.Command("kubectl", kubectlArgs...)
		kubectl.Stdout = os.Stdout
		kubectl.Stderr = os.Stderr
		if err := kubectl.Run(); err != nil {
			return fmt.Errorf("kubectl apply failed: %w", err)
		}
	}

	fmt.Printf("inferctl: deployed %s to namespace %s\n", s.Name, namespace)
	return nil
}
