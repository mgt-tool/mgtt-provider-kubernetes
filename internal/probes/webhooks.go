package probes

import (
	"context"
	"errors"
	"strings"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// webhooks: both validating and mutating expose the same facts since they
// share the admission-webhook shape (spec.webhooks[]).

func registerValidatingWebhookConfiguration(r *provider.Registry, c *shell.Client) {
	registerWebhookConfig(r, c, "validatingwebhookconfiguration")
}

func registerMutatingWebhookConfiguration(r *provider.Registry, c *shell.Client) {
	registerWebhookConfig(r, c, "mutatingwebhookconfiguration")
}

func registerWebhookConfig(r *provider.Registry, c *shell.Client, kind string) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", kind, req.Name)
	}
	r.Register(kind, map[string]provider.ProbeFn{
		"exists": Exists(c, kind, false),
		"webhook_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountList(d, "webhooks")), nil
		},
		"failure_policy": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			// Report the first webhook's failurePolicy — webhookconfigs
			// typically set it uniformly; expose the aggregate via majority.
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			hooks, _ := d["webhooks"].([]any)
			for _, h := range hooks {
				hm, _ := h.(map[string]any)
				if fp, ok := hm["failurePolicy"].(string); ok {
					return provider.StringResult(fp), nil
				}
			}
			return provider.StringResult(""), nil
		},
		"service_available": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(webhookBackendReachable(ctx, c, d)), nil
		},
	})
}

// webhookBackendReachable returns true iff every webhook in the config
// references a clientConfig.service whose backing Service exists. A single
// missing backend means the whole config is potentially degraded.
func webhookBackendReachable(ctx context.Context, c *shell.Client, d map[string]any) bool {
	hooks, _ := d["webhooks"].([]any)
	if len(hooks) == 0 {
		return false
	}
	for _, h := range hooks {
		hm, _ := h.(map[string]any)
		svc, _ := walk(hm, "clientConfig", "service").(map[string]any)
		if svc == nil {
			// URL-based webhook; assume reachable (we can't probe arbitrary URLs).
			continue
		}
		name, _ := svc["name"].(string)
		ns, _ := svc["namespace"].(string)
		if name == "" || ns == "" {
			return false
		}
		_, err := KubectlJSON(ctx, c, "-n", ns, "get", "service", name)
		if err != nil {
			if errors.Is(err, provider.ErrNotFound) ||
				strings.Contains(err.Error(), "not found") {
				return false
			}
			return false
		}
	}
	return true
}
