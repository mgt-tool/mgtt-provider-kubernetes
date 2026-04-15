package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerPDB(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "pdb", req.Name)
	}
	statusInt := func(field string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(JSONInt(d, "status", field)), nil
		}
	}
	r.Register("pdb", map[string]provider.ProbeFn{
		"allowed_disruptions": statusInt("disruptionsAllowed"),
		"current_healthy":     statusInt("currentHealthy"),
		"desired_healthy":     statusInt("desiredHealthy"),
		"expected_pods":       statusInt("expectedPods"),
	})
}
