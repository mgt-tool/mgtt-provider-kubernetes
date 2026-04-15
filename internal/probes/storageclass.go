package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerStorageClass(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", "storageclass", req.Name)
	}
	r.Register("storageclass", map[string]provider.ProbeFn{
		"exists": Exists(c, "storageclass", false),
		"provisioner": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "provisioner")), nil
		},
		"is_default": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			annos, _ := walk(d, "metadata", "annotations").(map[string]any)
			v, _ := annos["storageclass.kubernetes.io/is-default-class"].(string)
			return provider.BoolResult(v == "true"), nil
		},
	})
}
