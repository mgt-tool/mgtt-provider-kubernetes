//go:build integration

// Package integration exercises the kubernetes provider end-to-end against a
// real cluster (kind, running locally via docker) and against mgtt itself
// (pulled as a docker image).
//
// Run with:
//
//	go test -tags=integration ./test/integration/...
//
// Requirements on the host: docker, kind, kubectl on $PATH. The mgtt docker
// image is pulled automatically; override with MGTT_IMAGE env var.
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

const (
	clusterName  = "mgtt-provider-kubernetes-it"
	providerName = "kubernetes"
	namespace    = "default"
	deployment   = "nginx"
	replicas     = 2
)

// ---------------------------------------------------------------------------
// Test lifecycle
// ---------------------------------------------------------------------------

// kubeconfigPath is populated by TestMain and points at a temporary file
// containing only the kind cluster's context. This keeps the test isolated
// from whatever KUBECONFIG / current-context the host already had.
var kubeconfigPath string

func TestMain(m *testing.M) {
	if err := ensureCluster(); err != nil {
		panic("ensureCluster: " + err.Error())
	}
	tmp, err := exportKubeconfig()
	if err != nil {
		panic("exportKubeconfig: " + err.Error())
	}
	kubeconfigPath = tmp
	defer os.Remove(tmp)

	if err := applyWorkload(); err != nil {
		panic("applyWorkload: " + err.Error())
	}
	if err := waitForDeployment(namespace, deployment, replicas, 3*time.Minute); err != nil {
		panic("waitForDeployment: " + err.Error())
	}
	code := m.Run()
	// Preserve cluster across runs for iteration speed. Destroy with:
	//   kind delete cluster --name mgtt-provider-kubernetes-it
	os.Exit(code)
}

