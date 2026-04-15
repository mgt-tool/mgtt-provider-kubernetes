package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerCronJob(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "cronjob", req.Name)
	}
	r.Register("cronjob", map[string]provider.ProbeFn{
		"suspended": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			v, _ := walk(d, "spec", "suspend").(bool)
			return provider.BoolResult(v), nil
		},
		"active_jobs": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountList(d, "status", "active")), nil
		},
		"last_schedule_age": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(AgeSeconds(JSONString(d, "status", "lastScheduleTime"))), nil
		},
		"last_successful_age": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(AgeSeconds(JSONString(d, "status", "lastSuccessfulTime"))), nil
		},
	})
}
