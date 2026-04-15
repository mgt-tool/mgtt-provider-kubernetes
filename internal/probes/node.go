package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerNode(r *provider.Registry, c *shell.Client) {
	// Nodes are cluster-scoped — no namespace.
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", "node", req.Name)
	}
	cond := func(name string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(ConditionStatus(d, name)), nil
		}
	}
	r.Register("node", map[string]provider.ProbeFn{
		"ready":               cond("Ready"),
		"memory_pressure":     cond("MemoryPressure"),
		"disk_pressure":       cond("DiskPressure"),
		"pid_pressure":        cond("PIDPressure"),
		"network_unavailable": cond("NetworkUnavailable"),
		"unschedulable": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			v, _ := walk(d, "spec", "unschedulable").(bool)
			return provider.BoolResult(v), nil
		},
		"cpu_allocatable": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "status", "allocatable", "cpu")), nil
		},
		"memory_allocatable": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "status", "allocatable", "memory")), nil
		},
		"pod_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := KubectlJSON(ctx, c, "get", "pods", "--all-namespaces",
				"--field-selector=spec.nodeName="+req.Name)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountList(d, "items")), nil
		},
	})
}
