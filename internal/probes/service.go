package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerService(r *provider.Registry, c *shell.Client) {
	getSvc := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "service", req.Name)
	}
	r.Register("service", map[string]provider.ProbeFn{
		"type": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getSvc(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "spec", "type")), nil
		},
		"endpoint_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := KubectlJSON(ctx, c, "-n", req.Namespace, "get", "endpoints", req.Name)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountEndpointAddresses(d)), nil
		},
		"selector_match": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getSvc(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			sel, _ := walk(d, "spec", "selector").(map[string]any)
			return provider.BoolResult(len(sel) > 0), nil
		},
		"external_ip_assigned": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getSvc(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			lb, _ := walk(d, "status", "loadBalancer", "ingress").([]any)
			return provider.BoolResult(len(lb) > 0), nil
		},
	})
}
