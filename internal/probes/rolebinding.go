package probes

import (
	"context"
	"errors"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// rolebinding / clusterrolebinding expose role_ref_exists, which probes
// whether the referenced Role/ClusterRole is present. Dangling bindings are a
// common source of silently-broken RBAC.

func registerRoleBinding(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "-n", req.Namespace, "get", "rolebinding", req.Name)
	}
	r.Register("rolebinding", map[string]provider.ProbeFn{
		"exists": Exists(c, "rolebinding", true),
		"subject_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountList(d, "subjects")), nil
		},
		"role_ref_exists": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(roleRefExists(ctx, c, req.Namespace, d)), nil
		},
	})
}

func registerClusterRoleBinding(r *provider.Registry, c *shell.Client) {
	get := func(ctx context.Context, req provider.Request) (map[string]any, error) {
		return KubectlJSON(ctx, c, "get", "clusterrolebinding", req.Name)
	}
	r.Register("clusterrolebinding", map[string]provider.ProbeFn{
		"exists": Exists(c, "clusterrolebinding", false),
		"subject_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.IntResult(CountList(d, "subjects")), nil
		},
		"role_ref_exists": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			d, err := get(ctx, req)
			if err != nil {
				return provider.Result{}, err
			}
			// ClusterRoleBindings only reference ClusterRoles.
			return provider.BoolResult(roleRefExists(ctx, c, "", d)), nil
		},
	})
}

// roleRefExists resolves metadata.roleRef {kind, name} and probes whether the
// referenced role is actually present. bindingNamespace is "" for cluster-
// scoped bindings.
func roleRefExists(ctx context.Context, c *shell.Client, bindingNamespace string,
	d map[string]any) bool {
	ref, _ := walk(d, "roleRef").(map[string]any)
	kind, _ := ref["kind"].(string)
	name, _ := ref["name"].(string)
	if kind == "" || name == "" {
		return false
	}
	args := []string{"get"}
	switch kind {
	case "Role":
		if bindingNamespace == "" {
			return false // invalid: ClusterRoleBinding can't reference a Role
		}
		args = append(args, "-n", bindingNamespace, "role", name)
	case "ClusterRole":
		args = append(args, "clusterrole", name)
	default:
		return false
	}
	_, err := KubectlJSON(ctx, c, args...)
	if err == nil {
		return true
	}
	if errors.Is(err, provider.ErrNotFound) {
		return false
	}
	// Any other error (forbidden, transient) is treated as "unknown → false"
	// at this fact level; callers see the error from other facts if needed.
	return false
}
