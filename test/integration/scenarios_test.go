//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Scenario helpers — each scenario applies a fixture into its own namespace,
// waits for it to settle, and cleans up via t.Cleanup.
// ---------------------------------------------------------------------------

// applyFixture kubectl-applies the given file using the test's KUBECONFIG.
// Registers a t.Cleanup that deletes the namespace on test completion so
// scenarios don't leak state between runs.
func applyFixture(t *testing.T, fixture, ns string) {
	t.Helper()
	path := filepath.Join("testdata", fixture)
	cmd := exec.Command("kubectl", "apply", "-f", path)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("kubectl apply %s: %v\n%s", fixture, err, out)
	}
	t.Cleanup(func() {
		c := exec.Command("kubectl", "delete", "namespace", ns, "--wait=false", "--ignore-not-found")
		c.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
		_ = c.Run()
	})
}

// waitForCondition polls kubectl jsonpath until the returned value equals
// want, or until timeout. Useful for "wait for restart_count >= 3".
func waitForCondition(t *testing.T, args []string, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(deadline) {
		cmd := exec.Command("kubectl", args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
		out, err := cmd.Output()
		if err == nil {
			last = strings.TrimSpace(string(out))
			if last == want {
				return last
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out waiting for %v == %q (last=%q)", args, want, last)
	return ""
}

// waitForAtLeast polls a numeric kubectl jsonpath until the value >= want.
func waitForAtLeast(t *testing.T, args []string, want int, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := 0
	for time.Now().Before(deadline) {
		cmd := exec.Command("kubectl", args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
		out, err := cmd.Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if s != "" {
				last = atoi(s)
				if last >= want {
					return last
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out waiting for %v >= %d (last=%d)", args, want, last)
	return last
}

// probeNS is a variant of runProvider that lets the caller specify the
// component name, namespace, and type. The original runProvider is hard-coded
// to the default-namespace nginx scenario.
func probeNS(t *testing.T, binary, component, fact, ns, typ string, extra ...string) string {
	t.Helper()
	args := []string{"probe", component, fact, "--namespace", ns, "--type", typ}
	args = append(args, extra...)
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("probe %s.%s (ns=%s, type=%s) failed: %v\nstderr: %s",
			component, fact, ns, typ, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// probeResult is the full probe response — used by scenarios that assert on
// Status (not_found) in addition to Value.
type probeResult struct {
	Value  any    `json:"value"`
	Raw    string `json:"raw"`
	Status string `json:"status"`
}

func parseProbe(t *testing.T, out string) probeResult {
	t.Helper()
	var r probeResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("parse probe output: %v (raw=%q)", err, out)
	}
	return r
}

// probeAllowFail runs a probe that may legitimately return non-zero exit
// (e.g. to exercise ErrUsage). Returns stdout + stderr + exit code.
func probeAllowFail(t *testing.T, binary string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run provider: %v", err)
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), code
}

// ---------------------------------------------------------------------------
// Scenario 1 — CrashLoopBackOff
// ---------------------------------------------------------------------------
//
// Applies a deployment whose container exits immediately. The probe surface
// should reflect:
//   - deployment.ready_replicas == 0
//   - deployment.restart_count > 0 (at least one restart observed)
//   - pod.phase eventually stabilizes but ready stays false
//   - condition_available == false

func TestScenario_CrashLoop(t *testing.T) {
	const ns = "mgtt-it-crashloop"
	applyFixture(t, "crashloop.yaml", ns)

	// Wait until the kubelet has restarted the container at least 2 times,
	// proving the CrashLoopBackOff cycle is active.
	waitForAtLeast(t, []string{
		"-n", ns, "get", "pods", "-l", "app=crasher",
		"-o", "jsonpath={.items[0].status.containerStatuses[0].restartCount}",
	}, 2, 2*time.Minute)

	binary := buildProviderBinary(t)

	t.Run("ready_replicas is zero", func(t *testing.T) {
		out := probeNS(t, binary, "crasher", "ready_replicas", ns, "deployment")
		r := parseProbe(t, out)
		if v, _ := r.Value.(float64); v != 0 {
			t.Fatalf("ready_replicas want 0, got %v (status=%s)", r.Value, r.Status)
		}
	})

	t.Run("restart_count observed", func(t *testing.T) {
		out := probeNS(t, binary, "crasher", "restart_count", ns, "deployment")
		r := parseProbe(t, out)
		v, _ := r.Value.(float64)
		if int(v) < 2 {
			t.Fatalf("restart_count should be >= 2, got %v", r.Value)
		}
	})

	t.Run("condition_available is false", func(t *testing.T) {
		out := probeNS(t, binary, "crasher", "condition_available", ns, "deployment")
		r := parseProbe(t, out)
		if r.Value != false {
			t.Fatalf("condition_available want false, got %v", r.Value)
		}
	})
}

// ---------------------------------------------------------------------------
// Scenario 2 — Multi-tier cascade
// ---------------------------------------------------------------------------
//
// Applies ConfigMap + Deployment (mounting the ConfigMap) + Service. Probes
// each tier independently so a future `mgtt plan` run can demonstrate the
// engine walking the chain. The fixture is fully healthy; the interesting
// thing is coverage — every tier of the dependency graph returns authoritative
// values through one provider.

func TestScenario_MultiTierCascade(t *testing.T) {
	const ns = "mgtt-it-cascade"
	applyFixture(t, "cascade.yaml", ns)

	// Wait for deployment readiness — service and endpoints follow.
	waitForCondition(t, []string{
		"-n", ns, "get", "deploy", "api",
		"-o", "jsonpath={.status.readyReplicas}",
	}, "2", 3*time.Minute)

	binary := buildProviderBinary(t)

	t.Run("configmap.key_count includes data keys", func(t *testing.T) {
		out := probeNS(t, binary, "api-config", "key_count", ns, "configmap")
		r := parseProbe(t, out)
		v, _ := r.Value.(float64)
		if int(v) != 2 {
			t.Fatalf("key_count want 2, got %v", r.Value)
		}
	})

	t.Run("deployment.ready_replicas == 2", func(t *testing.T) {
		out := probeNS(t, binary, "api", "ready_replicas", ns, "deployment")
		r := parseProbe(t, out)
		if v, _ := r.Value.(float64); v != 2 {
			t.Fatalf("ready_replicas want 2, got %v", r.Value)
		}
	})

	t.Run("service.selector_match is true", func(t *testing.T) {
		out := probeNS(t, binary, "api", "selector_match", ns, "service")
		r := parseProbe(t, out)
		if r.Value != true {
			t.Fatalf("selector_match want true, got %v", r.Value)
		}
	})

	t.Run("endpoints.ready_count matches replicas", func(t *testing.T) {
		// endpoints object shares the service's name.
		out := probeNS(t, binary, "api", "ready_count", ns, "endpoints")
		r := parseProbe(t, out)
		v, _ := r.Value.(float64)
		if int(v) != 2 {
			t.Fatalf("endpoints.ready_count want 2, got %v", r.Value)
		}
	})

	t.Run("namespace.phase == Active", func(t *testing.T) {
		out := probeNS(t, binary, ns, "phase", "", "namespace")
		r := parseProbe(t, out)
		if r.Value != "Active" {
			t.Fatalf("namespace phase want Active, got %v", r.Value)
		}
	})
}

// ---------------------------------------------------------------------------
// Scenario 3 — Missing resource (status: not_found)
// ---------------------------------------------------------------------------
//
// Probes a deployment that does not exist. Provider MUST exit 0 and return
// status: not_found with a null Value — NOT an error — so design-time
// simulations and engine UnresolvedError translation work end-to-end.

func TestScenario_NotFound(t *testing.T) {
	binary := buildProviderBinary(t)

	stdout, stderr, code := probeAllowFail(t, binary,
		"probe", "nonexistent-deploy", "ready_replicas",
		"--namespace", "default", "--type", "deployment")

	if code != 0 {
		t.Fatalf("not_found should exit 0, got %d\nstderr: %s", code, stderr)
	}
	r := parseProbe(t, stdout)
	if r.Status != "not_found" {
		t.Fatalf("want status not_found, got %q (stdout=%s)", r.Status, stdout)
	}
	if r.Value != nil {
		t.Fatalf("value should be null on not_found, got %v", r.Value)
	}
}

// ---------------------------------------------------------------------------
// Scenario 4 — Dangling RoleBinding (Tier-3 composite probe)
// ---------------------------------------------------------------------------
//
// The fixture installs a RoleBinding that references a Role that does NOT
// exist. `kubectl describe rolebinding` shows the binding as healthy. The
// Tier-3 role_ref_exists probe surfaces the dangling reference.

func TestScenario_DanglingRoleBinding(t *testing.T) {
	const ns = "mgtt-it-rbac"
	applyFixture(t, "rbac-dangling.yaml", ns)

	// Ensure the binding is visible before probing — apply is not always
	// immediately queryable.
	waitForCondition(t, []string{
		"-n", ns, "get", "rolebinding", "reader-bind",
		"-o", "jsonpath={.metadata.name}",
	}, "reader-bind", 30*time.Second)

	binary := buildProviderBinary(t)

	t.Run("rolebinding.exists == true", func(t *testing.T) {
		out := probeNS(t, binary, "reader-bind", "exists", ns, "rolebinding")
		r := parseProbe(t, out)
		if r.Value != true {
			t.Fatalf("binding should exist, got %v", r.Value)
		}
	})

	t.Run("rolebinding.role_ref_exists == false (DANGLING)", func(t *testing.T) {
		out := probeNS(t, binary, "reader-bind", "role_ref_exists", ns, "rolebinding")
		r := parseProbe(t, out)
		if r.Value != false {
			t.Fatalf("dangling ref: want false, got %v — the Tier-3 composite probe failed to detect the missing Role", r.Value)
		}
	})

	t.Run("rolebinding.subject_count == 1", func(t *testing.T) {
		out := probeNS(t, binary, "reader-bind", "subject_count", ns, "rolebinding")
		r := parseProbe(t, out)
		v, _ := r.Value.(float64)
		if int(v) != 1 {
			t.Fatalf("subject_count want 1, got %v", r.Value)
		}
	})
}
