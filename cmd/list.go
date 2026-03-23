package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listNamespace string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all inferctl-managed deployments on the cluster",
	RunE:  runList,
}

func init() {
	listCmd.Flags().StringVarP(&listNamespace, "namespace", "n", "", "Kubernetes namespace (default: all namespaces)")
	rootCmd.AddCommand(listCmd)
}

type k8sDeployment struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
	} `json:"status"`
}

type k8sDeploymentList struct {
	Items []k8sDeployment `json:"items"`
}

func runList(cmd *cobra.Command, args []string) error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH")
	}

	kubectlArgs := []string{"get", "deployments",
		"-l", "app.kubernetes.io/managed-by=inferctl",
		"-o", "json",
	}

	if listNamespace != "" {
		kubectlArgs = append(kubectlArgs, "-n", listNamespace)
	} else {
		kubectlArgs = append(kubectlArgs, "--all-namespaces")
	}

	out, err := exec.Command("kubectl", kubectlArgs...).Output()
	if err != nil {
		return fmt.Errorf("kubectl failed: %w", err)
	}

	var list k8sDeploymentList
	if err := json.Unmarshal(out, &list); err != nil {
		return fmt.Errorf("parsing kubectl output: %w", err)
	}

	if len(list.Items) == 0 {
		fmt.Println("No inferctl-managed deployments found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tNAME\tMODEL\tREPLICAS\tREADY\tSTATUS")

	for _, d := range list.Items {
		model := d.Metadata.Labels["app.kubernetes.io/name"]
		status := "NotReady"
		if d.Status.ReadyReplicas >= d.Spec.Replicas && d.Spec.Replicas > 0 {
			status = "Running"
		} else if d.Status.ReadyReplicas > 0 {
			status = "Partial"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
			d.Metadata.Namespace,
			d.Metadata.Name,
			model,
			d.Spec.Replicas,
			d.Status.ReadyReplicas,
			status,
		)
	}
	w.Flush()

	return nil
}
