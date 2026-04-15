package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerVolumeAttachment(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", "volumeattachment", req.Name)
	}
	r.Register("volumeattachment", map[string]provider.ProbeFn{
		"attached": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			v, _ := walk(d, "status", "attached").(bool)
			return provider.BoolResult(v), nil
		},
		"attach_error": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "status", "attachError", "message")), nil
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
