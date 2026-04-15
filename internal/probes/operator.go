package probes

import (
	"context"
	"errors"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// operator models a typical "operator" deployment (an operator pod + CRDs it
// owns). Probes compose existing primitives: deployment health, CRD presence,
// and webhook backend reachability when the operator ships an admission
// webhook. The caller can supply:
//   --crd <full-crd-name>      (e.g. widgets.example.com)
//   --webhook <webhook-config> (optional)
//
// The component name itself is treated as the operator's Deployment name.

func registerOperator(r *provider.Registry, c *shell.Client) {
	deployOK := func(ctx context.Context, req provider.Request) bool {
		d, err := KubectlJSON(ctx, c, "-n", req.Namespace, "get", "deploy", req.Name)
		if err != nil {
			return false
		}
		return JSONInt(d, "status", "readyReplicas") > 0 && ConditionStatus(d, "Available")
	}
	r.Register("operator", map[string]provider.ProbeFn{
		"deployment_ready": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			return provider.BoolResult(deployOK(ctx, req)), nil
		},
		"crd_registered": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			crd := req.Extra["crd"]
			if crd == "" {
				return provider.BoolResult(true), nil // no CRD declared
			}
			_, err := KubectlJSON(ctx, c, "get", "customresourcedefinition", crd)
			if err != nil {
				if errors.Is(err, provider.ErrNotFound) {
					return provider.BoolResult(false), nil
				}
				return provider.Result{}, err
			}
			return provider.BoolResult(true), nil
		},
		"webhook_healthy": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			wh := req.Extra["webhook"]
			if wh == "" {
				return provider.BoolResult(true), nil // no webhook declared
			}
			// Try validating first, then mutating. Operators ship either,
			// both, or neither; a caller supplies only the name.
			for _, kind := range []string{
				"validatingwebhookconfiguration",
				"mutatingwebhookconfiguration",
			} {
				d, err := KubectlJSON(ctx, c, "get", kind, wh)
				if err != nil {
					if errors.Is(err, provider.ErrNotFound) {
						continue
					}
					return provider.Result{}, err
				}
				return provider.BoolResult(webhookBackendReachable(ctx, c, d)), nil
			}
			// Neither kind matched the supplied name.
			return provider.BoolResult(false), nil
		},
		"restart_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := KubectlJSON(ctx, c, "-n", req.Namespace, "get", "pods", "-l", "app="+req.Name)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(MaxRestartCount(d)), nil
		},
	})
}
