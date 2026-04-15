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
			// Upstream count = number of ready endpoint addresses across every
			// Service the ingress references. The endpoints object is named
			// after the Service, NOT the ingress itself — so we walk
			// spec.rules[].http.paths[].backend.service.name and also
			// spec.defaultBackend.service.name.
			ing, err := getIngress(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			serviceNames := collectIngressBackendServices(ing)
			if len(serviceNames) == 0 {
				return provider.IntResult(0), nil
			}
			total := 0
			for _, name := range serviceNames {
				ep, err := KubectlJSON(ctx, c, "-n", req.Namespace, "get", "endpoints", name)
				if err != nil {
					// A missing backend endpoints object counts as zero
					// upstreams — that's the actionable signal for the
					// operator, not a probe error.
					continue
				}
				total += CountEndpointAddresses(ep)
			}
			return provider.IntResult(total), nil
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

// collectIngressBackendServices returns the deduped list of Service names
// referenced by an Ingress, including both per-path backends and the default
// backend. Order is preserved for stable output.
func collectIngressBackendServices(ing map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	add := func(m map[string]any) {
		svc, _ := walk(m, "service").(map[string]any)
		if svc == nil {
			return
		}
		name, _ := svc["name"].(string)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if db, ok := walk(ing, "spec", "defaultBackend").(map[string]any); ok {
		add(db)
	}
	rules, _ := walk(ing, "spec", "rules").([]any)
	for _, r := range rules {
		rm, _ := r.(map[string]any)
		paths, _ := walk(rm, "http", "paths").([]any)
		for _, p := range paths {
			pm, _ := p.(map[string]any)
			if b, ok := pm["backend"].(map[string]any); ok {
				add(b)
			}
		}
	}
	return out
}
