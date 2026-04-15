package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerDeployment(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "deploy", req.Name)
	}
	podsByLabel := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "pods", "-l", "app="+req.Name)
	}
	endpointsGet := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "endpoints", req.Name)
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
	cond := func(name string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(ConditionStatus(d, name)), nil
		}
	}

	r.Register("deployment", map[string]provider.ProbeFn{
		"ready_replicas":        statusInt("readyReplicas"),
		"updated_replicas":      statusInt("updatedReplicas"),
		"available_replicas":    statusInt("availableReplicas"),
		"unavailable_replicas":  statusInt("unavailableReplicas"),
		"condition_available":   cond("Available"),
		"condition_progressing": cond("Progressing"),
		"desired_replicas": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(JSONInt(d, "spec", "replicas")), nil
		},
		"restart_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := podsByLabel(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(MaxRestartCount(d)), nil
		},
		"endpoints": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := endpointsGet(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountEndpointAddresses(d)), nil
		},
	})
}
