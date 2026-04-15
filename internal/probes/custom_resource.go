package probes

import (
	"context"
	"fmt"
	"strings"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// custom_resource probes an arbitrary CRD instance. The caller supplies one
// of these flag combinations via Request.Extra:
//
//   --resource widgets.example.com          (preferred — plural.group)
//   --kind Widget --api_version example.com/v1
//   --kind Widget --api_version example.com  (version inferred by kubectl)
//
// Scope defaults to namespaced; pass --scope cluster for cluster-scoped CRDs.
//
// Examples:
//
//   probe my-app ready --type custom_resource --namespace prod \
//     --resource widgets.example.com
//
//   probe my-app synced --type custom_resource --namespace prod \
//     --kind Widget --api_version example.com/v1

func registerCustomResource(r *provider.Registry, c *shell.Client) {
	getCR := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		target, err := crTarget(req)
		if err != nil {
			return nil, err
		}
		args := []string{"get", target, req.Name}
		if crScopedNamespaced(req) {
			args = append([]string{"-n", req.Namespace}, args...)
		}
		return KubectlJSON(ctx, c, args...)
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
			return Exists(c, target, crScopedNamespaced(req))(ctx, req)
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

// crTarget derives the kubectl resource argument from Request.Extra.
//
//   - Preferred: --resource <plural.group> (e.g. "widgets.example.com")
//   - Alternate: --kind <Kind> [--api_version <group> | <group/version>]
//
// For --kind/--api_version, kubectl expects a dotted resource form
// (kind.group). The version qualifier must be dropped from the target
// argument because kubectl does not accept "Kind.group/v1" as a single
// positional — version pinning belongs in the CRD, not the argv.
func crTarget(req provider.Request) (string, error) {
	if res := req.Extra["resource"]; res != "" {
		return res, nil
	}
	kind := req.Extra["kind"]
	// Accept both "api_version" (documented) and "api" (legacy) to be kind
	// to callers migrating. Documented form wins when both are set.
	apiVer := req.Extra["api_version"]
	if apiVer == "" {
		apiVer = req.Extra["api"]
	}
	if kind == "" {
		return "", fmt.Errorf(
			"%w: custom_resource requires --resource <plural.group> or --kind <Kind> [--api_version <group[/version]>]",
			provider.ErrUsage)
	}
	if apiVer == "" {
		return kind, nil
	}
	// Strip the /version suffix if the caller passed "group/version".
	// kubectl takes Kind.group as a single resource name; version is
	// resolved via API discovery.
	group := apiVer
	if slash := strings.IndexByte(apiVer, '/'); slash >= 0 {
		group = apiVer[:slash]
	}
	if group == "" {
		return kind, nil
	}
	return kind + "." + group, nil
}

// crScopedNamespaced reports whether the caller wants namespaced scope for
// this CR. Defaults to true; --scope=cluster switches to cluster-scoped so
// cluster-wide CRDs (e.g. CertificateRequest in some operators) work.
func crScopedNamespaced(req provider.Request) bool {
	return req.Extra["scope"] != "cluster"
}
