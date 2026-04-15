package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerEndpoints(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "endpoints", req.Name)
	}
	countAcross := func(field string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			subsets, _ := d["subsets"].([]any)
			total := 0
			for _, s := range subsets {
				sm, _ := s.(map[string]any)
				list, _ := sm[field].([]any)
				total += len(list)
			}
			return provider.IntResult(total), nil
		}
	}
	r.Register("endpoints", map[string]provider.ProbeFn{
		"ready_count":     countAcross("addresses"),
		"not_ready_count": countAcross("notReadyAddresses"),
	})
}
