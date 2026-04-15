package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerIngress(r *provider.Registry, c *shell.Client) {
	getIngress := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "ingress", req.Name)
	}
	r.Register("ingress", map[string]provider.ProbeFn{
		"upstream_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := KubectlJSON(ctx, c, "-n", req.Namespace, "get", "endpoints", req.Name)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountEndpointAddresses(d)), nil
		},
		"backend_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getIngress(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			rules, _ := walk(d, "spec", "rules").([]any)
			total := 0
			for _, rule := range rules {
				rm, _ := rule.(map[string]any)
				if paths, ok := walk(rm, "http", "paths").([]any); ok {
					total += len(paths)
				}
			}
			return provider.IntResult(total), nil
		},
		"address_assigned": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getIngress(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			lb, _ := walk(d, "status", "loadBalancer", "ingress").([]any)
			return provider.BoolResult(len(lb) > 0), nil
		},
		"tls_configured": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getIngress(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			tls, _ := walk(d, "spec", "tls").([]any)
			return provider.BoolResult(len(tls) > 0), nil
		},
		"class": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getIngress(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "spec", "ingressClassName")), nil
		},
	})
}
