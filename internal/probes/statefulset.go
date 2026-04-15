package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerStatefulset(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "statefulset", req.Name)
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
	statusStr := func(field string) provider.ProbeFn {
		return func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(JSONString(d, "status", field)), nil
		}
	}
	r.Register("statefulset", map[string]provider.ProbeFn{
		"ready_replicas":   statusInt("readyReplicas"),
		"updated_replicas": statusInt("updatedReplicas"),
		"current_replicas": statusInt("currentReplicas"),
		"current_revision": statusStr("currentRevision"),
		"update_revision":  statusStr("updateRevision"),
		"desired_replicas": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(JSONInt(d, "spec", "replicas")), nil
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
