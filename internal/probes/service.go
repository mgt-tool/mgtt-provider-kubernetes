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
			// True iff the service's selector matches at least one pod in
			// the namespace. A non-empty selector that matches nothing is
			// the common "my service has no endpoints and I don't know why"
			// case operators need to see.
			d, err := getSvc(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			sel, _ := walk(d, "spec", "selector").(map[string]any)
			if len(sel) == 0 {
				// No selector at all (ExternalName / headless-by-endpoints).
				// Not a match, but not a failure — operators reading this
				// see `false` and can cross-check the service type.
				return provider.BoolResult(false), nil
			}
			labelSel := buildLabelSelector(sel)
			pods, err := KubectlJSON(ctx, c, "-n", req.Namespace, "get", "pods",
				"-l", labelSel)
			if err != nil {
				return provider.Result{}, err
			}
			items, _ := pods["items"].([]any)
			return provider.BoolResult(len(items) > 0), nil
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

// buildLabelSelector converts a selector map into the comma-separated form
// kubectl accepts via -l. Keys are sorted for stable output.
func buildLabelSelector(sel map[string]any) string {
	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sortStrings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v, _ := sel[k].(string)
		parts = append(parts, k+"="+v)
	}
	return joinWith(parts, ",")
}

// sortStrings / joinWith — tiny shims to avoid importing "sort" and "strings"
// into this file alone. The caller paths are hot but not performance-sensitive.
func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j-1] > xs[j]; j-- {
			xs[j-1], xs[j] = xs[j], xs[j-1]
		}
	}
}

func joinWith(xs []string, sep string) string {
	if len(xs) == 0 {
		return ""
	}
	out := xs[0]
	for _, s := range xs[1:] {
		out += sep + s
	}
	return out
}
