package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerDaemonset(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "daemonset", req.Name)
	}
	statusInt := func(field string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(JSONInt(d, "status", field)), nil
		}
	}
	r.Register("daemonset", map[string]provider.ProbeFn{
		"desired_scheduled": statusInt("desiredNumberScheduled"),
		"current_scheduled": statusInt("currentNumberScheduled"),
		"ready":             statusInt("numberReady"),
		"misscheduled":      statusInt("numberMisscheduled"),
		"unavailable":       statusInt("numberUnavailable"),
		"restart_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := KubectlJSON(ctx, c, "-n", req.Namespace, "get", "pods", "-l", "app="+req.Name)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(MaxRestartCount(d)), nil
		},
	})
}
