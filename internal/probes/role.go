package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerRole(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "role", req.Name)
	}
	r.Register("role", map[string]provider.ProbeFn{
		"exists": Exists(c, "role", true),
		"rule_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountList(d, "rules")), nil
		},
	})
}

func registerClusterRole(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", "clusterrole", req.Name)
	}
	r.Register("clusterrole", map[string]provider.ProbeFn{
		"exists": Exists(c, "clusterrole", false),
		"rule_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountList(d, "rules")), nil
		},
	})
}
