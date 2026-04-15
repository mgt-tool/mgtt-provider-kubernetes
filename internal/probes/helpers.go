// Package probes holds the kubernetes-specific probe implementations and
// JSON shape helpers. main() wires them into a provider.Registry and calls
// provider.Main.
package probes

import (
	"context"
	"fmt"
	"strings"

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

// Register adds all Tier-1 types to the provider registry. Each type's
// concrete probe map lives in its own file.
func Register(r *provider.Registry) {
	c := NewKubectl()
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
}
