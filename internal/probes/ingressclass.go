package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerIngressClass(r *provider.Registry, c *shell.Client) {
	// Cluster-scoped resource.
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", "ingressclass", req.Name)
	}
	r.Register("ingressclass", map[string]provider.ProbeFn{
		"exists": Exists(c, "ingressclass", false),
		"controller": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "spec", "controller")), nil
		},
	})
}
