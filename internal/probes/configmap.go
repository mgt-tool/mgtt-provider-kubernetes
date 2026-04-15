package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerConfigMap(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "configmap", req.Name)
	}
	r.Register("configmap", map[string]provider.ProbeFn{
		"exists": Exists(c, "configmap", true),
		"key_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountMapKeys(d, "data") + CountMapKeys(d, "binaryData")), nil
		},
		"age": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(AgeSeconds(JSONString(d, "metadata", "creationTimestamp"))), nil
		},
	})
}
