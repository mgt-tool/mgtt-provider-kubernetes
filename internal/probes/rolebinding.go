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
			ok, err := roleRefExists(ctx, c, req.Namespace, d)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(ok), nil
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
			ok, err := roleRefExists(ctx, c, "", d)
			if err != nil {
				return provider.Result{}, err
			}
			return provider.BoolResult(ok), nil
		},
	})
}

// roleRefExists resolves metadata.roleRef {kind, name} and probes whether
// the referenced role is actually present. bindingNamespace is "" for
// cluster-scoped bindings.
//
// Returns (true, nil) when the ref resolves, (false, nil) when it's
// definitively missing (ErrNotFound), (false, nil) when the probing
// credential cannot see the role (ErrForbidden — operationally a dangling
// ref from this caller's perspective), and (_, err) on transient errors so
// a network blip doesn't flip a healthy binding into "dangling".
func roleRefExists(ctx context.Context, c *shell.Client, bindingNamespace string,
	d map[string]any) (bool, error) {
	ref, _ := walk(d, "roleRef").(map[string]any)
	kind, _ := ref["kind"].(string)
	name, _ := ref["name"].(string)
	if kind == "" || name == "" {
		return false, nil
	}
	args := []string{"get"}
	switch kind {
	case "Role":
		if bindingNamespace == "" {
			// Invalid: a ClusterRoleBinding can't reference a Role.
			return false, nil
		}
		args = append(args, "-n", bindingNamespace, "role", name)
	case "ClusterRole":
		args = append(args, "clusterrole", name)
	default:
		return false, nil
	}
	_, err := KubectlJSON(ctx, c, args...)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, provider.ErrNotFound):
		return false, nil
	case errors.Is(err, provider.ErrForbidden):
		return false, nil
	case errors.Is(err, provider.ErrTransient):
		return false, err
	}
	// Unknown classification: bubble up rather than silently decide.
	return false, err
}
