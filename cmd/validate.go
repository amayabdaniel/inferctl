package cmd

import (
	"fmt"

	"github.com/amayabdaniel/inferctl/pkg/spec"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a model.yaml spec",
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	s, err := spec.Load(specFile)
	if err != nil {
		return err
	}

	fmt.Printf("inferctl: %s is valid (model: %s)\n", s.Name, s.Model)
	return nil
}
