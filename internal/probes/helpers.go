// Package probes holds the kubernetes-specific probe implementations and
// JSON shape helpers. main() wires them into a provider.Registry and calls
// provider.Main.
package probes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mgt-tool/mgtt-provider-kubernetes/internal/kubeclassify"
	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// NewKubectl returns a shell.Client pointed at kubectl with the kubectl
// stderr classifier wired in.
func NewKubectl() *shell.Client {
	c := shell.New("kubectl")
	c.Classify = kubeclassify.Classify
	return c
}

// KubectlJSON runs kubectl with -o json and returns the parsed map.
func KubectlJSON(ctx context.Context, c *shell.Client, args ...string) (map[string]any, error) {
	full := append(append([]string{}, args...), "-o", "json")
	return c.RunJSON(ctx, full...)
}

// --- JSON shape helpers -----------------------------------------------------

// JSONInt traverses nested map by path, returning int. 0 for missing/nil.
func JSONInt(data map[string]any, path ...string) int {
	v := walk(data, path...)
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		var n int
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

// JSONBool accepts both native bool and the "True"/"False" strings Kubernetes
// uses for conditions.
func JSONBool(data map[string]any, path ...string) bool {
	switch x := walk(data, path...).(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "True")
	}
	return false
}

// JSONString returns the string at path, or "" if absent/wrong type.
func JSONString(data map[string]any, path ...string) string {
	if s, ok := walk(data, path...).(string); ok {
		return s
	}
	return ""
}

// CountList returns len of the []any at path, or 0.
func CountList(data map[string]any, path ...string) int {
	if l, ok := walk(data, path...).([]any); ok {
		return len(l)
	}
	return 0
}

// ConditionStatus returns whether the named status.conditions entry is "True".
func ConditionStatus(data map[string]any, name string) bool {
	conds, _ := walk(data, "status", "conditions").([]any)
	for _, c := range conds {
		m, _ := c.(map[string]any)
		if t, _ := m["type"].(string); t == name {
			s, _ := m["status"].(string)
			return strings.EqualFold(s, "True")
		}
	}
	return false
}

// MaxRestartCount returns the highest restartCount across containers in a pod list.
func MaxRestartCount(data map[string]any) int {
	items, _ := data["items"].([]any)
	maxVal := 0
	for _, item := range items {
		pod, _ := item.(map[string]any)
		status, _ := pod["status"].(map[string]any)
		containers, _ := status["containerStatuses"].([]any)
		for _, c := range containers {
			cs, _ := c.(map[string]any)
			if v, ok := cs["restartCount"].(float64); ok && int(v) > maxVal {
				maxVal = int(v)
			}
		}
	}
	return maxVal
}

// CountEndpointAddresses counts addresses across all subsets of an Endpoints.
func CountEndpointAddresses(data map[string]any) int {
	subsets, _ := data["subsets"].([]any)
	count := 0
	for _, s := range subsets {
		subset, _ := s.(map[string]any)
		addrs, _ := subset["addresses"].([]any)
		count += len(addrs)
	}
	return count
}

// AnyContainerReason returns true if any container in any pod terminated with
// the given reason.
func AnyContainerReason(data map[string]any, reason string) bool {
	items, _ := data["items"].([]any)
	for _, item := range items {
		pod, _ := item.(map[string]any)
		status, _ := pod["status"].(map[string]any)
		containers, _ := status["containerStatuses"].([]any)
		for _, c := range containers {
			cs, _ := c.(map[string]any)
			last, _ := cs["lastState"].(map[string]any)
			term, _ := last["terminated"].(map[string]any)
			if r, _ := term["reason"].(string); r == reason {
				return true
			}
		}
	}
	return false
}

// AgeSeconds parses an RFC3339 timestamp and returns the elapsed seconds
// since that time. Returns 0 for empty or malformed input.
func AgeSeconds(rfc3339 string) int {
	if rfc3339 == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return 0
	}
	return int(time.Since(t).Seconds())
}

// CountMapKeys returns the number of keys in the map at path, or 0.
func CountMapKeys(data map[string]any, path ...string) int {
	if m, ok := walk(data, path...).(map[string]any); ok {
		return len(m)
	}
	return 0
}

// Exists returns a ProbeFn that reports whether `kubectl get <kind> <name>`
// succeeds. Uses the standard kubeclassify NotFound translation to distinguish
// "resource missing" from "backend error". Namespaced kinds pass scoped=true;
// cluster-scoped kinds pass false.
func Exists(c *shell.Client, kind string, scoped bool) provider.ProbeFn {
	return func(ctx context.Context, req provider.Request) (provider.Result, error) {
		args := []string{"get", kind, req.Name}
		if scoped {
			args = append([]string{"-n", req.Namespace}, args...)
		}
		_, err := KubectlJSON(ctx, c, args...)
		if err != nil {
			// NotFound → return bool:false, not an error — the `exists` fact
			// is specifically intended to observe presence.
			if errors.Is(err, provider.ErrNotFound) {
				return provider.BoolResult(false), nil
			}
			return provider.Result{}, err
		}
		return provider.BoolResult(true), nil
	}
}

func walk(m map[string]any, path ...string) any {
	var cur any = m
	for _, k := range path {
		mp, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mp[k]
	}
	return cur
}

// Register adds all implemented types to the provider registry. Each type's
// concrete probe map lives in its own file.
func Register(r *provider.Registry) {
	c := NewKubectl()
	// Workloads
	registerDeployment(r, c)
	registerIngress(r, c)
	registerPod(r, c)
	registerService(r, c)
	registerEndpoints(r, c)
	registerStatefulset(r, c)
	registerDaemonset(r, c)
	registerPVC(r, c)
	registerNode(r, c)
	registerHPA(r, c)
	registerReplicaset(r, c)
	registerCronJob(r, c)
	registerJob(r, c)
	registerNetworkPolicy(r, c)
	registerIngressClass(r, c)
	registerPDB(r, c)
	registerNamespace(r, c)
	registerConfigMap(r, c)
	registerSecret(r, c)
	registerServiceAccount(r, c)
	registerRole(r, c)
	registerClusterRole(r, c)
	registerRoleBinding(r, c)
	registerClusterRoleBinding(r, c)
	registerValidatingWebhookConfiguration(r, c)
	registerMutatingWebhookConfiguration(r, c)
	registerCRD(r, c)
	registerCustomResource(r, c)
	registerOperator(r, c)
	registerPriorityClass(r, c)
	registerLease(r, c)
	registerCSIDriver(r, c)
	registerVolumeAttachment(r, c)
	registerPersistentVolume(r, c)
	registerStorageClass(r, c)
	registerResourceQuota(r, c)
	registerLimitRange(r, c)
}
