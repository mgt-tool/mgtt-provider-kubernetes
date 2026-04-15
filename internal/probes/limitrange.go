package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerLimitRange(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "limitrange", req.Name)
	}
	// Returns the default container limit for a given resource, walking
	// spec.limits[].default.<resource>. The first matching "Container" type
	// limit wins.
	defaultContainerLimit := func(resource string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			limits, _ := walk(d, "spec", "limits").([]any)
			for _, l := range limits {
				lm, _ := l.(map[string]any)
				if t, _ := lm["type"].(string); t != "Container" {
					continue
				}
				def, _ := lm["default"].(map[string]any)
				if v, ok := def[resource].(string); ok {
					return provider.StringResult(v), nil
				}
			}
			return provider.StringResult(""), nil
		}
	}
	r.Register("limitrange", map[string]provider.ProbeFn{
		"exists":               Exists(c, "limitrange", true),
		"default_cpu_limit":    defaultContainerLimit("cpu"),
		"default_memory_limit": defaultContainerLimit("memory"),
	})
}
