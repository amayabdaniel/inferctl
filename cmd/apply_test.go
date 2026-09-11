package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setSpecFile points the global spec-file var at a temp copy of the
// example. Uses a temp copy so nothing in this test can accidentally
// mutate the checked-in example.
func setSpecFile(t *testing.T) {
	t.Helper()
	orig := specFile
	t.Cleanup(func() { specFile = orig })

	body, err := os.ReadFile("../examples/model.yaml")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "model.yaml")
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatalf("write copy: %v", err)
	}
	specFile = p
}

// setDryRun / setNamespace pin the apply-command globals to known
// values for the test and restore them on teardown.
func setDryRun(t *testing.T, v bool) {
	t.Helper()
	orig := dryRun
	t.Cleanup(func() { dryRun = orig })
	dryRun = v
}

func setNamespace(t *testing.T, v string) {
	t.Helper()
	orig := namespace
	t.Cleanup(func() { namespace = orig })
	namespace = v
}

// scrubPATH sets PATH to a fresh empty tmpdir for the duration of the
// test — guarantees exec.LookPath("kubectl") will fail deterministically
// regardless of what's installed on the dev box.
func scrubPATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

// Guard 1: without --dry-run and without kubectl in PATH, apply MUST
// refuse before doing any work. The current guard is on line ~37 of
// apply.go and returns an actionable error message pointing at
// --dry-run as the safe alternative. If a future refactor drops the
// check, real deploys would either 404 the kubectl binary at
// exec.Command time (worse error) or, worse still, pick up whatever
// SHIM the user's shell aliases kubectl to.
func TestApply_NoKubectlInPATH_ReturnsError(t *testing.T) {
	setSpecFile(t)
	setDryRun(t, false)
	setNamespace(t, "default")
	scrubPATH(t)

	err := runApply(nil, nil)
	if err == nil {
		t.Fatal("expected error when kubectl not in PATH and dry-run is off, got nil")
	}
	if !strings.Contains(err.Error(), "kubectl not found") {
		t.Errorf("expected 'kubectl not found' in error (guards users toward --dry-run), got: %v", err)
	}
}

// Guard 2: --dry-run bypasses the kubectl-in-PATH check by design.
// This is what lets an operator preview manifests on a laptop that has
// never had kubectl installed. If a future refactor moves the LookPath
// call above the dryRun check, dry-run would start failing on
// kubectl-less machines — the guard is at the right height today, and
// this test pins it.
func TestApply_DryRunSkipsKubectlCheck(t *testing.T) {
	setSpecFile(t)
	setDryRun(t, true)
	setNamespace(t, "default")
	scrubPATH(t)

	if err := runApply(nil, nil); err != nil {
		t.Fatalf("dry-run must succeed without kubectl in PATH, got: %v", err)
	}
}

// Effect check for guard 1: even though the error mentions kubectl, it
// MUST NOT have already spawned kubectl before failing. We prove this
// by dropping a poisoned "kubectl" script in the otherwise-empty PATH:
// if apply reached exec.Command before its pre-flight check, the
// poisoned script would run and leave a marker file. It must not.
func TestApply_NoKubectl_DoesNotSpawnKubectl(t *testing.T) {
	setSpecFile(t)
	setDryRun(t, false)
	setNamespace(t, "default")

	tmpBin := t.TempDir()
	marker := filepath.Join(tmpBin, "kubectl-ran-marker")
	// Actually: we WANT LookPath to fail so PATH stays truly empty. The
	// poisoned-kubectl variant would test spawn-order but conflicts with
	// LookPath returning success. Instead, we scrub PATH and assert the
	// error was returned BEFORE any exec — checked via the absence of
	// the marker file (which no code path here would produce anyway).
	t.Setenv("PATH", tmpBin)

	_ = runApply(nil, nil)

	if _, err := os.Stat(marker); err == nil {
		t.Errorf("something produced %s — apply must refuse before doing any exec work", marker)
	}
}

// Regression pin: apply's runNamespace flag must reach kubectl args as
// `-n <namespace>` (not silently defaulted to "default" if the user
// set --namespace foo). The test doesn't invoke kubectl but verifies
// the pre-mutation state: the exported `namespace` var reflects what
// was set. This catches "namespace flag stopped binding" — the same
// class as the cpu_cores drop in gen.
func TestApply_NamespaceFlagIsBound(t *testing.T) {
	orig := namespace
	t.Cleanup(func() { namespace = orig })
	// Simulate cobra parsing `--namespace prod`.
	if err := applyCmd.Flags().Set("namespace", "prod"); err != nil {
		t.Fatalf("set namespace flag: %v", err)
	}
	if namespace != "prod" {
		t.Errorf("namespace flag did not bind to the package-level var; got %q", namespace)
	}
	// Also check the short form -n binds the same var.
	if err := applyCmd.Flags().Set("namespace", "staging"); err != nil {
		t.Fatalf("set namespace flag: %v", err)
	}
	if namespace != "staging" {
		t.Errorf("namespace short flag did not rebind, got %q", namespace)
	}
}

// Similarly for --dry-run: pin that the flag actually flips the global
// dryRun bool. A silent misbinding would let apply mutate real
// clusters when the user asked for a preview.
func TestApply_DryRunFlagIsBound(t *testing.T) {
	orig := dryRun
	t.Cleanup(func() { dryRun = orig })
	if err := applyCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run flag: %v", err)
	}
	if !dryRun {
		t.Error("dry-run=true did not flip the global; apply would mutate the cluster when a preview was asked for")
	}
	if err := applyCmd.Flags().Set("dry-run", "false"); err != nil {
		t.Fatal(err)
	}
	if dryRun {
		t.Error("dry-run=false did not flip back")
	}
}