// exportKubeconfig writes kind's kubeconfig (with the kind cluster as the
// only context) to a temp file and returns its path.
func exportKubeconfig() (string, error) {
	out, err := exec.Command("kind", "get", "kubeconfig", "--name", clusterName).Output()
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "mgtt-it-kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(out); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// ---------------------------------------------------------------------------
// Test 1: provider binary probes a real cluster correctly.
// ---------------------------------------------------------------------------

func TestProbeRunner_AgainstRealCluster(t *testing.T) {
	binary := buildProviderBinary(t)

	cases := []struct {
		fact string
		want int
	}{
		{"ready_replicas", replicas},
		{"desired_replicas", replicas},
		{"restart_count", 0},
	}

	for _, tc := range cases {
		t.Run(tc.fact, func(t *testing.T) {
			out, err := runProvider(binary, tc.fact)
			if err != nil {
				t.Fatalf("provider probe: %v", err)
			}
			got, err := intValue(out)
			if err != nil {
				t.Fatalf("decode %q: %v (raw=%q)", tc.fact, err, out)
			}
			if got != tc.want {
				t.Errorf("%s = %d, want %d (raw=%s)", tc.fact, got, tc.want, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 2: mgtt (via docker) consumes this provider and inspects the model.
// ---------------------------------------------------------------------------
//
// This demonstrates the integration contract: mgtt knows nothing about
// kubernetes — it just loads manifest.yaml + types/ from $MGTT_HOME and calls
// the runner binary via the declared `command` path.

func TestMgttDocker_ProviderInspect(t *testing.T) {
	image := ensureMgttImage(t)
	mgttHome := stagedMgttHome(t)

	out, err := dockerRun(image, mgttHome, "", "provider", "inspect", "kubernetes")
	if err != nil {
		t.Fatalf("mgtt provider inspect failed: %v\n%s", err, out)
	}

	text := string(out)
	// Inspect output must list at least a sampling of the v2.0.0 vocabulary.
	wantTypes := []string{"deployment", "service", "pod", "secret", "configmap"}
	for _, typ := range wantTypes {
		if !strings.Contains(text, typ) {
			t.Errorf("inspect output missing type %q. Full output:\n%s", typ, text)
		}
	}
}

func TestMgttDocker_ModelValidate(t *testing.T) {
	image := ensureMgttImage(t)
	mgttHome := stagedMgttHome(t)
	modelDir := filepath.Join(repoRoot(t), "test", "integration", "testdata")

	out, err := dockerRun(image, mgttHome, modelDir, "model", "validate", "/workspace/model.yaml")
	if err != nil {
		t.Fatalf("mgtt model validate failed: %v\n%s", err, out)
	}
	// Validation passing means mgtt loaded the k8s provider (via LoadFromDir),
	// resolved `type: deployment` against it, and found the vocabulary
	// coherent — which is exactly the end-to-end claim.
	_ = out
}

// ---------------------------------------------------------------------------
// Helpers — cluster lifecycle
// ---------------------------------------------------------------------------

func ensureCluster() error {
	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == clusterName {
			return nil
		}
	}
	cfg := filepath.Join("testdata", "kind-config.yaml")
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--config", cfg, "--wait", "120s")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func applyWorkload() error {
	manifest := filepath.Join("testdata", "healthy-nginx.yaml")
	cmd := exec.Command("kubectl", "apply", "-f", manifest)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForDeployment(ns, name string, want int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := exec.Command(
			"kubectl", "-n", ns, "get", "deploy", name,
			"-o", "jsonpath={.status.readyReplicas}",
		)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
		out, err := cmd.Output()
		if err == nil {
			got := strings.TrimSpace(string(out))
			if got != "" && got == intStr(want) {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return &timeoutErr{name: name, want: want}
}

type timeoutErr struct {
	name string
	want int
}

func (e *timeoutErr) Error() string {
	return "deployment " + e.name + " did not reach " + intStr(e.want) + " ready replicas in time"
}

// ---------------------------------------------------------------------------
// Helpers — provider binary
// ---------------------------------------------------------------------------

func buildProviderBinary(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "mgtt-provider-kubernetes")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build provider: %v\n%s", err, out)
	}
	return bin
}

func runProvider(binary, fact string) (string, error) {
	// The provider expects kubectl on $PATH and a working KUBECONFIG.
	cmd := exec.Command(binary,
		"probe", deployment, fact,
		"--namespace", namespace,
		"--type", "deployment",
	)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", &runErr{msg: string(ee.Stderr)}
		}
		return "", err
	}
	return string(out), nil
}

type runErr struct{ msg string }

func (e *runErr) Error() string { return "provider exit: " + e.msg }

func intValue(jsonLine string) (int, error) {
	var r struct {
		Value any    `json:"value"`
		Raw   string `json:"raw"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonLine)), &r); err != nil {
		return 0, err
	}
	switch v := r.Value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case string:
		// Some facts (regex-parsed) come back as strings; coerce.
		return atoi(v), nil
	}
	return 0, &runErr{msg: "unexpected value type"}
}

// ---------------------------------------------------------------------------
// Helpers — mgtt docker
// ---------------------------------------------------------------------------

// ensureMgttImage resolves a usable mgtt docker image. Priority:
//
//  1. $MGTT_IMAGE (caller-provided image name)
//  2. Build from $MGTT_SRC (path to a local mgtt checkout) into an
//     "mgtt:integration-test" tag — this is the canonical local workflow.
//  3. Build from ../mgtt if it exists next to this repo.
//
// Skips the test if none of the above produce an image.
func ensureMgttImage(t *testing.T) string {
	t.Helper()
	if img := os.Getenv("MGTT_IMAGE"); img != "" {
		return img
	}
	srcDir := os.Getenv("MGTT_SRC")
	if srcDir == "" {
		// Try the sibling directory convention.
		if wd, err := os.Getwd(); err == nil {
			candidate := filepath.Join(wd, "..", "..", "..", "mgtt")
			if _, err := os.Stat(filepath.Join(candidate, "Dockerfile")); err == nil {
				srcDir = candidate
			}
		}
	}
	if srcDir == "" {
		t.Skip("no mgtt image available: set MGTT_IMAGE or MGTT_SRC, or place the mgtt checkout at ../mgtt")
	}

	const tag = "mgtt:integration-test"
	cmd := exec.Command("docker", "build", "-t", tag, srcDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker build mgtt from %s: %v\n%s", srcDir, err, out)
	}
	return tag
}

// dockerRun runs `mgtt <args...>` inside the given image with the staged
// provider home bind-mounted at /data (matching the Dockerfile's MGTT_HOME)
// and, optionally, a workspace directory mounted at /workspace.
func dockerRun(image, mgttHome, workspace string, args ...string) ([]byte, error) {
	dockerArgs := []string{
		"run", "--rm",
		"-v", mgttHome + ":/data:ro",
	}
	if workspace != "" {
		dockerArgs = append(dockerArgs, "-v", workspace+":/workspace:ro")
	}
	dockerArgs = append(dockerArgs, image)
	dockerArgs = append(dockerArgs, args...)
	return exec.Command("docker", dockerArgs...).CombinedOutput()
}

// stagedMgttHome builds the provider binary, then lays out a directory with
// the layout mgtt expects: $MGTT_HOME/providers/kubernetes/{manifest.yaml,
// types/, bin/}. Returns an absolute path safe to bind-mount into docker.
func stagedMgttHome(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	home := t.TempDir()
	dest := filepath.Join(home, "providers", providerName)
	if err := os.MkdirAll(filepath.Join(dest, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy manifest.yaml, types/, hooks/.
	for _, rel := range []string{"manifest.yaml"} {
		if err := copyFile(filepath.Join(root, rel), filepath.Join(dest, rel)); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyDir(filepath.Join(root, "types"), filepath.Join(dest, "types")); err != nil {
		t.Fatal(err)
	}

	// Build provider binary into dest/bin/.
	bin := filepath.Join(dest, "bin", "mgtt-provider-"+providerName)
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build into staged home: %v\n%s", err, out)
	}
	return home
}

// ---------------------------------------------------------------------------
// Helpers — generic
// ---------------------------------------------------------------------------

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		return -n
	}
	return n
}
