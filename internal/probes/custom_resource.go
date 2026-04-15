package probes

import (
	"context"
	"fmt"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// custom_resource probes an arbitrary CRD instance. The caller MUST supply
// either --kind (and optionally --api) or --resource to identify which CR
// to probe. Required flags come in via Request.Extra (the SDK exposes all
// --<key> <value> pairs except --type/--namespace there).
//
// Examples of valid argv the kubernetes provider accepts for custom_resource:
//
//   probe my-app ready --type custom_resource --namespace prod \
//     --resource widgets.example.com
//
//   probe my-app synced --type custom_resource --namespace prod \
//     --kind Widget --api example.com/v1

func registerCustomResource(r *provider.Registry, c *shell.Client) {
	getCR := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		target, err := crTarget(req)
		if err != nil {
			return nil, err
		}
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", target, req.Name)
	}
	cond := func(name string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getCR(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(ConditionStatus(d, name)), nil
		}
	}
	r.Register("custom_resource", map[string]provider.ProbeFn{
		"exists": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			target, err := crTarget(req)
			if err != nil {
				return provider.Result{}, err
			}
			return Exists(c, target, true)(ctx, req)
		},
		"ready":  cond("Ready"),
		"synced": cond("Synced"),
		"age": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := getCR(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(AgeSeconds(JSONString(d, "metadata", "creationTimestamp"))), nil
		},
	})
}

// crTarget derives the kubectl resource argument from Request.Extra. Returns
// ErrUsage if neither --resource nor --kind is supplied.
func crTarget(req provider.Request) (string, error) {
	if res := req.Extra["resource"]; res != "" {
		return res, nil
	}
	kind := req.Extra["kind"]
	api := req.Extra["api"]
	if kind != "" && api != "" {
		return fmt.Sprintf("%s.%s", kind, api), nil
	}
	if kind != "" {
		return kind, nil
	}
	return "", fmt.Errorf("%w: custom_resource requires --resource <plural.group> or --kind <Kind> [--api <group/version>]",
		provider.ErrUsage)
}
