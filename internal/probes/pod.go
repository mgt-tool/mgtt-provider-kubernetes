package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerPod(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "pod", req.Name)
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
	r.Register("pod", map[string]provider.ProbeFn{
		"phase": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "status", "phase")), nil
		},
		"ready":            cond("Ready"),
		"scheduled":        cond("PodScheduled"),
		"containers_ready": cond("ContainersReady"),
		"restart_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			containers, _ := walk(d, "status", "containerStatuses").([]any)
			maxVal := 0
			for _, cs := range containers {
				m, _ := cs.(map[string]any)
				if v, ok := m["restartCount"].(float64); ok && int(v) > maxVal {
					maxVal = int(v)
				}
			}
			return provider.IntResult(maxVal), nil
		},
		"oom_killed": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			// Single pod: wrap in items: [] shape for AnyContainerReason reuse.
			list := map[string]any{"items": []any{d}}
			return provider.BoolResult(AnyContainerReason(list, "OOMKilled")), nil
		},
	})
}
