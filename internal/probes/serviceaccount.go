package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerServiceAccount(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "serviceaccount", req.Name)
	}
	// IRSA annotation convention (AWS EKS):
	// eks.amazonaws.com/role-arn → ARN of the IAM role bound to this SA.
	irsaARN := func(d map[string]any) string {
		anno, _ := walk(d, "metadata", "annotations").(map[string]any)
		if v, ok := anno["eks.amazonaws.com/role-arn"].(string); ok {
			return v
		}
		return ""
	}
	r.Register("serviceaccount", map[string]provider.ProbeFn{
		"exists": Exists(c, "serviceaccount", true),
		"irsa_role_arn": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.StringResult(irsaARN(d)), nil
		},
		"has_irsa": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(irsaARN(d) != ""), nil
		},
	})
}
