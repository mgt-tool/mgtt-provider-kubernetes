package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerResourceQuota(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "resourcequota", req.Name)
	}
	// Quantity fields ("200m", "2Gi") are returned as strings — we surface
	// them as-is so operators see the same units kubectl shows. Downstream
	// comparison logic can convert if needed.
	quantity := func(path ...string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, path...)), nil
		}
	}
	r.Register("resourcequota", map[string]provider.ProbeFn{
		"exists":      Exists(c, "resourcequota", true),
		"cpu_used":    quantity("status", "used", "requests.cpu"),
		"cpu_hard":    quantity("status", "hard", "requests.cpu"),
		"memory_used": quantity("status", "used", "requests.memory"),
		"memory_hard": quantity("status", "hard", "requests.memory"),
		"pods_used":   quantity("status", "used", "pods"),
		"pods_hard":   quantity("status", "hard", "pods"),
	})
}
