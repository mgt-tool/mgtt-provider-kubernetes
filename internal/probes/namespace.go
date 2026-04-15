package probes

import (
	"context"
	"strings"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerNamespace(r *provider.Registry, c *shell.Client) {
	// Cluster-scoped.
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", "namespace", req.Name)
	}
	r.Register("namespace", map[string]provider.ProbeFn{
		"exists": Exists(c, "namespace", false),
		"phase": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "status", "phase")), nil
		},
		"terminating": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(strings.EqualFold(JSONString(d, "status", "phase"), "Terminating")), nil
		},
	})
}
